package session

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
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
	CardType    string    `json:"card_type,omitempty"` // plan_brief, goal_run, release_verdict
	CardPayload string    `json:"card_payload,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// Store defines persistence for Sessions and Messages.
type Store interface {
	ListSessions() ([]Session, error)
	CreateSession(title string) (*Session, error)
	GetSessionMessages(sessionID string) ([]ChatMessage, error)
	AppendMessage(msg ChatMessage) error
}

// SQLiteStore keeps conversation history in one SQLite file with WAL
// and transactional writes. It replaces the earlier JSON-file store:
// a half-written JSON file could silently lose a whole thread, and
// list/append were not atomic against each other.
type SQLiteStore struct {
	db   *sql.DB
	Path string
}

const sessionSchema = `
CREATE TABLE IF NOT EXISTS sessions (
	id         TEXT PRIMARY KEY,
	title      TEXT NOT NULL,
	status     TEXT NOT NULL DEFAULT 'active',
	created_at TEXT NOT NULL,
	updated_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS messages (
	id           TEXT PRIMARY KEY,
	session_id   TEXT NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	role         TEXT NOT NULL,
	content      TEXT NOT NULL,
	card_type    TEXT NOT NULL DEFAULT '',
	card_payload TEXT NOT NULL DEFAULT '',
	created_at   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, created_at);
`

// DefaultPath resolves the conversation store: $BEACON_SESSIONS_PATH,
// otherwise ~/.beacon/sessions.db. It is deliberately separate from the
// kernel ledger: conversation history is UI state, not runtime
// authority over runs or gates (charter rule 7 / AC-016).
func DefaultPath() (string, error) {
	if p := os.Getenv("BEACON_SESSIONS_PATH"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".beacon", "sessions.db"), nil
}

// NewStore opens (creating if needed) the conversation store at path.
// An empty path resolves through DefaultPath.
func NewStore(path string) (*SQLiteStore, error) {
	if path == "" {
		p, err := DefaultPath()
		if err != nil {
			return nil, err
		}
		path = p
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}
	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(sessionSchema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("session schema: %w", err)
	}
	return &SQLiteStore{db: db, Path: path}, nil
}

// Close releases the database handle.
func (s *SQLiteStore) Close() error { return s.db.Close() }

// ListSessions returns threads newest-updated first.
func (s *SQLiteStore) ListSessions() ([]Session, error) {
	rows, err := s.db.Query(`SELECT id, title, status, created_at, updated_at
		FROM sessions ORDER BY updated_at DESC, id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []Session{}
	for rows.Next() {
		var (
			sess             Session
			created, updated string
		)
		if err := rows.Scan(&sess.ID, &sess.Title, &sess.Status, &created, &updated); err != nil {
			return nil, err
		}
		sess.CreatedAt = parseTime(created)
		sess.UpdatedAt = parseTime(updated)
		list = append(list, sess)
	}
	return list, rows.Err()
}

// CreateSession inserts one thread and returns it.
func (s *SQLiteStore) CreateSession(title string) (*Session, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "New session"
	}
	now := time.Now().UTC()
	sess := &Session{
		ID:        "sess-" + newID(),
		Title:     title,
		CreatedAt: now,
		UpdatedAt: now,
		Status:    "active",
	}
	if _, err := s.db.Exec(`INSERT INTO sessions (id, title, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`,
		sess.ID, sess.Title, sess.Status, formatTime(now), formatTime(now)); err != nil {
		return nil, err
	}
	return sess, nil
}

// GetSessionMessages returns one thread's turns in insertion order.
func (s *SQLiteStore) GetSessionMessages(sessionID string) ([]ChatMessage, error) {
	rows, err := s.db.Query(`SELECT id, session_id, role, content, card_type, card_payload, created_at
		FROM messages WHERE session_id = ? ORDER BY created_at, rowid`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ChatMessage{}
	for rows.Next() {
		var (
			m       ChatMessage
			created string
		)
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.CardType, &m.CardPayload, &created); err != nil {
			return nil, err
		}
		m.CreatedAt = parseTime(created)
		out = append(out, m)
	}
	return out, rows.Err()
}

// AppendMessage writes one turn and bumps the thread's updated_at in the
// same transaction, so a listed session can never lag its messages.
func (s *SQLiteStore) AppendMessage(msg ChatMessage) error {
	if msg.SessionID == "" {
		return fmt.Errorf("session: append requires a session id")
	}
	if msg.ID == "" {
		msg.ID = "msg-" + newID()
	}
	now := time.Now().UTC()
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = now
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`INSERT INTO messages (id, session_id, role, content, card_type, card_payload, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		msg.ID, msg.SessionID, msg.Role, msg.Content, msg.CardType, msg.CardPayload,
		formatTime(msg.CreatedAt)); err != nil {
		return err
	}
	if _, err := tx.Exec(`UPDATE sessions SET updated_at = ? WHERE id = ?`,
		formatTime(now), msg.SessionID); err != nil {
		return err
	}
	return tx.Commit()
}

func formatTime(t time.Time) string { return t.UTC().Format(time.RFC3339Nano) }

func parseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}
