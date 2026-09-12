package store

import "time"

type RouterDecisionModel struct {
	ID              int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	EventID         string     `json:"event_id"`
	TraceID         string     `json:"trace_id"`
	SchemaVersion   string     `json:"schema_version"`
	Source          string     `json:"source"`
	Equipment       string     `json:"equipment"`
	EventTS         *time.Time `json:"event_ts"`
	InferredAt      *time.Time `json:"inferred_at"`
	TemplateID      string     `json:"template_id"`
	RawLogHash      string     `json:"raw_log_hash"`
	EdgePred        string     `json:"edge_pred"`
	Severity        int        `json:"severity"`
	Confidence      float64    `json:"confidence"`
	EdgeUncertainty float64    `json:"edge_uncertainty"`
	InferencePath   string     `json:"inference_path"`
	ModelVersion    string     `json:"model_version"`
	RuleVersion     string     `json:"rule_version"`
	Action          string     `json:"action"`
	Reason          string     `json:"reason"`
	PolicyRef       string     `json:"policy_ref"`
	Score           float64    `json:"score"`
	Threshold       float64    `json:"threshold"`
	CloudHealth     string     `json:"cloud_health"`
	RaiseVersion    string     `json:"raise_version"`
	LatencyMS       int64      `json:"latency_ms"`
	RoutedAt        time.Time  `json:"routed_at"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (RouterDecisionModel) TableName() string {
	return "router_decisions"
}

type RouterStateSnapshotModel struct {
	ID           int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	SnapshotType string     `json:"snapshot_type"`
	SnapshotKey  string     `json:"snapshot_key"`
	PayloadJSON  string     `json:"payload_json"`
	Version      string     `json:"version"`
	Checksum     string     `json:"checksum"`
	IsActive     bool       `json:"is_active"`
	RestoredAt   *time.Time `json:"restored_at"`
	MetadataJSON string     `json:"metadata_json"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (RouterStateSnapshotModel) TableName() string {
	return "router_state_snapshots"
}

type CloudOutboxModel struct {
	ID                int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	EventID           string     `json:"event_id"`
	RequestID         string     `json:"request_id"`
	IdempotencyKey    string     `json:"idempotency_key"`
	PayloadJSON       string     `json:"payload_json"`
	RouteDecisionJSON string     `json:"route_decision_json"`
	PriorityScore     float64    `json:"priority_score"`
	DeferReason       string     `json:"defer_reason"`
	Status            string     `json:"status"`
	Attempt           int        `json:"attempt"`
	MaxAttempts       int        `json:"max_attempts"`
	NextRetryAt       *time.Time `json:"next_retry_at"`
	DeadlineAt        *time.Time `json:"deadline_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	LockedBy          string     `json:"locked_by"`
	LockedUntil       *time.Time `json:"locked_until"`
	SentAt            *time.Time `json:"sent_at"`
	DoneAt            *time.Time `json:"done_at"`
	LastError         string     `json:"last_error"`
	LastErrorAt       *time.Time `json:"last_error_at"`
	DLQReason         string     `json:"dlq_reason"`
}

func (CloudOutboxModel) TableName() string {
	return "cloud_outbox"
}

type CloudFeedbackModel struct {
	ID                  int64      `gorm:"primaryKey;autoIncrement" json:"id"`
	EventID             string     `json:"event_id"`
	RequestID           string     `json:"request_id"`
	Source              string     `json:"source"`
	Equipment           string     `json:"equipment"`
	TemplateID          string     `json:"template_id"`
	EdgePred            string     `json:"edge_pred"`
	EdgeConfidence      float64    `json:"edge_confidence"`
	CloudModel          string     `json:"cloud_model"`
	CloudPred           string     `json:"cloud_pred"`
	RootCause           string     `json:"root_cause"`
	Recommendation      string     `json:"recommendation"`
	Explanation         string     `json:"explanation"`
	CloudConfidence     float64    `json:"cloud_confidence"`
	CloudLatencyMS      int64      `json:"cloud_latency_ms"`
	TokenUsage          int        `json:"token_usage"`
	Cost                float64    `json:"cost"`
	Reward              float64    `json:"reward"`
	RewardReason        string     `json:"reward_reason"`
	CorrectedEdge       bool       `json:"corrected_edge"`
	AddedActionableInfo bool       `json:"added_actionable_info"`
	ConfirmedHighRisk   bool       `json:"confirmed_high_risk"`
	ProcessedByRouter   bool       `json:"processed_by_router"`
	ProcessedAt         *time.Time `json:"processed_at"`
	CreatedAt           time.Time  `json:"created_at"`
}

func (CloudFeedbackModel) TableName() string { return "cloud_feedback" }
