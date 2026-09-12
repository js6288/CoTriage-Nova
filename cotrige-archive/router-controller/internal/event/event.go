package event

type StageLatency struct {
	Normalize int64 `json:"normalize"`
	Cache     int64 `json:"cache"`
	Model     int64 `json:"model"`
	EdgeLLM   int64 `json:"edge_llm"`
	Publish   int64 `json:"publish"`
}

type InferredEvent struct {
	EventID       string       `json:"event_id"`
	TraceID       string       `json:"trace_id,omitempty"`
	SchemaVersion string       `json:"schema_version,omitempty"`
	Source        string       `json:"source,omitempty"`
	Equipment     string       `json:"equipment,omitempty"`
	EventTS       string       `json:"event_ts,omitempty"`
	InferredAt    string       `json:"inferred_at,omitempty"`
	RawLog        string       `json:"raw_log,omitempty"`
	TemplateID    string       `json:"template_id,omitempty"`
	TemplateText  string       `json:"template_text,omitempty"`
	EdgePred      string       `json:"edge_pred"`
	Severity      int          `json:"severity"`
	Confidence    float64      `json:"confidence"`
	Uncertainty   float64      `json:"uncertainty"`
	Evidence      []string     `json:"evidence,omitempty"`
	AdviceCode    string       `json:"advice_code,omitempty"`
	Advice        string       `json:"advice,omitempty"`
	InferencePath string       `json:"inference_path"`
	ModelVersion  string       `json:"model_version,omitempty"`
	RuleVersion   string       `json:"rule_version,omitempty"`
	LatencyMS     int64        `json:"latency_ms"`
	StageLatency  StageLatency `json:"stage_latency_ms"`
}
