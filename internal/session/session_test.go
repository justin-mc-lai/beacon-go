package session

import (
	"testing"
)

func TestSessionStore_CRUD(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileStore(tmpDir)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	// 1. Create Session
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

	// 2. List Sessions
	list, err := store.ListSessions()
	if err != nil {
		t.Fatalf("failed to list sessions: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(list))
	}

	// 3. Append Messages
	err = store.AppendMessage(ChatMessage{
		SessionID: sess1.ID,
		Role:      "user",
		Content:   "请帮我梳理退款幂等逻辑",
	})
	if err != nil {
		t.Fatalf("failed to append message: %v", err)
	}

	err = store.AppendMessage(ChatMessage{
		SessionID:   sess1.ID,
		Role:        "chief",
		Content:     "参谋长已分析：核心边界在于重复回调加锁防重",
		CardType:    "plan_brief",
		CardPayload: `{"slug":"order-refund"}`,
	})
	if err != nil {
		t.Fatalf("failed to append message 2: %v", err)
	}

	// 4. Get Messages
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

	// sess2 should have 0 messages
	msgs2, err := store.GetSessionMessages(sess2.ID)
	if err != nil {
		t.Fatalf("failed to get messages for sess2: %v", err)
	}
	if len(msgs2) != 0 {
		t.Errorf("expected 0 messages for sess2, got %d", len(msgs2))
	}
}
