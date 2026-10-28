package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
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
