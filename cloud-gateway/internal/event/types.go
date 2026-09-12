package event

type InferredEvent struct {
	EventID      string   `json:"event_id"`
	Source       string   `json:"source"`
	Equipment    string   `json:"equipment"`
	EventTS      string   `json:"event_ts"`
	RawLog       string   `json:"raw_log"`
	TemplateID   string   `json:"template_id"`
	TemplateText string   `json:"template_text"`
	EdgePred     string   `json:"edge_pred"`
	Severity     int      `json:"severity"`
	Confidence   float64  `json:"confidence"`
	Uncertainty  float64  `json:"uncertainty"`
	Evidence     []string `json:"evidence"`
	AdviceCode   string   `json:"advice_code"`
	Advice       string   `json:"advice"`
}
type Decision struct {
	Action    string  `json:"action"`
	Reason    string  `json:"reason"`
	Score     float64 `json:"score"`
	Threshold float64 `json:"threshold"`
}
type RoutedEvent struct {
	Input       InferredEvent  `json:"input"`
	Decision    Decision       `json:"decision"`
	SystemState map[string]any `json:"system_state"`
}
type Request struct {
	EventID, RequestID, IdempotencyKey string
	Routed                             RoutedEvent
}
type Diagnosis struct {
	Pred           string   `json:"pred"`
	RootCause      string   `json:"root_cause"`
	Recommendation string   `json:"recommendation"`
	Explanation    string   `json:"explanation"`
	Confidence     float64  `json:"confidence"`
	Evidence       []string `json:"evidence"`
}
type Usage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	Cost             float64 `json:"cost"`
	LatencyMS        int64   `json:"latency_ms"`
}
type Feedback struct {
	CorrectedEdge       bool    `json:"corrected_edge"`
	AddedActionableInfo bool    `json:"added_actionable_info"`
	ConfirmedHighRisk   bool    `json:"confirmed_high_risk"`
	Reward              float64 `json:"reward"`
	RewardReason        string  `json:"reward_reason"`
}
type EdgeSummary struct {
	Pred       string  `json:"pred"`
	Confidence float64 `json:"confidence"`
	Severity   int     `json:"severity"`
	AdviceCode string  `json:"advice_code"`
}
type Response struct {
	SchemaVersion  string       `json:"schema_version"`
	EventID        string       `json:"event_id"`
	RequestID      string       `json:"request_id"`
	IdempotencyKey string       `json:"idempotency_key"`
	Status         string       `json:"status"`
	Edge           *EdgeSummary `json:"edge,omitempty"`
	Cloud          *Diagnosis   `json:"cloud,omitempty"`
	Feedback       *Feedback    `json:"feedback,omitempty"`
	Usage          *Usage       `json:"usage,omitempty"`
	Context        any          `json:"context,omitempty"`
	ErrorCode      string       `json:"error_code,omitempty"`
	ErrorMessage   string       `json:"error_message,omitempty"`
	Retryable      bool         `json:"retryable,omitempty"`
	ProcessedAt    string       `json:"processed_at"`
}
