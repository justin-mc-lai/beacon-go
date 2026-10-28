package session

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Session represents an active or archived planning conversation thread.
type Session struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Status    string    `json:"status"` // active, archived
}

// ChatMessage represents a single turn inside a Session.
type ChatMessage struct {
	ID          string    `json:"id"`
	SessionID   string    `json:"session_id"`
	Role        string    `json:"role"` // user, chief, architect, coder, qa
	Content     string    `json:"content"`
	CardType    string    `json:"card_type,omitempty"` // plan_brief, goal_gate
	CardPayload string    `json:"card_payload,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Store defines persistence methods for Sessions and Messages.
type Store interface {
	ListSessions() ([]Session, error)
	CreateSession(title string) (*Session, error)
	GetSessionMessages(sessionID string) ([]ChatMessage, error)
	AppendMessage(msg ChatMessage) error
}

// FileStore implements Store using persistent JSON files in ~/.beacon/sessions/.
type FileStore struct {
	mu      sync.RWMutex
	baseDir string
}

// NewFileStore initializes the session store under the given directory or ~/.beacon/sessions.
func NewFileStore(dir string) (*FileStore, error) {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		dir = filepath.Join(home, ".beacon", "sessions")
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	return &FileStore{baseDir: dir}, nil
}

type sessionData struct {
	Session  Session       `json:"session"`
	Messages []ChatMessage `json:"messages"`
}

func (s *FileStore) ListSessions() ([]Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.baseDir)
	if err != nil {
		return nil, err
	}

	var list []Session
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			data, err := os.ReadFile(filepath.Join(s.baseDir, entry.Name()))
			if err == nil {
				var sd sessionData
				if err := json.Unmarshal(data, &sd); err == nil && sd.Session.ID != "" {
					list = append(list, sd.Session)
				}
			}
		}
	}

	// Sort descending by UpdatedAt
	for i := 0; i < len(list); i++ {
		for j := i + 1; j < len(list); j++ {
			if list[i].UpdatedAt.Before(list[j].UpdatedAt) {
				list[i], list[j] = list[j], list[i]
			}
		}
	}

	return list, nil
}

func (s *FileStore) CreateSession(title string) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("sess-%d", time.Now().UnixNano())
	now := time.Now().UTC()
	sess := Session{
		ID:        id,
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
		Status:    "active",
	}

	sd := sessionData{
		Session:  sess,
		Messages: []ChatMessage{},
	}

	raw, err := json.MarshalIndent(sd, "", "  ")
	if err != nil {
		return nil, err
	}

	targetPath := filepath.Join(s.baseDir, fmt.Sprintf("%s.json", id))
	if err := os.WriteFile(targetPath, raw, 0644); err != nil {
		return nil, err
	}

	return &sess, nil
}

func (s *FileStore) GetSessionMessages(sessionID string) ([]ChatMessage, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	targetPath := filepath.Join(s.baseDir, fmt.Sprintf("%s.json", sessionID))
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return nil, err
	}

	var sd sessionData
	if err := json.Unmarshal(data, &sd); err != nil {
		return nil, err
	}
	return sd.Messages, nil
}

func (s *FileStore) AppendMessage(msg ChatMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if msg.ID == "" {
		msg.ID = fmt.Sprintf("msg-%d", time.Now().UnixNano())
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now().UTC()
	}

	targetPath := filepath.Join(s.baseDir, fmt.Sprintf("%s.json", msg.SessionID))
	data, err := os.ReadFile(targetPath)
	if err != nil {
		return err
	}

	var sd sessionData
	if err := json.Unmarshal(data, &sd); err != nil {
		return err
	}

	sd.Messages = append(sd.Messages, msg)
	sd.Session.UpdatedAt = msg.CreatedAt

	raw, err := json.MarshalIndent(sd, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(targetPath, raw, 0644)
}
