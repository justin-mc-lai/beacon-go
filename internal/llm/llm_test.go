package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type mockTransport struct {
	roundTrip func(req *http.Request) (*http.Response, error)
}

func (m *mockTransport) Do(req *http.Request) (*http.Response, error) {
	return m.roundTrip(req)
}

func TestLLMClientFactory(t *testing.T) {
	mockDoer := &mockTransport{
		roundTrip: func(r *http.Request) (*http.Response, error) {
			if r.Header.Get("Authorization") != "Bearer test-key" {
				return &http.Response{
					StatusCode: http.StatusUnauthorized,
					Body:       io.NopCloser(bytes.NewBufferString(`{"error":{"message":"unauthorized"}}`)),
				}, nil
			}
			var req openAIChatReq
			_ = json.NewDecoder(r.Body).Decode(&req)
			resp := openAIChatResp{}
			resp.Choices = append(resp.Choices, struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			}{
				Message: struct {
					Content string `json:"content"`
				}{Content: "Hello from Mock Server for model " + req.Model},
			})
			resp.Usage.PromptTokens = 15
			resp.Usage.CompletionTokens = 35
			respBytes, _ := json.Marshal(resp)
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewBuffer(respBytes)),
			}, nil
		},
	}

	cfg := ProviderConfig{
		BaseURL: "http://mock-api.internal",
		APIKey:  "test-key",
	}
	rawClient, err := NewClient(ProviderDeepSeek, cfg)
	if err != nil {
		t.Fatalf("unexpected error creating client: %v", err)
	}

	client, ok := rawClient.(*OpenAICompatibleClient)
	if !ok {
		t.Fatalf("expected *OpenAICompatibleClient")
	}
	client.SetDoer(mockDoer)

	res, err := client.Complete(context.Background(), Request{
		Role:        RoleArchitect,
		Model:       "deepseek-reasoner",
		Messages:    []Message{{Role: "user", Content: "Hello"}},
		Temperature: 0.0,
		MaxTokens:   100,
	})
	if err != nil {
		t.Fatalf("unexpected error calling complete: %v", err)
	}
	if res.Content != "Hello from Mock Server for model deepseek-reasoner" {
		t.Errorf("unexpected content: %s", res.Content)
	}
	if res.PromptTokens != 15 || res.CompTokens != 35 {
		t.Errorf("unexpected tokens: %d, %d", res.PromptTokens, res.CompTokens)
	}
}

func TestResolveAPIKey(t *testing.T) {
	cfg := DefaultConfig()
	t.Setenv("TEST_CUSTOM_KEY", "secret-val-123")
	cfg.Providers[ProviderOpenAI] = ProviderConfig{
		APIKey: "${TEST_CUSTOM_KEY}",
	}
	got := cfg.ResolveAPIKey(ProviderOpenAI)
	if got != "secret-val-123" {
		t.Fatalf("expected resolved env var 'secret-val-123', got %q", got)
	}
}

// AC-STREAM-002: StreamComplete forwards every incremental chunk to the
// handler (at least three increments for a multi-chunk answer) and the
// full content is the concatenation of what the provider sent.
func TestStreamCompleteChunks(t *testing.T) {
	body := strings.Join([]string{
		"data: {\"choices\":[{\"delta\":{\"content\":\"退\"}}]}",
		"data: {\"choices\":[{\"delta\":{\"content\":\"款\"}}]}",
		"data: {\"choices\":[{\"delta\":{\"content\":\"方案\"}}]}",
		"data: [DONE]",
		"",
	}, "\n")
	doer := &mockTransport{roundTrip: func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Accept") == "" {
			// no-op: the client does not require Accept
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
			Body:       io.NopCloser(strings.NewReader(body)),
		}, nil
	}}
	c := NewOpenAICompatibleClient(ProviderDeepSeek, "https://example.invalid/v1", "k")
	c.SetDoer(doer)

	var chunks []string
	resp, err := c.StreamComplete(context.Background(), Request{
		Model:    "deepseek-chat",
		Messages: []Message{{Role: "user", Content: "你好"}},
	}, func(chunk string) error {
		chunks = append(chunks, chunk)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) < 3 {
		t.Fatalf("chunks = %d want at least 3 incremental deliveries", len(chunks))
	}
	joined := strings.Join(chunks, "")
	if !strings.Contains(joined, "退") || !strings.Contains(joined, "方案") {
		t.Fatalf("streamed chunks lost content: %q", joined)
	}
	if resp == nil || !strings.Contains(resp.Content, "方案") {
		t.Fatalf("final response content = %+v", resp)
	}
}

// A provider that answers with no handler must fall back to Complete and
// still return content — streaming is an option, not a new requirement.
func TestStreamCompleteNilHandlerFallsBack(t *testing.T) {
	doer := &mockTransport{roundTrip: func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewBufferString(`{"choices":[{"message":{"content":"ok"}}]}`)),
		}, nil
	}}
	c := NewOpenAICompatibleClient(ProviderOpenAI, "https://example.invalid/v1", "k")
	c.SetDoer(doer)
	resp, err := c.StreamComplete(context.Background(), Request{Model: "gpt", Messages: []Message{{Role: "user", Content: "hi"}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != "ok" {
		t.Fatalf("fallback content = %q", resp.Content)
	}
}
