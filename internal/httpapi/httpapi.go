// Package httpapi is the read-only HTTP/JSON projection of the kernel
// runtime store (kernel-readonly-http-api). It exposes exactly five GET
// endpoints over a store opened read-only; there is no write, release or
// publish route anywhere in this package.
package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"

	"beacon-go/internal/store"
)

// Server serves the read-only JSON projection. Construct it with New and
// mount Handler; the zero value is not usable.
type Server struct {
	st      *store.Store
	version string
}

// New builds the projection over an already opened (read-only) store.
func New(st *store.Store, version string) *Server {
	return &Server{st: st, version: version}
}

// Handler returns the HTTP handler for the five read endpoints.
func (s *Server) Handler() http.Handler { return s }

// ValidateLoopbackAddr accepts only 127.0.0.1 or ::1 hosts; everything
// else (0.0.0.0, ::, hostnames, bare ports) is refused before a listener
// is created.
func ValidateLoopbackAddr(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("invalid address %q: %v", addr, err)
	}
	if host != "127.0.0.1" && host != "::1" {
		return fmt.Errorf("address %q is not loopback: host must be 127.0.0.1 or ::1", addr)
	}
	if port == "" {
		return fmt.Errorf("invalid address %q: missing port", addr)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 0 || n > 65535 {
		return fmt.Errorf("invalid address %q: port must be 0-65535", addr)
	}
	return nil
}

type apiError struct {
	status  int
	code    string
	message string
}

func (e apiError) Error() string { return e.message }

func errNotFound(message string) apiError {
	return apiError{status: http.StatusNotFound, code: "not_found", message: message}
}

func errBadRequest(message string) apiError {
	return apiError{status: http.StatusBadRequest, code: "bad_request", message: message}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	switch {
	case path == "/health":
		s.getOnly(w, r, s.health)
	case path == "/spine/runs":
		s.getOnly(w, r, s.spineRuns)
	case path == "/spine/gates":
		s.getOnly(w, r, s.spineGates)
	case strings.HasPrefix(path, "/runs/"):
		id := strings.TrimPrefix(path, "/runs/")
		if !singleSegment(id) {
			s.writeError(w, errNotFound("unknown path: "+path))
			return
		}
		s.getOnly(w, r, func(w http.ResponseWriter, r *http.Request) error {
			return s.runByID(w, r, id)
		})
	case strings.HasPrefix(path, "/features/"):
		slug := strings.TrimPrefix(path, "/features/")
		if !singleSegment(slug) {
			s.writeError(w, errNotFound("unknown path: "+path))
			return
		}
		s.getOnly(w, r, func(w http.ResponseWriter, r *http.Request) error {
			return s.featureBySlug(w, r, slug)
		})
	default:
		s.writeError(w, errNotFound("unknown path: "+path))
	}
}

// getOnly enforces the read-only method contract: only GET reaches a
// handler, everything else on a known path is 405 with the common error
// shape.
func (s *Server) getOnly(w http.ResponseWriter, r *http.Request, h func(http.ResponseWriter, *http.Request) error) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		s.writeError(w, apiError{
			status:  http.StatusMethodNotAllowed,
			code:    "method_not_allowed",
			message: fmt.Sprintf("%s is not allowed on %s: the API is read-only (GET only)", r.Method, r.URL.Path),
		})
		return
	}
	if err := h(w, r); err != nil {
		var ae apiError
		if errors.As(err, &ae) {
			s.writeError(w, ae)
			return
		}
		s.writeError(w, apiError{status: http.StatusInternalServerError, code: "internal_error", message: err.Error()})
	}
}

func singleSegment(v string) bool {
	return v != "" && !strings.Contains(v, "/")
}

func (s *Server) writeJSON(w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

func (s *Server) writeError(w http.ResponseWriter, e apiError) {
	type errBody struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	type errEnvelope struct {
		Error errBody `json:"error"`
	}
	_ = s.writeJSON(w, e.status, errEnvelope{Error: errBody{Code: e.code, Message: e.message}})
}

// ---- endpoint payloads (field sets are frozen by the truth package) ----

type healthPayload struct {
	Status   string `json:"status"`
	Version  string `json:"version"`
	ReadOnly bool   `json:"read_only"`
}

type spineRunItem struct {
	RunID        string  `json:"run_id"`
	Status       string  `json:"status"`
	CurrentStage *string `json:"current_stage"`
	Blocked      bool    `json:"blocked"`
	Feature      string  `json:"feature"`
}

type spineRunsPayload struct {
	Runs  []spineRunItem `json:"runs"`
	Count int            `json:"count"`
}

type gateItem struct {
	GateID      string   `json:"gate_id"`
	GateType    string   `json:"gate_type"`
	Verdict     string   `json:"verdict"`
	EvaluatedAt string   `json:"evaluated_at"`
	ReasonCodes []string `json:"reason_codes"`
}

type spineGatesPayload struct {
	Feature string     `json:"feature"`
	Gates   []gateItem `json:"gates"`
}

type runDetail struct {
	RunID        string  `json:"run_id"`
	Status       string  `json:"status"`
	CurrentStage *string `json:"current_stage"`
	Blocked      bool    `json:"blocked"`
	Feature      string  `json:"feature"`
	StartedAt    string  `json:"started_at"`
	Iteration    int     `json:"iteration"`
}

type runDetailPayload struct {
	Run runDetail `json:"run"`
}

type featureDetail struct {
	Slug      string `json:"slug"`
	Status    string `json:"status"`
	TruthPath string `json:"truth_path"`
	TestsPath string `json:"tests_path"`
}

type featurePayload struct {
	Feature featureDetail `json:"feature"`
}

type runMetaView struct {
	CurrentStageIdx int `json:"current_stage_idx"`
	StagePlan       []struct {
		Stage string `json:"stage"`
	} `json:"stage_plan"`
	Blocked bool `json:"blocked"`
}

func metaView(raw json.RawMessage) runMetaView {
	var m runMetaView
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &m)
	}
	return m
}

func (m runMetaView) currentStage() *string {
	if m.CurrentStageIdx < 0 || m.CurrentStageIdx >= len(m.StagePlan) {
		return nil
	}
	stage := m.StagePlan[m.CurrentStageIdx].Stage
	if stage == "" {
		return nil
	}
	return &stage
}

func (m runMetaView) blocked() bool { return m.Blocked }

func (s *Server) featureSlug(featureID string) string {
	feat, err := s.st.Features().GetByID(featureID)
	if err != nil {
		return ""
	}
	return feat.Slug
}

// ---- handlers ----

func (s *Server) health(w http.ResponseWriter, _ *http.Request) error {
	return s.writeJSON(w, http.StatusOK, healthPayload{Status: "ok", Version: s.version, ReadOnly: true})
}

func (s *Server) spineRuns(w http.ResponseWriter, _ *http.Request) error {
	runs, err := s.st.Runs().ListAll()
	if err != nil {
		return err
	}
	feats, err := s.st.Features().ListAll()
	if err != nil {
		return err
	}
	slugByID := make(map[string]string, len(feats))
	for _, f := range feats {
		slugByID[f.ID] = f.Slug
	}
	out := make([]spineRunItem, 0, len(runs))
	for _, run := range runs {
		m := metaView(run.Metadata)
		out = append(out, spineRunItem{
			RunID:        run.ID,
			Status:       string(run.Status),
			CurrentStage: m.currentStage(),
			Blocked:      m.blocked(),
			Feature:      slugByID[run.FeatureID],
		})
	}
	return s.writeJSON(w, http.StatusOK, spineRunsPayload{Runs: out, Count: len(out)})
}

func (s *Server) spineGates(w http.ResponseWriter, r *http.Request) error {
	feature := strings.TrimSpace(r.URL.Query().Get("feature"))
	if feature == "" {
		return errBadRequest("query parameter 'feature' is required")
	}
	feat, err := s.st.Features().FindBySlug(feature)
	if errors.Is(err, sql.ErrNoRows) {
		return errNotFound("feature not found: " + feature)
	}
	if err != nil {
		return err
	}
	gates, err := s.st.Gates().ListByFeature(feat.ID)
	if err != nil {
		return err
	}
	out := make([]gateItem, 0, len(gates))
	for _, g := range gates {
		codes := g.ReasonCodes
		if codes == nil {
			codes = []string{}
		}
		out = append(out, gateItem{
			GateID:      g.ID,
			GateType:    g.GateType,
			Verdict:     g.Verdict,
			EvaluatedAt: g.EvaluatedAt,
			ReasonCodes: codes,
		})
	}
	return s.writeJSON(w, http.StatusOK, spineGatesPayload{Feature: feat.Slug, Gates: out})
}

func (s *Server) runByID(w http.ResponseWriter, _ *http.Request, id string) error {
	run, err := s.st.Runs().GetByID(id)
	if errors.Is(err, sql.ErrNoRows) {
		return errNotFound("run not found: " + id)
	}
	if err != nil {
		return err
	}
	m := metaView(run.Metadata)
	return s.writeJSON(w, http.StatusOK, runDetailPayload{Run: runDetail{
		RunID:        run.ID,
		Status:       string(run.Status),
		CurrentStage: m.currentStage(),
		Blocked:      m.blocked(),
		Feature:      s.featureSlug(run.FeatureID),
		StartedAt:    run.StartedAt,
		Iteration:    run.Iteration,
	}})
}

func (s *Server) featureBySlug(w http.ResponseWriter, _ *http.Request, slug string) error {
	feat, err := s.st.Features().FindBySlug(slug)
	if errors.Is(err, sql.ErrNoRows) {
		return errNotFound("feature not found: " + slug)
	}
	if err != nil {
		return err
	}
	return s.writeJSON(w, http.StatusOK, featurePayload{Feature: featureDetail{
		Slug:      feat.Slug,
		Status:    feat.Status,
		TruthPath: feat.TruthPath,
		TestsPath: feat.TestsPath,
	}})
}
