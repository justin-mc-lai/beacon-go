package ui

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"beacon-go/internal/llm"
	"beacon-go/internal/orchestrator"
	"beacon-go/internal/session"
)

//go:embed assets/index.html
var consoleHTML []byte

// Server encapsulates the Beacon Agent Console Web server.
type Server struct {
	addr       string
	server     *http.Server
	cos        *orchestrator.ChiefOfStaff
	cfg        *llm.Config
	sessions   session.Store
	workdir    string
	eventsMu   sync.RWMutex
	eventChans []chan string
}

// FeatureItem represents a scanned feature summary.
type FeatureItem struct {
	Slug        string `json:"slug"`
	Version     string `json:"version"`
	Status      string `json:"status"`
	Title       string `json:"title"`
	HasTruth    bool   `json:"has_truth"`
	HasTests    bool   `json:"has_tests"`
	HasEvidence bool   `json:"has_evidence"`
}

// FeatureDetail contains full document content.
type FeatureDetail struct {
	Slug     string `json:"slug"`
	Version  string `json:"version"`
	TruthMD  string `json:"truth_md"`
	TestsMD  string `json:"tests_md"`
	TasksMD  string `json:"tasks_md"`
	Evidence string `json:"evidence_json"`
}

// NewServer initializes the Agent Console Server.
func NewServer(addr string) *Server {
	cfg, _ := llm.LoadConfig()
	cli, model, _ := cfg.GetClientForRole(llm.RoleChiefOfStaff)
	cos := orchestrator.NewChiefOfStaff(cli, model)
	sStore, _ := session.NewFileStore("")

	cwd, _ := os.Getwd()

	s := &Server{
		addr:     addr,
		cfg:      cfg,
		cos:      cos,
		sessions: sStore,
		workdir:  cwd,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", s.handleHealth)
	mux.HandleFunc("/api/events", s.handleSSE)
	mux.HandleFunc("/api/chat", s.handleChat)
	mux.HandleFunc("/api/sessions", s.handleSessions)
	mux.HandleFunc("/api/sessions/messages", s.handleSessionMessages)
	mux.HandleFunc("/api/action/approve", s.handleApprove)
	mux.HandleFunc("/api/workspace/features", s.handleWorkspaceFeatures)
	mux.HandleFunc("/api/workspace/feature", s.handleWorkspaceFeatureDetail)
	mux.HandleFunc("/api/settings", s.handleSettings)
	mux.HandleFunc("/", s.handleIndex)

	s.server = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 90 * time.Second,
	}
	return s
}

func (s *Server) Handler() http.Handler {
	return s.server.Handler
}

func (s *Server) SetWorkdir(dir string) {
	s.workdir = dir
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  "ok",
		"mode":    "beacon-agent-console",
		"version": "v2.0.0",
		"roles":   s.cfg.Roles,
	})
}

// handleSettings reads or saves the LLM BYOK configuration.
func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodGet {
		_ = json.NewEncoder(w).Encode(s.cfg)
		return
	}
	if r.Method == http.MethodPost {
		var newCfg llm.Config
		if err := json.NewDecoder(r.Body).Decode(&newCfg); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := newCfg.SaveConfig(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		s.cfg = &newCfg
		// Re-initialize Chief of Staff client with updated role config
		cli, model, _ := s.cfg.GetClientForRole(llm.RoleChiefOfStaff)
		s.cos = orchestrator.NewChiefOfStaff(cli, model)

		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":  "ok",
			"message": "Settings updated and saved successfully",
		})
		return
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

// handleWorkspaceFeatures scans docs/beacon/ for all defined features.
func (s *Server) handleWorkspaceFeatures(w http.ResponseWriter, r *http.Request) {
	var list []FeatureItem
	baseDir := filepath.Join(s.workdir, "docs", "beacon")

	_ = filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || !info.IsDir() {
			return nil
		}
		if filepath.Base(filepath.Dir(path)) == "features" {
			slug := filepath.Base(path)
			ver := filepath.Base(filepath.Dir(filepath.Dir(path)))
			
			item := FeatureItem{
				Slug:    slug,
				Version: ver,
				Status:  "Frozen",
				Title:   strings.ReplaceAll(slug, "-", " "),
			}
			if _, err := os.Stat(filepath.Join(path, "truth.md")); err == nil {
				item.HasTruth = true
			}
			if _, err := os.Stat(filepath.Join(path, "tests.md")); err == nil {
				item.HasTests = true
			}
			if _, err := os.Stat(filepath.Join(path, "evidence.md")); err == nil {
				item.HasEvidence = true
			}
			list = append(list, item)
		}
		return nil
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"features": list,
		"count":    len(list),
	})
}

// handleWorkspaceFeatureDetail returns Markdown contents of a feature.
func (s *Server) handleWorkspaceFeatureDetail(w http.ResponseWriter, r *http.Request) {
	slug := r.URL.Query().Get("slug")
	if slug == "" {
		http.Error(w, "missing slug parameter", http.StatusBadRequest)
		return
	}

	detail := FeatureDetail{
		Slug:    slug,
		Version: "v2.0.0",
	}

	// Search in docs/beacon/
	baseDir := filepath.Join(s.workdir, "docs", "beacon")
	_ = filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && info.IsDir() && filepath.Base(path) == slug && filepath.Base(filepath.Dir(path)) == "features" {
			ver := filepath.Base(filepath.Dir(filepath.Dir(path)))
			detail.Version = ver

			truthBytes, _ := os.ReadFile(filepath.Join(path, "truth.md"))
			detail.TruthMD = string(truthBytes)

			testsBytes, _ := os.ReadFile(filepath.Join(path, "tests.md"))
			detail.TestsMD = string(testsBytes)

			tasksBytes, _ := os.ReadFile(filepath.Join(path, "tasks.md"))
			detail.TasksMD = string(tasksBytes)

			evDir := filepath.Join(s.workdir, ".beacon", "evidence", "implement", slug)
			if files, err := os.ReadDir(evDir); err == nil && len(files) > 0 {
				evBytes, _ := os.ReadFile(filepath.Join(evDir, files[0].Name()))
				detail.Evidence = string(evBytes)
			}
		}
		return nil
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(detail)
}

// handleSessions lists or creates conversation threads.
func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method == http.MethodGet {
		list, err := s.sessions.ListSessions()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(list)
		return
	}
	if r.Method == http.MethodPost {
		var req struct {
			Title string `json:"title"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		sess, err := s.sessions.CreateSession(req.Title)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(sess)
		return
	}
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

// handleSessionMessages gets messages for a session.
func (s *Server) handleSessionMessages(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	sessID := r.URL.Query().Get("session_id")
	if sessID == "" {
		http.Error(w, "missing session_id", http.StatusBadRequest)
		return
	}
	msgs, err := s.sessions.GetSessionMessages(sessID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(msgs)
}

// handleChat handles user messages to the Chief of Staff.
func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		SessionID string `json:"session_id"`
		Prompt    string `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if req.SessionID == "" {
		sess, _ := s.sessions.CreateSession("新需求会话")
		req.SessionID = sess.ID
	}

	// 1. Record user message
	_ = s.sessions.AppendMessage(session.ChatMessage{
		SessionID: req.SessionID,
		Role:      "user",
		Content:   req.Prompt,
	})

	// 2. Stream tokens to SSE and align intent
	brief, err := s.cos.AlignIntentStream(r.Context(), req.Prompt, func(chunk string) error {
		s.broadcastEvent(fmt.Sprintf(`{"type":"chat_chunk","session_id":"%s","chunk":%q}`, req.SessionID, chunk))
		return nil
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// 3. Record assistant message
	_ = s.sessions.AppendMessage(session.ChatMessage{
		SessionID:   req.SessionID,
		Role:        "chief",
		Content:     brief.RestatedIntent,
		CardType:    "plan_brief",
		CardPayload: brief.ProposedSlugs[0],
	})

	// Broadcast agent activity to SSE
	s.broadcastEvent(fmt.Sprintf(`{"type":"agent_action","agent":"Chief of Staff","action":"Intent Aligned","slug":"%s"}`, brief.ProposedSlugs[0]))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"session_id": req.SessionID,
		"brief":      brief,
	})
}

func (s *Server) handleApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Slug     string `json:"slug"`
		Action   string `json:"action"` // approve / deny
		Operator string `json:"operator"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	s.broadcastEvent(fmt.Sprintf(`{"type":"gate_event","action":"%s","slug":"%s","operator":"%s"}`, req.Action, req.Slug, req.Operator))

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"slug":     req.Slug,
		"action":   req.Action,
		"result":   "approved",
		"operator": req.Operator,
		"stamp":    time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) broadcastEvent(msg string) {
	s.eventsMu.RLock()
	defer s.eventsMu.RUnlock()
	for _, ch := range s.eventChans {
		select {
		case ch <- msg:
		default:
		}
	}
}

func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan string, 32)
	s.eventsMu.Lock()
	s.eventChans = append(s.eventChans, ch)
	s.eventsMu.Unlock()

	defer func() {
		s.eventsMu.Lock()
		for i, c := range s.eventChans {
			if c == ch {
				s.eventChans = append(s.eventChans[:i], s.eventChans[i+1:]...)
				break
			}
		}
		s.eventsMu.Unlock()
	}()

	fmt.Fprintf(w, "data: %s\n\n", `{"type":"connected","message":"Beacon Agent Stream Active"}`)
	flusher.Flush()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case msg := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", msg)
			flusher.Flush()
		}
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(consoleHTML)
}
