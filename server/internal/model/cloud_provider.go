package model

import "time"

type CloudProvider struct {
	ID            int64      `json:"id"`
	Name          string     `json:"name"`
	Provider      string     `json:"provider"`
	Endpoint      string     `json:"endpoint"`
	Model         string     `json:"model"`
	APIKeyEnv     string     `json:"api_key_env"`
	APIKey        string     `json:"api_key"`
	TimeoutMS     int        `json:"timeout_ms"`
	MaxTokens     int        `json:"max_tokens"`
	Temperature   float64    `json:"temperature"`
	TopP          float64    `json:"top_p"`
	Seed          *int       `json:"seed"`
	HealthPath    *string    `json:"health_path"`
	ResponsePath  string     `json:"response_path"`
	Status        string     `json:"status"`
	IsActive      bool       `json:"is_active"`
	ConfigVersion int64      `json:"config_version"`
	Etag          string     `json:"etag"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
	LastPublishAt *time.Time `json:"last_publish_at,omitempty"`
}

type CreateCloudProviderRequest struct {
	Name         string  `json:"name" binding:"required"`
	Provider     string  `json:"provider" binding:"required"`
	Endpoint     string  `json:"endpoint" binding:"required"`
	Model        string  `json:"model" binding:"required"`
	APIKeyEnv    string  `json:"api_key_env" binding:"required"`
	APIKey       string  `json:"api_key" binding:"required"`
	TimeoutMS    int     `json:"timeout_ms" binding:"required"`
	MaxTokens    int     `json:"max_tokens" binding:"required"`
	Temperature  float64 `json:"temperature"`
	TopP         float64 `json:"top_p"`
	Seed         *int    `json:"seed"`
	HealthPath   *string `json:"health_path"`
	ResponsePath string  `json:"response_path"`
}

type PatchCloudProviderRequest struct {
	Name         *string  `json:"name"`
	Provider     *string  `json:"provider"`
	Endpoint     *string  `json:"endpoint"`
	Model        *string  `json:"model"`
	APIKeyEnv    *string  `json:"api_key_env"`
	APIKey       *string  `json:"api_key"`
	TimeoutMS    *int     `json:"timeout_ms"`
	MaxTokens    *int     `json:"max_tokens"`
	Temperature  *float64 `json:"temperature"`
	TopP         *float64 `json:"top_p"`
	Seed         **int    `json:"seed"`
	HealthPath   **string `json:"health_path"`
	ResponsePath *string  `json:"response_path"`
	Status       *string  `json:"status"`
}

type CloudConfigResponse struct {
	Enabled       bool              `json:"enabled"`
	Reason        string            `json:"reason,omitempty"`
	ConfigVersion int64             `json:"config_version,omitempty"`
	Etag          string            `json:"etag,omitempty"`
	Provider      *CloudProvider    `json:"provider,omitempty"`
	Prompt        CloudPromptConfig `json:"prompt,omitempty"`
	Reward        CloudRewardConfig `json:"reward,omitempty"`
	GeneratedAt   time.Time         `json:"generated_at"`
}

type CloudPromptConfig struct {
	Version          string `json:"version"`
	RawLogMaxChars   int    `json:"raw_log_max_chars"`
	ContextWindow    string `json:"context_window"`
	ContextMaxEvents int    `json:"context_max_events"`
	ContextMaxTokens int    `json:"context_max_tokens"`
}

type CloudRewardConfig struct {
	LatencySLAMS          int `json:"latency_sla_ms"`
	TokenBudgetPerRequest int `json:"token_budget_per_req"`
}
