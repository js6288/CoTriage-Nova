package model

import "time"

type EdgeProviderStatus string

type EdgeProviderType string

const (
	EdgeProviderTypeLlamaCPP   EdgeProviderType = "llama_cpp"
	EdgeProviderTypeOpenAI     EdgeProviderType = "openai_compat"
	EdgeProviderTypeVLLM       EdgeProviderType = "vllm"
	EdgeProviderTypeCustomHTTP EdgeProviderType = "custom_http"
)

const (
	EdgeProviderStatusDraft     EdgeProviderStatus = "draft"
	EdgeProviderStatusPublished EdgeProviderStatus = "published"
	EdgeProviderStatusDisabled  EdgeProviderStatus = "disabled"
	EdgeProviderStatusDeleted   EdgeProviderStatus = "deleted"
)

type EdgeProvider struct {
	ID            int64              `json:"id"`
	Name          string             `json:"name"`
	Provider      EdgeProviderType   `json:"provider"`
	Endpoint      string             `json:"endpoint"`
	Model         string             `json:"model"`
	TimeoutMS     int                `json:"timeout_ms"`
	MaxTokens     int                `json:"max_tokens"`
	Temperature   float64            `json:"temperature"`
	TopP          float64            `json:"top_p"`
	Seed          *int               `json:"seed"`
	HealthPath    *string            `json:"healthcheck_path"`
	ResponsePath  string             `json:"response_path"`
	Status        EdgeProviderStatus `json:"status"`
	IsActive      bool               `json:"is_active"`
	ConfigVersion int64              `json:"config_version"`
	Etag          string             `json:"etag"`
	CreatedAt     time.Time          `json:"created_at"`
	UpdatedAt     time.Time          `json:"updated_at"`
	CreatedBy     *string            `json:"created_by"`
	UpdatedBy     *string            `json:"updated_by"`
	DeletedAt     *time.Time         `json:"deleted_at"`
	LastPublishAt *time.Time         `json:"last_publish_at"`
}

// Requests

type CreateEdgeProviderRequest struct {
	Name         string           `json:"name" binding:"required"`
	Provider     EdgeProviderType `json:"provider" binding:"required"`
	Endpoint     string           `json:"endpoint" binding:"required"`
	Model        string           `json:"model" binding:"required"`
	TimeoutMS    int              `json:"timeout_ms" binding:"required"`
	MaxTokens    int              `json:"max_tokens" binding:"required"`
	Temperature  float64          `json:"temperature" binding:"required"`
	TopP         float64          `json:"top_p" binding:"required"`
	Seed         *int             `json:"seed"`
	HealthPath   *string          `json:"healthcheck_path"`
	ResponsePath string           `json:"response_path" binding:"required"`
}

type PatchEdgeProviderRequest struct {
	Name         *string             `json:"name"`
	Provider     *EdgeProviderType   `json:"provider"`
	Endpoint     *string             `json:"endpoint"`
	Model        *string             `json:"model"`
	TimeoutMS    *int                `json:"timeout_ms"`
	MaxTokens    *int                `json:"max_tokens"`
	Temperature  *float64            `json:"temperature"`
	TopP         *float64            `json:"top_p"`
	Seed         **int               `json:"seed"`             // allow explicit null to clear
	HealthPath   **string            `json:"healthcheck_path"` // allow null to clear
	ResponsePath *string             `json:"response_path"`
	Status       *EdgeProviderStatus `json:"status"`
	IsActive     *bool               `json:"is_active"`
}

// Responses

type ListEdgeProvidersResponse struct {
	Items    []EdgeProvider `json:"items"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
	Total    int            `json:"total"`
}

type EdgeConfigResponse struct {
	ConfigVersion  int64               `json:"config_version"`
	Etag           string              `json:"etag"`
	ActiveProvider *EdgeProvider       `json:"active_provider"`
	Classifier     *ClassifierConfig   `json:"classifier"`
	Rules          EdgeConfigRules     `json:"rules"`
	Templates      EdgeConfigTemplates `json:"templates"`
}

type ClassifierConfig struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Endpoint      string    `json:"endpoint"`
	Model         string    `json:"model"`
	TimeoutMS     int       `json:"timeout_ms"`
	Threshold     float64   `json:"threshold"`
	Status        string    `json:"status"`
	IsActive      bool      `json:"is_active"`
	ConfigVersion int64     `json:"config_version"`
	Etag          string    `json:"etag"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type UpsertClassifierConfigRequest struct {
	Name      string  `json:"name" binding:"required"`
	Endpoint  string  `json:"endpoint" binding:"required"`
	Model     string  `json:"model" binding:"required"`
	TimeoutMS int     `json:"timeout_ms" binding:"required"`
	Threshold float64 `json:"threshold"`
	Status    string  `json:"status"`
}

type ListClassifierConfigsResponse struct {
	Items    []ClassifierConfig `json:"items"`
	Page     int                `json:"page"`
	PageSize int                `json:"page_size"`
	Total    int                `json:"total"`
}
