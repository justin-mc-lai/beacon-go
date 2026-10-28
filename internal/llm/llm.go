package llm

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Provider identifies the supported LLM backend.
type Provider string

const (
	ProviderOpenCodex Provider = "opencodex"
	ProviderOpenAI    Provider = "openai"
	ProviderDeepSeek  Provider = "deepseek"
	ProviderAnthropic Provider = "anthropic"
	ProviderGemini    Provider = "gemini"
	ProviderMock      Provider = "mock"
)

// Role defines the Agent persona inside the Beacon ecosystem.
type Role string

const (
	RoleChiefOfStaff Role = "chief_of_staff"
	RoleArchitect    Role = "architect"
	RoleCoder        Role = "coder"
	RoleQA           Role = "qa"
)

// Message is a chat turn in standard structure.
type Message struct {
	Role    string `json:"role"` // system, user, assistant
	Content string `json:"content"`
}

// Request is the invocation payload passed to the Client.
type Request struct {
	Role        Role      `json:"role"`
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

// Response holds the generated text and token stats.
type Response struct {
	Content      string `json:"content"`
	PromptTokens int    `json:"prompt_tokens"`
	CompTokens   int    `json:"completion_tokens"`
	Model        string `json:"model"`
}

// Client defines the interface for calling any LLM provider.
type Client interface {
	Complete(ctx context.Context, req Request) (*Response, error)
	Provider() Provider
}

// RoleConfig configures which model and provider a specific role uses.
type RoleConfig struct {
	Provider    Provider `json:"provider" yaml:"provider"`
	Model       string   `json:"model" yaml:"model"`
	Temperature float64  `json:"temperature" yaml:"temperature"`
	MaxTokens   int      `json:"max_tokens" yaml:"max_tokens"`
}

// Config represents the BYOK configuration for Beacon.
type Config struct {
	Version   string                      `json:"version" yaml:"version"`
	Providers map[Provider]ProviderConfig `json:"providers" yaml:"providers"`
	Roles     map[Role]RoleConfig         `json:"roles" yaml:"roles"`
}

// ProviderConfig stores credentials and base URLs.
type ProviderConfig struct {
	BaseURL string `json:"base_url" yaml:"base_url"`
	APIKey  string `json:"api_key" yaml:"api_key"`
}

// LoadConfig attempts to read ~/.beacon/config.yaml, falling back to DefaultConfig.
func LoadConfig() (*Config, error) {
	home, err := os.UserHomeDir()
	if err == nil {
		path := filepath.Join(home, ".beacon", "config.yaml")
		data, err := os.ReadFile(path)
		if err == nil {
			var cfg Config
			if err := yaml.Unmarshal(data, &cfg); err == nil {
				return &cfg, nil
			}
		}
	}
	return DefaultConfig(), nil
}

// SaveConfig persists the current configuration to ~/.beacon/config.yaml.
func (c *Config) SaveConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".beacon")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0644)
}

// DefaultConfig provides a robust fallback mapping.
func DefaultConfig() *Config {
	return &Config{
		Version: "v2",
		Providers: map[Provider]ProviderConfig{
			ProviderOpenCodex: {
				BaseURL: "http://127.0.0.1:10100/v1",
				APIKey:  "tuzki-local",
			},
			ProviderDeepSeek: {
				BaseURL: "https://api.deepseek.com/v1",
				APIKey:  os.Getenv("DEEPSEEK_API_KEY"),
			},
			ProviderOpenAI: {
				BaseURL: "https://api.openai.com/v1",
				APIKey:  os.Getenv("OPENAI_API_KEY"),
			},
		},
		Roles: map[Role]RoleConfig{
			RoleChiefOfStaff: {
				Provider:    ProviderOpenCodex,
				Model:       "deepseek/deepseek-chat",
				Temperature: 0.1,
				MaxTokens:   4096,
			},
			RoleArchitect: {
				Provider:    ProviderOpenCodex,
				Model:       "deepseek/deepseek-chat",
				Temperature: 0.0,
				MaxTokens:   8192,
			},
			RoleCoder: {
				Provider:    ProviderOpenCodex,
				Model:       "deepseek/deepseek-chat",
				Temperature: 0.2,
				MaxTokens:   8192,
			},
			RoleQA: {
				Provider:    ProviderOpenCodex,
				Model:       "deepseek/deepseek-chat",
				Temperature: 0.0,
				MaxTokens:   4096,
			},
		},
	}
}

// ResolveAPIKey extracts the key from config or environment.
func (c *Config) ResolveAPIKey(p Provider) string {
	cfg, ok := c.Providers[p]
	if ok && cfg.APIKey != "" {
		if strings.HasPrefix(cfg.APIKey, "${") && strings.HasSuffix(cfg.APIKey, "}") {
			envVar := strings.TrimSuffix(strings.TrimPrefix(cfg.APIKey, "${"), "}")
			return os.Getenv(envVar)
		}
		return cfg.APIKey
	}
	switch p {
	case ProviderOpenAI:
		return os.Getenv("OPENAI_API_KEY")
	case ProviderDeepSeek:
		return os.Getenv("DEEPSEEK_API_KEY")
	case ProviderAnthropic:
		return os.Getenv("ANTHROPIC_API_KEY")
	case ProviderGemini:
		return os.Getenv("GEMINI_API_KEY")
	case ProviderOpenCodex:
		return "tuzki-local"
	}
	return ""
}

// GetClientForRole builds a Client using the configured Provider and Model.
func (c *Config) GetClientForRole(role Role) (Client, string, error) {
	rc, ok := c.Roles[role]
	if !ok {
		// Fallback to ChiefOfStaff
		rc = c.Roles[RoleChiefOfStaff]
	}
	pCfg, ok := c.Providers[rc.Provider]
	if !ok {
		pCfg = ProviderConfig{
			BaseURL: "http://127.0.0.1:10100/v1",
			APIKey:  "tuzki-local",
		}
	}
	key := c.ResolveAPIKey(rc.Provider)
	if key != "" {
		pCfg.APIKey = key
	}
	cli, err := NewClient(rc.Provider, pCfg)
	return cli, rc.Model, err
}

// NewClient returns a provider client implementation.
func NewClient(p Provider, cfg ProviderConfig) (Client, error) {
	switch p {
	case ProviderOpenCodex, ProviderOpenAI, ProviderDeepSeek:
		return NewOpenAICompatibleClient(p, cfg.BaseURL, cfg.APIKey), nil
	case ProviderMock:
		return &MockClient{}, nil
	default:
		// Default to OpenAI compatible
		return NewOpenAICompatibleClient(p, cfg.BaseURL, cfg.APIKey), nil
	}
}
