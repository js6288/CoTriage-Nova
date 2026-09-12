package diagnosis

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/pangjian-pj/cotriage-dev/cloud-gateway/internal/event"
)

var ip = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)
var email = regexp.MustCompile(`[\w.+-]+@[\w.-]+\.[A-Za-z]{2,}`)
var secret = regexp.MustCompile(`(?i)(password|passwd|token|api[_-]?key|secret)\s*[:=]\s*[^\s,;]+`)
var hash = regexp.MustCompile(`\b[A-Fa-f0-9]{32,}\b`)

func Redact(s string) string {
	s = ip.ReplaceAllString(s, "<IP>")
	s = email.ReplaceAllString(s, "<EMAIL>")
	s = secret.ReplaceAllString(s, "$1=<SECRET>")
	return hash.ReplaceAllString(s, "<HASH>")
}
func Prompt(in event.RoutedEvent, related []event.InferredEvent, maxChars int) (string, string) {
	raw := Redact(in.Input.RawLog)
	if len([]rune(raw)) > maxChars {
		raw = string([]rune(raw)[:maxChars])
	}
	var logs []string
	for _, r := range related {
		logs = append(logs, fmt.Sprintf("- %s | %s | %s | severity=%d | %s", r.EventTS, r.Source, r.EdgePred, r.Severity, Redact(r.RawLog)))
	}
	system := "You are a senior AIOps diagnosis assistant for industrial edge-cloud log triage. Return only valid JSON. Do not invent facts. If evidence is insufficient, use UNKNOWN."
	user := fmt.Sprintf("Diagnose this event.\nRaw log: %s\nTemplate: %s\nRelated logs:\n%s\nEquipment: %s\nSource: %s\nEdge: pred=%s severity=%d confidence=%.3f uncertainty=%.3f evidence=%v advice=%s\nRouter: reason=%s score=%.3f threshold=%.3f\nReturn JSON fields pred, root_cause, recommendation, explanation, confidence, evidence.", raw, in.Input.TemplateText, strings.Join(logs, "\n"), in.Input.Equipment, in.Input.Source, in.Input.EdgePred, in.Input.Severity, in.Input.Confidence, in.Input.Uncertainty, in.Input.Evidence, in.Input.Advice, in.Decision.Reason, in.Decision.Score, in.Decision.Threshold)
	return system, user
}
func Feedback(in event.InferredEvent, d event.Diagnosis, u event.Usage, latencySLA, tokenBudget int) event.Feedback {
	disagree := d.Pred != in.EdgePred && d.Pred != "UNKNOWN"
	actionable := d.RootCause != "" && d.Recommendation != ""
	high := in.Severity >= 75 && d.Confidence >= .65
	cg := math.Max(0, d.Confidence-in.Confidence)
	lp := math.Min(1, float64(u.LatencyMS)/float64(max(latencySLA, 1)))
	tp := math.Min(1, float64(u.TotalTokens)/float64(max(tokenBudget, 1)))
	r := boolf(disagree)*.35 + boolf(actionable)*.30 + boolf(high)*.20 + cg*.15 - lp*.10 - tp*.05
	r = math.Max(-1, math.Min(1, r))
	reason := "LOW_INCREMENTAL_GAIN"
	if disagree {
		reason = "CORRECTED_EDGE"
	} else if actionable {
		reason = "ADDED_ACTIONABLE_INFO"
	} else if high {
		reason = "CONFIRMED_HIGH_RISK"
	} else if r < 0 {
		reason = "LOW_UTILITY_OR_HIGH_COST"
	}
	return event.Feedback{disagree, actionable, high, r, reason}
}
func boolf(v bool) float64 {
	if v {
		return 1
	}
	return 0
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
