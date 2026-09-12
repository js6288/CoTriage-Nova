package contextlog

import (
	"context"
	"encoding/json"
	"math"
	"sort"
	"strconv"
	"time"

	"github.com/pangjian-pj/cotriage-dev/cloud-gateway/internal/event"
	"github.com/redis/go-redis/v9"
)

type scored struct {
	event.InferredEvent
	score float64
}
type Summary struct {
	EventCount   int      `json:"event_count"`
	TopTemplates []string `json:"top_templates"`
	TopSources   []string `json:"top_sources"`
	TopPreds     []string `json:"top_preds"`
	MaxSeverity  int      `json:"max_severity"`
	TimeSpanMS   int64    `json:"time_span_ms"`
}
type Context struct {
	CenterEvent   event.InferredEvent   `json:"center_event"`
	RelatedEvents []event.InferredEvent `json:"related_events"`
	Aggregate     Summary               `json:"aggregate"`
}
type Aggregator struct {
	Redis                *redis.Client
	Stream               string
	Window               time.Duration
	MaxEvents, MaxTokens int
}

func (a Aggregator) Build(ctx context.Context, center event.InferredEvent) Context {
	out := Context{CenterEvent: center}
	if a.Redis == nil {
		return out
	}
	ct := parse(center.EventTS)
	start := "-"
	if !ct.IsZero() {
		start = strconv.FormatInt(ct.Add(-a.Window).UnixMilli(), 10) + "-0"
	}
	xs, e := a.Redis.XRangeN(ctx, a.Stream, start, "+", int64(max(a.MaxEvents*20, 100))).Result()
	if e != nil {
		return out
	}
	var candidates []scored
	for _, x := range xs {
		raw, _ := x.Values["payload"].(string)
		if raw == "" {
			raw, _ = x.Values["payload_json"].(string)
		}
		var v event.InferredEvent
		if json.Unmarshal([]byte(raw), &v) != nil || v.EventID == center.EventID {
			continue
		}
		ts := parse(v.EventTS)
		if !ct.IsZero() && (ts.IsZero() || ts.Before(ct.Add(-a.Window)) || ts.After(ct.Add(a.Window))) {
			continue
		}
		delta := math.Abs(float64(ts.Sub(ct).Milliseconds()))
		timeScore := math.Exp(-delta / math.Max(float64(a.Window.Milliseconds()), 1))
		s := .25*timeScore + .2*same(v.TemplateID, center.TemplateID) + .2*same(v.Source, center.Source) + .15*same(v.Equipment, center.Equipment) + .1*same(v.EdgePred, center.EdgePred) + .1*float64(v.Severity)/100
		candidates = append(candidates, scored{v, s})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	limit := min(a.MaxEvents, len(candidates))
	budget := a.MaxTokens * 4
	for _, v := range candidates[:limit] {
		if budget <= 0 {
			break
		}
		if len(v.RawLog) > budget {
			v.RawLog = v.RawLog[:budget]
		}
		budget -= len(v.RawLog)
		out.RelatedEvents = append(out.RelatedEvents, v.InferredEvent)
	}
	out.Aggregate = summarize(out.RelatedEvents)
	return out
}
func parse(s string) time.Time { t, _ := time.Parse(time.RFC3339Nano, s); return t }
func same(a, b string) float64 {
	if a != "" && a == b {
		return 1
	}
	return 0
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func summarize(es []event.InferredEvent) Summary {
	s := Summary{EventCount: len(es)}
	tm, sm, pm := map[string]int{}, map[string]int{}, map[string]int{}
	var lo, hi time.Time
	for _, e := range es {
		tm[e.TemplateID]++
		sm[e.Source]++
		pm[e.EdgePred]++
		if e.Severity > s.MaxSeverity {
			s.MaxSeverity = e.Severity
		}
		t := parse(e.EventTS)
		if lo.IsZero() || t.Before(lo) {
			lo = t
		}
		if hi.IsZero() || t.After(hi) {
			hi = t
		}
	}
	s.TopTemplates = top(tm, 3)
	s.TopSources = top(sm, 3)
	s.TopPreds = top(pm, 3)
	if !lo.IsZero() {
		s.TimeSpanMS = hi.Sub(lo).Milliseconds()
	}
	return s
}
func top(m map[string]int, n int) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		if k != "" {
			ks = append(ks, k)
		}
	}
	sort.Slice(ks, func(i, j int) bool { return m[ks[i]] > m[ks[j]] })
	if len(ks) > n {
		ks = ks[:n]
	}
	return ks
}
