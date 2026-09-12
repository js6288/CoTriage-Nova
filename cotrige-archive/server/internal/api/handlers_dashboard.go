package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	redis "github.com/redis/go-redis/v9"
)

type inferredEvent struct {
	EventID       string           `json:"event_id"`
	Source        string           `json:"source,omitempty"`
	Equipment     string           `json:"equipment"`
	EventTS       string           `json:"event_ts"`
	InferredAt    string           `json:"inferred_at"`
	RawLog        string           `json:"raw_log,omitempty"`
	TemplateID    string           `json:"template_id"`
	TemplateText  string           `json:"template_text"`
	EdgePred      string           `json:"edge_pred"`
	Severity      int              `json:"severity"`
	Confidence    float64          `json:"confidence"`
	Uncertainty   float64          `json:"uncertainty"`
	Evidence      []string         `json:"evidence"`
	AdviceCode    string           `json:"advice_code,omitempty"`
	Advice        string           `json:"advice,omitempty"`
	InferencePath string           `json:"inference_path"`
	ModelVersion  string           `json:"model_version,omitempty"`
	RuleVersion   string           `json:"rule_version,omitempty"`
	LatencyMS     int64            `json:"latency_ms"`
	StageLatency  map[string]int64 `json:"stage_latency_ms"`
}

type dashboardSummary struct {
	RawTotal       int64          `json:"raw_total"`
	InferredTotal  int64          `json:"inferred_total"`
	Pending        int64          `json:"pending"`
	DLQTotal       int64          `json:"dlq_total"`
	IngestRate1M   int            `json:"ingest_rate_1m"`
	InferRate1M    int            `json:"infer_rate_1m"`
	P95LatencyMS   int64          `json:"p95_latency_ms"`
	AbnormalRate   float64        `json:"abnormal_rate"`
	FallbackRate   float64        `json:"fallback_rate"`
	Classification map[string]int `json:"classification"`
	Paths          map[string]int `json:"paths"`
	GeneratedAt    string         `json:"generated_at"`
}

type timeseriesPoint struct {
	Time     string `json:"time"`
	Inferred int    `json:"inferred"`
	Abnormal int    `json:"abnormal"`
}

type abnormalAnalysis struct {
	RiskLevel     string            `json:"risk_level"`
	Abnormal1M    int               `json:"abnormal_1m"`
	Abnormal5M    int               `json:"abnormal_5m"`
	GrowthRatio   float64           `json:"growth_ratio"`
	TopTypes      map[string]int    `json:"top_types"`
	TopClusters   []abnormalCluster `json:"top_clusters"`
	PriorityQueue []abnormalCluster `json:"priority_queue"`
	GeneratedAt   string            `json:"generated_at"`
}

type abnormalCluster struct {
	ClusterID          string   `json:"cluster_id"`
	EdgePred           string   `json:"edge_pred"`
	TemplateID         string   `json:"template_id"`
	TemplateText       string   `json:"template_text"`
	SampleRawLog       string   `json:"sample_raw_log"`
	Count              int      `json:"count"`
	AffectedEquipments int      `json:"affected_equipments"`
	Equipments         []string `json:"equipments"`
	InferencePaths     []string `json:"inference_paths"`
	PrimaryPath        string   `json:"primary_inference_path"`
	MaxSeverity        int      `json:"max_severity"`
	AvgConfidence      float64  `json:"avg_confidence"`
	FirstSeen          string   `json:"first_seen"`
	LastSeen           string   `json:"last_seen"`
	Advice             string   `json:"advice,omitempty"`
	AdviceCode         string   `json:"advice_code,omitempty"`
	Evidence           []string `json:"evidence,omitempty"`
	PriorityScore      float64  `json:"priority_score"`
}

func (h *Handler) GetDashboardSummary(c *gin.Context) {
	ctx := c.Request.Context()
	events, err := h.recentInferredEvents(ctx, 500)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}

	rawTotal, _ := h.Redis.XLen(ctx, h.Cfg.RedisStream).Result()
	inferredTotal, _ := h.Redis.XLen(ctx, h.Cfg.InferredStream).Result()
	dlqTotal, _ := h.Redis.XLen(ctx, h.Cfg.DLQStream).Result()
	pending := int64(0)
	if groups, groupErr := h.Redis.XInfoGroups(ctx, h.Cfg.RedisStream).Result(); groupErr == nil {
		for _, group := range groups {
			if group.Name == "edge-infer-g" {
				pending = group.Pending
				break
			}
		}
	}

	now := time.Now().UTC()
	latencies := make([]int64, 0, len(events))
	classification := make(map[string]int)
	paths := make(map[string]int)
	inferRate := 0
	abnormal := 0
	fallback := 0
	for _, item := range events {
		classification[item.EdgePred]++
		paths[item.InferencePath]++
		latencies = append(latencies, item.LatencyMS)
		if item.EdgePred != "NORMAL" {
			abnormal++
		}
		if item.InferencePath == "FALLBACK" {
			fallback++
		}
		if ts, parseErr := time.Parse(time.RFC3339Nano, item.InferredAt); parseErr == nil && now.Sub(ts) <= time.Minute {
			inferRate++
		}
	}

	summary := dashboardSummary{
		RawTotal: rawTotal, InferredTotal: inferredTotal, Pending: pending, DLQTotal: dlqTotal,
		IngestRate1M: countRecentStreamEntries(ctx, h.Redis, h.Cfg.RedisStream, now.Add(-time.Minute)),
		InferRate1M:  inferRate, P95LatencyMS: percentile95(latencies),
		Classification: classification, Paths: paths, GeneratedAt: now.Format(time.RFC3339Nano),
	}
	if len(events) > 0 {
		summary.AbnormalRate = float64(abnormal) / float64(len(events))
		summary.FallbackRate = float64(fallback) / float64(len(events))
	}
	c.JSON(http.StatusOK, summary)
}

func (h *Handler) GetDashboardTimeseries(c *gin.Context) {
	events, err := h.recentInferredEvents(c.Request.Context(), 1000)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	now := time.Now().UTC().Truncate(time.Minute)
	points := make([]timeseriesPoint, 10)
	index := make(map[string]int, len(points))
	for i := range points {
		ts := now.Add(time.Duration(i-9) * time.Minute)
		points[i].Time = ts.Format(time.RFC3339)
		index[ts.Format(time.RFC3339)] = i
	}
	for _, item := range events {
		ts, parseErr := time.Parse(time.RFC3339Nano, item.InferredAt)
		if parseErr != nil {
			continue
		}
		key := ts.UTC().Truncate(time.Minute).Format(time.RFC3339)
		if i, ok := index[key]; ok {
			points[i].Inferred++
			if item.EdgePred != "NORMAL" {
				points[i].Abnormal++
			}
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": points})
}

func (h *Handler) GetAbnormalAnalysis(c *gin.Context) {
	events, err := h.recentInferredEvents(c.Request.Context(), 2000)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	analysis := buildAbnormalAnalysis(events, time.Now().UTC())
	c.JSON(http.StatusOK, analysis)
}

func (h *Handler) ListEvents(c *gin.Context) {
	limit := 50
	if value, err := strconv.Atoi(c.Query("limit")); err == nil && value > 0 && value <= 200 {
		limit = value
	}
	events, err := h.recentInferredEvents(c.Request.Context(), int64(limit))
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": events, "total": len(events)})
}

func (h *Handler) StreamEvents(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(http.StatusOK)
	c.Writer.Flush()

	lastID := c.DefaultQuery("last_id", "$")
	for {
		streams, err := h.Redis.XRead(c.Request.Context(), &redis.XReadArgs{
			Streams: []string{h.Cfg.InferredStream, lastID}, Count: 20, Block: 15 * time.Second,
		}).Result()
		if err == redis.Nil {
			_, _ = c.Writer.WriteString(": keepalive\n\n")
			c.Writer.Flush()
			continue
		}
		if err != nil {
			if errors.Is(err, context.Canceled) || c.Request.Context().Err() != nil {
				return
			}
			return
		}
		for _, stream := range streams {
			for _, message := range stream.Messages {
				item, decodeErr := decodeInferredMessage(message)
				if decodeErr != nil {
					continue
				}
				lastID = message.ID
				body, _ := json.Marshal(item)
				_, _ = c.Writer.WriteString("id: " + message.ID + "\n")
				_, _ = c.Writer.WriteString("event: inferred\n")
				_, _ = c.Writer.WriteString("data: " + string(body) + "\n\n")
				c.Writer.Flush()
			}
		}
	}
}

func (h *Handler) recentInferredEvents(ctx context.Context, limit int64) ([]inferredEvent, error) {
	messages, err := h.Redis.XRevRangeN(ctx, h.Cfg.InferredStream, "+", "-", limit).Result()
	if err != nil {
		return nil, err
	}
	events := make([]inferredEvent, 0, len(messages))
	for _, message := range messages {
		item, decodeErr := decodeInferredMessage(message)
		if decodeErr == nil {
			events = append(events, item)
		}
	}
	return events, nil
}

func decodeInferredMessage(message redis.XMessage) (inferredEvent, error) {
	payload, ok := message.Values["payload"].(string)
	if !ok {
		return inferredEvent{}, errors.New("payload is missing")
	}
	var item inferredEvent
	if err := json.Unmarshal([]byte(payload), &item); err != nil {
		return inferredEvent{}, err
	}
	return item, nil
}

func countRecentStreamEntries(ctx context.Context, rdb *redis.Client, stream string, since time.Time) int {
	start := strconv.FormatInt(since.UnixMilli(), 10) + "-0"
	messages, err := rdb.XRangeN(ctx, stream, start, "+", 2000).Result()
	if err != nil {
		return 0
	}
	return len(messages)
}

func percentile95(values []int64) int64 {
	if len(values) == 0 {
		return 0
	}
	sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
	index := (95*len(values) + 99) / 100
	if index < 1 {
		index = 1
	}
	return values[index-1]
}

func buildAbnormalAnalysis(events []inferredEvent, now time.Time) abnormalAnalysis {
	type clusterAgg struct {
		cluster       abnormalCluster
		equipments    map[string]struct{}
		paths         map[string]int
		confidenceSum float64
		firstSeenTime time.Time
		lastSeenTime  time.Time
		hasFirstSeen  bool
		hasLastSeen   bool
	}

	analysis := abnormalAnalysis{
		RiskLevel:   "LOW",
		TopTypes:    map[string]int{},
		GeneratedAt: now.Format(time.RFC3339Nano),
	}
	clusters := map[string]*clusterAgg{}
	previous5M := 0
	for _, item := range events {
		if item.EdgePred == "NORMAL" {
			continue
		}
		ts, parseErr := time.Parse(time.RFC3339Nano, item.InferredAt)
		if parseErr != nil {
			continue
		}
		age := now.Sub(ts)
		if age < 0 {
			age = 0
		}
		if age > 10*time.Minute {
			continue
		}
		if age <= time.Minute {
			analysis.Abnormal1M++
		}
		if age <= 5*time.Minute {
			analysis.Abnormal5M++
		} else {
			previous5M++
		}
		analysis.TopTypes[item.EdgePred]++

		key := item.TemplateID + ":" + item.EdgePred
		agg, ok := clusters[key]
		if !ok {
			agg = &clusterAgg{
				cluster: abnormalCluster{
					ClusterID: key, EdgePred: item.EdgePred, TemplateID: item.TemplateID,
					TemplateText: item.TemplateText, SampleRawLog: item.RawLog,
					Advice: item.Advice, AdviceCode: item.AdviceCode, Evidence: item.Evidence,
				},
				equipments: map[string]struct{}{},
				paths:      map[string]int{},
			}
			clusters[key] = agg
		}
		agg.cluster.Count++
		if item.Equipment != "" {
			agg.equipments[item.Equipment] = struct{}{}
		}
		if item.InferencePath != "" {
			agg.paths[item.InferencePath]++
		}
		if item.Severity > agg.cluster.MaxSeverity {
			agg.cluster.MaxSeverity = item.Severity
		}
		agg.confidenceSum += item.Confidence
		if item.RawLog != "" && agg.cluster.SampleRawLog == "" {
			agg.cluster.SampleRawLog = item.RawLog
		}
		if item.Advice != "" && agg.cluster.Advice == "" {
			agg.cluster.Advice = item.Advice
		}
		if item.AdviceCode != "" && agg.cluster.AdviceCode == "" {
			agg.cluster.AdviceCode = item.AdviceCode
		}
		if len(item.Evidence) > 0 && len(agg.cluster.Evidence) == 0 {
			agg.cluster.Evidence = item.Evidence
		}
		if !agg.hasFirstSeen || ts.Before(agg.firstSeenTime) {
			agg.firstSeenTime = ts
			agg.hasFirstSeen = true
		}
		if !agg.hasLastSeen || ts.After(agg.lastSeenTime) {
			agg.lastSeenTime = ts
			agg.hasLastSeen = true
		}
	}
	if previous5M == 0 {
		if analysis.Abnormal5M > 0 {
			analysis.GrowthRatio = float64(analysis.Abnormal5M)
		}
	} else {
		analysis.GrowthRatio = float64(analysis.Abnormal5M) / float64(previous5M)
	}

	items := make([]abnormalCluster, 0, len(clusters))
	for _, agg := range clusters {
		cluster := agg.cluster
		cluster.AffectedEquipments = len(agg.equipments)
		cluster.Equipments = sortedKeys(agg.equipments)
		cluster.InferencePaths = sortedPathKeys(agg.paths)
		if len(cluster.InferencePaths) > 0 {
			cluster.PrimaryPath = cluster.InferencePaths[0]
		}
		if cluster.Count > 0 {
			cluster.AvgConfidence = round2(agg.confidenceSum / float64(cluster.Count))
		}
		if agg.hasFirstSeen {
			cluster.FirstSeen = agg.firstSeenTime.Format(time.RFC3339Nano)
		}
		if agg.hasLastSeen {
			cluster.LastSeen = agg.lastSeenTime.Format(time.RFC3339Nano)
		}
		cluster.PriorityScore = round2(float64(cluster.MaxSeverity)*0.5 + float64(cluster.Count)*2 + float64(cluster.AffectedEquipments)*8 + cluster.AvgConfidence*20)
		items = append(items, cluster)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Count == items[j].Count {
			return items[i].MaxSeverity > items[j].MaxSeverity
		}
		return items[i].Count > items[j].Count
	})
	analysis.TopClusters = limitClusters(items, 8)

	priority := append([]abnormalCluster(nil), items...)
	sort.Slice(priority, func(i, j int) bool { return priority[i].PriorityScore > priority[j].PriorityScore })
	analysis.PriorityQueue = limitClusters(priority, 5)

	if analysis.Abnormal1M >= 50 || analysis.GrowthRatio >= 3 || hasSevere(priority, 90) {
		analysis.RiskLevel = "HIGH"
	} else if analysis.Abnormal1M >= 10 || analysis.Abnormal5M >= 30 || analysis.GrowthRatio >= 1.5 || hasSevere(priority, 70) {
		analysis.RiskLevel = "MEDIUM"
	}
	return analysis
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedPathKeys(values map[string]int) []string {
	type kv struct {
		key   string
		count int
	}
	items := make([]kv, 0, len(values))
	for key, count := range values {
		items = append(items, kv{key: key, count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count == items[j].count {
			return items[i].key < items[j].key
		}
		return items[i].count > items[j].count
	})
	keys := make([]string, 0, len(items))
	for _, item := range items {
		keys = append(keys, item.key)
	}
	return keys
}

func limitClusters(items []abnormalCluster, limit int) []abnormalCluster {
	if len(items) <= limit {
		return items
	}
	return items[:limit]
}

func hasSevere(items []abnormalCluster, threshold int) bool {
	for _, item := range items {
		if item.MaxSeverity >= threshold {
			return true
		}
	}
	return false
}

func round2(value float64) float64 {
	return float64(int(value*100+0.5)) / 100
}
