package store

import "time"

const (
	OutboxStatusPending = "PENDING"
	OutboxStatusSending = "SENDING"
	OutboxStatusSent    = "SENT"
	OutboxStatusDone    = "DONE"
	OutboxStatusRetry   = "RETRY"
	OutboxStatusDLQ     = "DLQ"
)

type RouterDecisionModel struct {
	ID              int64      `gorm:"primaryKey;autoIncrement"`
	EventID         string     `gorm:"size:128;uniqueIndex;not null"`
	TraceID         string     `gorm:"size:128;index"`
	SchemaVersion   string     `gorm:"size:32"`
	Source          string     `gorm:"size:255;index:idx_router_decisions_source_routed_at,priority:1"`
	Equipment       string     `gorm:"size:255"`
	EventTS         *time.Time `gorm:"index"`
	InferredAt      *time.Time
	TemplateID      string `gorm:"size:128;index:idx_router_decisions_template_routed_at,priority:1"`
	RawLogHash      string `gorm:"size:64"`
	EdgePred        string `gorm:"size:64"`
	Severity        int
	Confidence      float64
	EdgeUncertainty float64
	InferencePath   string `gorm:"size:64"`
	ModelVersion    string `gorm:"size:128"`
	RuleVersion     string `gorm:"size:128"`
	Action          string `gorm:"size:16;index:idx_router_decisions_action_routed_at,priority:1;not null"`
	Reason          string `gorm:"size:128"`
	PolicyRef       string `gorm:"size:128"`
	Score           float64
	Threshold       float64
	CloudHealth     string `gorm:"size:32"`
	RaiseVersion    string `gorm:"size:64"`
	LatencyMS       int64
	RoutedAt        time.Time `gorm:"index;index:idx_router_decisions_action_routed_at,priority:2;index:idx_router_decisions_template_routed_at,priority:2;index:idx_router_decisions_source_routed_at,priority:2;not null"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (RouterDecisionModel) TableName() string {
	return "router_decisions"
}

type RouterStateSnapshotModel struct {
	ID           int64  `gorm:"primaryKey;autoIncrement"`
	SnapshotType string `gorm:"size:64;index:idx_router_state_type_key_active,priority:1;not null"`
	SnapshotKey  string `gorm:"size:255;index:idx_router_state_type_key_active,priority:2;not null"`
	PayloadJSON  string `gorm:"type:longtext;not null"`
	Version      string `gorm:"size:128;index"`
	Checksum     string `gorm:"size:128"`
	IsActive     bool   `gorm:"index:idx_router_state_type_key_active,priority:3"`
	RestoredAt   *time.Time
	MetadataJSON string    `gorm:"type:text"`
	CreatedAt    time.Time `gorm:"index"`
}

func (RouterStateSnapshotModel) TableName() string {
	return "router_state_snapshots"
}

type CloudOutboxModel struct {
	ID                int64   `gorm:"primaryKey;autoIncrement"`
	EventID           string  `gorm:"size:128;uniqueIndex;not null"`
	RequestID         string  `gorm:"size:160;uniqueIndex;not null"`
	IdempotencyKey    string  `gorm:"size:160;uniqueIndex;not null"`
	PayloadJSON       string  `gorm:"type:longtext;not null"`
	RouteDecisionJSON string  `gorm:"type:text"`
	PriorityScore     float64 `gorm:"index:idx_cloud_outbox_status_priority,priority:2"`
	DeferReason       string  `gorm:"size:128"`
	Status            string  `gorm:"size:16;index:idx_cloud_outbox_status_priority,priority:1;index;not null"`
	Attempt           int
	MaxAttempts       int
	NextRetryAt       *time.Time `gorm:"index"`
	DeadlineAt        *time.Time
	CreatedAt         time.Time `gorm:"index"`
	UpdatedAt         time.Time
	LockedBy          string     `gorm:"size:128"`
	LockedUntil       *time.Time `gorm:"index"`
	SentAt            *time.Time
	DoneAt            *time.Time
	LastError         string `gorm:"type:text"`
	LastErrorAt       *time.Time
	DLQReason         string `gorm:"type:text"`
}

func (CloudOutboxModel) TableName() string {
	return "cloud_outbox"
}

type CloudFeedbackModel struct {
	ID                  int64  `gorm:"primaryKey;autoIncrement"`
	EventID             string `gorm:"size:128;uniqueIndex;not null"`
	RequestID           string `gorm:"size:160;index"`
	IdempotencyKey      string `gorm:"size:160;index"`
	Source              string `gorm:"size:255"`
	Equipment           string `gorm:"size:255"`
	TemplateID          string `gorm:"size:128;index"`
	EdgePred            string `gorm:"size:64"`
	EdgeConfidence      float64
	RouteAction         string `gorm:"size:16"`
	RouteScore          float64
	RouteThreshold      float64
	CloudModel          string `gorm:"size:128"`
	CloudPred           string `gorm:"size:64"`
	RootCause           string `gorm:"type:text"`
	Recommendation      string `gorm:"type:text"`
	Explanation         string `gorm:"type:text"`
	CloudConfidence     float64
	CloudLatencyMS      int64
	TokenUsage          int
	Cost                float64
	Reward              float64
	RewardReason        string `gorm:"size:128"`
	CorrectedEdge       bool
	AddedActionableInfo bool
	ConfirmedHighRisk   bool
	FeedbackJSON        string `gorm:"type:longtext"`
	ProcessedByRouter   bool   `gorm:"index"`
	ProcessedAt         *time.Time
	CreatedAt           time.Time `gorm:"index"`
}

func (CloudFeedbackModel) TableName() string {
	return "cloud_feedback"
}
