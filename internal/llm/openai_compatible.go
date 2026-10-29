package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPDoer allows custom transport injection (e.g. for mock tests without sockets).
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// OpenAICompatibleClient talks to standard v1/chat/completions endpoints.
type OpenAICompatibleClient struct {
	provider Provider
	baseURL  string
	apiKey   string
	client   HTTPDoer
}

// NewOpenAICompatibleClient constructs a client for OpenAI, DeepSeek, OneAPI, etc.
func NewOpenAICompatibleClient(p Provider, baseURL, apiKey string) *OpenAICompatibleClient {
	if baseURL == "" {
		if p == ProviderDeepSeek {
			baseURL = "https://api.deepseek.com/v1"
		} else {
			baseURL = "https://api.openai.com/v1"
		}
	}
	return &OpenAICompatibleClient{
		provider: p,
		baseURL:  baseURL,
		apiKey:   apiKey,
		client:   &http.Client{Timeout: 90 * time.Second},
	}
}

// SetDoer overrides the underlying HTTP transport (useful for sandboxed tests).
func (c *OpenAICompatibleClient) SetDoer(d HTTPDoer) {
	c.client = d
}

func (c *OpenAICompatibleClient) Provider() Provider {
	return c.provider
}

type openAIReqMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatReq struct {
	Model       string         `json:"model"`
	Messages    []openAIReqMsg `json:"messages"`
	Temperature float64        `json:"temperature"`
	MaxTokens   int            `json:"max_tokens,omitempty"`
	Stream      bool           `json:"stream,omitempty"`
}

type openAIChatResp struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (c *OpenAICompatibleClient) Complete(ctx context.Context, req Request) (*Response, error) {
	if c.apiKey == "" {
		return nil, errors.New("missing API key for provider: " + string(c.provider))
	}
	endpoint := c.baseURL + "/chat/completions"
	var msgs []openAIReqMsg
	for _, m := range req.Messages {
		msgs = append(msgs, openAIReqMsg{Role: m.Role, Content: m.Content})
	}
	bodyData := openAIChatReq{
		Model:       req.Model,
		Messages:    msgs,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}
	rawJSON, err := json.Marshal(bodyData)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(rawJSON))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("provider %s HTTP %d: %s", c.provider, resp.StatusCode, string(respBytes))
	}
	var chatResp openAIChatResp
	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		return nil, err
	}
	if chatResp.Error != nil && chatResp.Error.Message != "" {
		return nil, errors.New(chatResp.Error.Message)
	}
	if len(chatResp.Choices) == 0 {
		return nil, errors.New("empty choices returned from provider")
	}
	return &Response{
		Content:      chatResp.Choices[0].Message.Content,
		PromptTokens: chatResp.Usage.PromptTokens,
		CompTokens:   chatResp.Usage.CompletionTokens,
		Model:        req.Model,
	}, nil
}

func (c *OpenAICompatibleClient) StreamComplete(ctx context.Context, req Request, handler StreamChunkHandler) (*Response, error) {
	// If no handler, fallback to Complete
	if handler == nil {
		return c.Complete(ctx, req)
	}

	var msgs []openAIReqMsg
	for _, m := range req.Messages {
		msgs = append(msgs, openAIReqMsg{Role: m.Role, Content: m.Content})
	}

	payload := openAIChatReq{
		Model:       req.Model,
		Messages:    msgs,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      true,
	}

	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	endpoint := fmt.Sprintf("%s/chat/completions", c.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("provider %s HTTP %d: %s", c.provider, resp.StatusCode, string(respBytes))
	}

	// SSE: one `data:` line per delta. Forward the DELTA text, not the
	// raw read buffer — a single TCP read can carry several events, and a
	// raw buffer makes the host's typing animation jump in blocks. A
	// non-SSE body (some gateways ignore stream=true) is parsed as one
	// complete answer.
	var fullContent strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	sawSSE := false
	respUsage := struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	}{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, ":") {
			continue
		}
		if !strings.HasPrefix(line, "data:") {
			if !sawSSE {
				// Non-SSE answer: re-read is impossible, so treat the
				// remaining body as one JSON document.
				rest := line
				for scanner.Scan() {
					rest += "\n" + scanner.Text()
				}
				var chatResp openAIChatResp
				if err := json.Unmarshal([]byte(rest), &chatResp); err != nil {
					return nil, err
				}
				if chatResp.Error != nil && chatResp.Error.Message != "" {
					return nil, errors.New(chatResp.Error.Message)
				}
				if len(chatResp.Choices) == 0 {
					return nil, errors.New("empty choices returned from provider")
				}
				content := chatResp.Choices[0].Message.Content
				if err := handler(content); err != nil {
					return nil, err
				}
				return &Response{
					Content: content, PromptTokens: chatResp.Usage.PromptTokens,
					CompTokens: chatResp.Usage.CompletionTokens, Model: req.Model,
				}, nil
			}
			continue
		}
		sawSSE = true
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}
		var event struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
			Usage *struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
			} `json:"usage"`
		}
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			continue // keepalive / vendor noise: not a content event
		}
		if event.Usage != nil {
			respUsage.PromptTokens = event.Usage.PromptTokens
			respUsage.CompletionTokens = event.Usage.CompletionTokens
		}
		for _, ch := range event.Choices {
			if ch.Delta.Content == "" {
				continue
			}
			fullContent.WriteString(ch.Delta.Content)
			if err := handler(ch.Delta.Content); err != nil {
				return nil, err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &Response{
		Content:      fullContent.String(),
		PromptTokens: respUsage.PromptTokens,
		CompTokens:   respUsage.CompletionTokens,
		Model:        req.Model,
	}, nil
}

// MockClient is used for testing and zero-cost simulation.
type MockClient struct {
	Handler func(req Request) string
}

func (m *MockClient) Provider() Provider {
	return ProviderMock
}

func (m *MockClient) Complete(ctx context.Context, req Request) (*Response, error) {
	content := "mock output"
	if m.Handler != nil {
		content = m.Handler(req)
	}
	return &Response{
		Content:      content,
		PromptTokens: 10,
		CompTokens:   20,
		Model:        "mock-model",
	}, nil
}

func (m *MockClient) StreamComplete(ctx context.Context, req Request, handler StreamChunkHandler) (*Response, error) {
	resp, err := m.Complete(ctx, req)
	if err != nil {
		return nil, err
	}
	if handler != nil {
		chunks := []string{"参谋长思考中: ", "分析需求事实...", " 识别隐性边界..."}
		for _, c := range chunks {
			_ = handler(c)
		}
	}
	return resp, nil
}
