package event

import "time"

type RawEvent struct {
	EventID   string `json:"event_id"`
	TS        string `json:"ts"`
	Equipment string `json:"equipment"`
	Source    string `json:"source,omitempty"`
	Log       struct {
		Raw string `json:"raw"`
	} `json:"log"`
}

type StageLatency struct {
	Normalize int64 `json:"normalize"`
	Cache     int64 `json:"cache"`
	Model     int64 `json:"model"`
	EdgeLLM   int64 `json:"edge_llm"`
	Publish   int64 `json:"publish"`
}

type InferredEvent struct {
	EventID       string       `json:"event_id"`
	Source        string       `json:"source,omitempty"`
	Equipment     string       `json:"equipment"`
	EventTS       string       `json:"event_ts"`
	InferredAt    string       `json:"inferred_at"`
	RawLog        string       `json:"raw_log,omitempty"`
	TemplateID    string       `json:"template_id"`
	TemplateText  string       `json:"template_text"`
	EdgePred      string       `json:"edge_pred"`
	Severity      int          `json:"severity"`
	Confidence    float64      `json:"confidence"`
	Uncertainty   float64      `json:"uncertainty"`
	Module        string       `json:"module,omitempty"`
	Evidence      []string     `json:"evidence"`
	AdviceCode    string       `json:"advice_code,omitempty"`
	Advice        string       `json:"advice,omitempty"`
	InferencePath string       `json:"inference_path"`
	ModelVersion  string       `json:"model_version,omitempty"`
	RuleVersion   string       `json:"rule_version,omitempty"`
	LatencyMS     int64        `json:"latency_ms"`
	StageLatency  StageLatency `json:"stage_latency_ms"`
}

func (result *InferredEvent) Finish(start time.Time) {
	result.InferredAt = time.Now().UTC().Format(time.RFC3339Nano)
	result.LatencyMS = time.Since(start).Milliseconds()
}
