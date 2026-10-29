package session

import (
	"path/filepath"
	"testing"
)

func newStore(t *testing.T) *SQLiteStore {
	t.Helper()
	st, err := NewStore(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	return st
}

func TestSessionStore_CRUD(t *testing.T) {
	store := newStore(t)

	sess1, err := store.CreateSession("微信退款方案设计")
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	if sess1.ID == "" || sess1.Title != "微信退款方案设计" {
		t.Errorf("unexpected session fields: %+v", sess1)
	}

	sess2, err := store.CreateSession("用户积分重构")
	if err != nil {
		t.Fatalf("failed to create session 2: %v", err)
	}

	list, err := store.ListSessions()
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(list))
	}

	if err := store.AppendMessage(ChatMessage{
		SessionID: sess1.ID,
		Role:      "user",
		Content:   "请帮我梳理退款幂等逻辑",
	}); err != nil {
		t.Fatalf("failed to append message: %v", err)
	}
	if err := store.AppendMessage(ChatMessage{
		SessionID:   sess1.ID,
		Role:        "chief",
		Content:     "参谋长已分析：核心边界在于重复回调加锁防重",
		CardType:    "plan_brief",
		CardPayload: `{"slug":"order-refund"}`,
	}); err != nil {
		t.Fatalf("failed to append message 2: %v", err)
	}

	msgs, err := store.GetSessionMessages(sess1.ID)
	if err != nil {
		t.Fatalf("failed to get messages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[1].Role != "chief" {
		t.Errorf("message roles mismatch: %+v", msgs)
	}
	if msgs[1].CardType != "plan_brief" {
		t.Errorf("expected plan_brief card type, got %s", msgs[1].CardType)
	}
	if msgs[1].CardPayload != `{"slug":"order-refund"}` {
		t.Errorf("card payload not round-tripped: %q", msgs[1].CardPayload)
	}

	msgs2, err := store.GetSessionMessages(sess2.ID)
	if err != nil {
		t.Fatalf("failed to get messages for sess2: %v", err)
	}
	if len(msgs2) != 0 {
		t.Errorf("expected 0 messages for sess2, got %d", len(msgs2))
	}
}

// A thread must sort to the top once it receives a message — that is
// the whole point of bumping updated_at inside the append transaction.
func TestAppendBumpsSessionToTop(t *testing.T) {
	store := newStore(t)
	first, _ := store.CreateSession("first")
	second, _ := store.CreateSession("second")

	before, err := store.ListSessions()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if before[0].ID != second.ID {
		t.Fatalf("newest thread should sort first, got %s", before[0].ID)
	}

	if err := store.AppendMessage(ChatMessage{SessionID: first.ID, Role: "user", Content: "hi"}); err != nil {
		t.Fatalf("append: %v", err)
	}
	after, err := store.ListSessions()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if after[0].ID != first.ID {
		t.Fatalf("thread with the newest message should sort first, got %s", after[0].ID)
	}
}

// Messages must survive a close/reopen cycle — the earlier file store
// could leave a truncated JSON file behind on a crash.
func TestMessagesSurviveReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sessions.db")
	st, err := NewStore(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sess, err := st.CreateSession("persisted")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := st.AppendMessage(ChatMessage{SessionID: sess.ID, Role: "user", Content: "keep me"}); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	st2, err := NewStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = st2.Close() }()
	msgs, err := st2.GetSessionMessages(sess.ID)
	if err != nil {
		t.Fatalf("messages after reopen: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Content != "keep me" {
		t.Fatalf("message lost across reopen: %+v", msgs)
	}
}

func TestAppendRejectsUnknownSession(t *testing.T) {
	store := newStore(t)
	if err := store.AppendMessage(ChatMessage{SessionID: "sess-missing", Role: "user", Content: "x"}); err == nil {
		t.Fatal("foreign key must reject a message for an unknown session")
	}
}
