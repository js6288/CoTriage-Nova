package model

import "time"

type KnowledgeStatus string

const (
	KnowledgeStatusDraft     KnowledgeStatus = "draft"
	KnowledgeStatusPublished KnowledgeStatus = "published"
	KnowledgeStatusDisabled  KnowledgeStatus = "disabled"
	KnowledgeStatusDeleted   KnowledgeStatus = "deleted"
)

type SetStatus string

const (
	SetStatusDraft     SetStatus = "draft"
	SetStatusPublished SetStatus = "published"
	SetStatusActive    SetStatus = "active"
	SetStatusArchived  SetStatus = "archived"
)

type EdgeRule struct {
	ID          int64           `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	MatchType   string          `json:"match_type"`
	Pattern     string          `json:"pattern"`
	Keywords    []string        `json:"keywords"`
	TargetField string          `json:"target_field"`
	EdgePred    string          `json:"edge_pred"`
	Severity    int             `json:"severity"`
	Confidence  float64         `json:"confidence"`
	Evidence    []string        `json:"evidence"`
	AdviceCode  string          `json:"advice_code"`
	Advice      string          `json:"advice"`
	Priority    int             `json:"priority"`
	Enabled     bool            `json:"enabled"`
	Status      KnowledgeStatus `json:"status"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

type UpsertEdgeRuleRequest struct {
	Name        string   `json:"name" binding:"required"`
	Description string   `json:"description"`
	MatchType   string   `json:"match_type" binding:"required"`
	Pattern     string   `json:"pattern"`
	Keywords    []string `json:"keywords"`
	TargetField string   `json:"target_field"`
	EdgePred    string   `json:"edge_pred" binding:"required"`
	Severity    int      `json:"severity"`
	Confidence  float64  `json:"confidence"`
	Evidence    []string `json:"evidence"`
	AdviceCode  string   `json:"advice_code"`
	Advice      string   `json:"advice"`
	Priority    int      `json:"priority"`
	Enabled     bool     `json:"enabled"`
	Status      string   `json:"status"`
}

type EdgeTemplate struct {
	ID           int64           `json:"id"`
	TemplateID   string          `json:"template_id"`
	TemplateText string          `json:"template_text"`
	RawExample   string          `json:"raw_example"`
	Source       string          `json:"source"`
	EdgePred     string          `json:"edge_pred"`
	Severity     int             `json:"severity"`
	Confidence   float64         `json:"confidence"`
	Evidence     []string        `json:"evidence"`
	AdviceCode   string          `json:"advice_code"`
	Advice       string          `json:"advice"`
	Status       KnowledgeStatus `json:"status"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

type UpsertEdgeTemplateRequest struct {
	TemplateText string   `json:"template_text" binding:"required"`
	RawExample   string   `json:"raw_example"`
	Source       string   `json:"source"`
	EdgePred     string   `json:"edge_pred" binding:"required"`
	Severity     int      `json:"severity"`
	Confidence   float64  `json:"confidence"`
	Evidence     []string `json:"evidence"`
	AdviceCode   string   `json:"advice_code"`
	Advice       string   `json:"advice"`
	Status       string   `json:"status"`
}

type CreateTemplateFromClusterRequest struct {
	TemplateText  string   `json:"template_text" binding:"required"`
	SampleRawLog  string   `json:"sample_raw_log"`
	Source        string   `json:"source"`
	EdgePred      string   `json:"edge_pred" binding:"required"`
	MaxSeverity   int      `json:"max_severity"`
	AvgConfidence float64  `json:"avg_confidence"`
	Evidence      []string `json:"evidence"`
	AdviceCode    string   `json:"advice_code"`
	Advice        string   `json:"advice"`
	Status        string   `json:"status"`
}

type ListKnowledgeResponse[T any] struct {
	Items    []T `json:"items"`
	Page     int `json:"page"`
	PageSize int `json:"page_size"`
	Total    int `json:"total"`
}

type EdgeRuleSet struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Status        SetStatus `json:"status"`
	ConfigVersion int64     `json:"config_version"`
	Etag          string    `json:"etag"`
	ItemCount     int       `json:"item_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type EdgeTemplateSet EdgeRuleSet

type CreateSetRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description string  `json:"description"`
	ItemIDs     []int64 `json:"item_ids"`
}

type EdgeConfigRules struct {
	Version int64      `json:"version"`
	Etag    string     `json:"etag"`
	Items   []EdgeRule `json:"items"`
}

type EdgeConfigTemplates struct {
	Version int64          `json:"version"`
	Etag    string         `json:"etag"`
	Items   []EdgeTemplate `json:"items"`
}
