package api

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"time"

	"github.com/gin-gonic/gin"
)

type metricStatus struct {
	Status      string `json:"status"`
	Reason      string `json:"reason,omitempty"`
	SampleCount int64  `json:"sample_count"`
}

type latencyMetric struct {
	Count int     `json:"count"`
	AvgMS float64 `json:"avg_ms"`
	P50MS int64   `json:"p50_ms"`
	P95MS int64   `json:"p95_ms"`
}

type competitionMetrics struct {
	SchemaVersion string `json:"schema_version"`
	Window        string `json:"window"`
	CollectedAt   string `json:"collected_at"`
	Realtime      struct {
		EndToEnd              latencyMetric            `json:"end_to_end"`
		Processing            latencyMetric            `json:"edge_processing"`
		ByPath                map[string]latencyMetric `json:"by_path"`
		CloudAverageMS        float64                  `json:"cloud_average_ms"`
		ImprovementVsCloudPct *float64                 `json:"improvement_vs_cloud_pct,omitempty"`
	} `json:"realtime"`
	Effect struct {
		GroundTruth        metricStatus `json:"ground_truth_evaluation"`
		CloudComparison    metricStatus `json:"cloud_comparison"`
		AgreementRate      float64      `json:"edge_cloud_agreement_rate"`
		ConflictRate       float64      `json:"edge_cloud_conflict_rate"`
		CorrectionRate     float64      `json:"edge_correction_rate"`
		ActionableGainRate float64      `json:"actionable_gain_rate"`
		AverageReward      float64      `json:"average_reward"`
	} `json:"effect"`
	Efficiency struct {
		RoutedTotal                 int64   `json:"routed_total"`
		LocalTotal                  int64   `json:"local_total"`
		CloudTotal                  int64   `json:"cloud_total"`
		CloudOffloadRate            float64 `json:"cloud_offload_rate"`
		UploadReductionRate         float64 `json:"estimated_upload_reduction_rate"`
		RecentInputBytes            int64   `json:"recent_input_bytes"`
		EstimatedAvoidedUploadBytes int64   `json:"estimated_avoided_upload_bytes"`
		CloudTokenTotal             int64   `json:"cloud_token_total"`
		CloudCostTotal              float64 `json:"cloud_cost_total"`
	} `json:"efficiency"`
	Scalability struct {
		DistinctSources        int64        `json:"distinct_sources"`
		DistinctEquipment      int64        `json:"distinct_equipment"`
		ObservedInferencePaths int          `json:"observed_inference_paths"`
		ScenarioEvidence       metricStatus `json:"scenario_evidence"`
	} `json:"scalability"`
	Stability struct {
		CompletionRate    float64      `json:"completion_rate"`
		FallbackRate      float64      `json:"fallback_rate"`
		DLQTotal          int64        `json:"dlq_total"`
		OutboxSuccessRate float64      `json:"outbox_success_rate"`
		OutboxRetry       int64        `json:"outbox_retry"`
		OutboxDLQ         int64        `json:"outbox_dlq"`
		NetworkExperiment metricStatus `json:"network_experiment"`
	} `json:"stability"`
	Consistency struct {
		PairedSamples         int64        `json:"paired_samples"`
		ConflictCount         int64        `json:"conflict_count"`
		ConflictRate          float64      `json:"conflict_rate"`
		ResolvedCount         int64        `json:"resolved_count"`
		ResolutionSuccessRate float64      `json:"resolution_success_rate"`
		MultiNodeExperiment   metricStatus `json:"multi_node_experiment"`
	} `json:"consistency"`
	Evidence struct {
		Mechanisms         []string `json:"mechanisms"`
		ApplicationSignals []string `json:"application_signals"`
	} `json:"evidence"`
}

func (h *Handler) collectCompetitionMetrics(ctx context.Context) (competitionMetrics, error) {
	now := time.Now().UTC()
	out := competitionMetrics{SchemaVersion: "competition-metrics-v1", Window: "recent-2000-events+cumulative-db", CollectedAt: now.Format(time.RFC3339Nano)}
	events, err := h.recentInferredEvents(ctx, 2000)
	if err != nil {
		return out, err
	}
	dbMetrics, err := h.routerStore().CompetitionMetrics(ctx)
	if err != nil {
		return out, err
	}

	processing := make([]int64, 0, len(events))
	endToEnd := make([]int64, 0, len(events))
	byPath := map[string][]int64{}
	var recentBytes int64
	fallback := 0
	for _, event := range events {
		processing = append(processing, event.LatencyMS)
		byPath[event.InferencePath] = append(byPath[event.InferencePath], event.LatencyMS)
		recentBytes += int64(len([]byte(event.RawLog)))
		if event.InferencePath == "FALLBACK" {
			fallback++
		}
		start, startErr := time.Parse(time.RFC3339Nano, event.EventTS)
		end, endErr := time.Parse(time.RFC3339Nano, event.InferredAt)
		if startErr == nil && endErr == nil && !end.Before(start) {
			endToEnd = append(endToEnd, end.Sub(start).Milliseconds())
		}
	}
	out.Realtime.Processing = latencyStats(processing)
	out.Realtime.EndToEnd = latencyStats(endToEnd)
	out.Realtime.ByPath = map[string]latencyMetric{}
	for path, values := range byPath {
		out.Realtime.ByPath[path] = latencyStats(values)
	}
	out.Realtime.CloudAverageMS = dbMetrics.CloudLatencyAvgMS
	if dbMetrics.CloudLatencyAvgMS > 0 && out.Realtime.Processing.Count > 0 {
		value := (dbMetrics.CloudLatencyAvgMS - out.Realtime.Processing.AvgMS) / dbMetrics.CloudLatencyAvgMS * 100
		out.Realtime.ImprovementVsCloudPct = &value
	}

	out.Effect.GroundTruth = metricStatus{Status: "not_available", Reason: "运行事件没有人工真值标签；Accuracy/Recall/F1 必须由标注评测集计算"}
	out.Effect.CloudComparison = metricStatus{Status: "proxy", Reason: "以云端诊断作为参照，不等同于真实准确率", SampleCount: dbMetrics.CloudSamples}
	if dbMetrics.CloudSamples > 0 {
		out.Effect.AgreementRate = ratio(dbMetrics.EdgeCloudAgree, dbMetrics.CloudSamples)
		out.Effect.ConflictRate = ratio(dbMetrics.EdgeCloudConflict, dbMetrics.CloudSamples)
		out.Effect.CorrectionRate = ratio(dbMetrics.CorrectedTotal, dbMetrics.CloudSamples)
		out.Effect.ActionableGainRate = ratio(dbMetrics.ActionableTotal, dbMetrics.CloudSamples)
	}
	out.Effect.AverageReward = dbMetrics.RewardAverage

	out.Efficiency.RoutedTotal, out.Efficiency.LocalTotal, out.Efficiency.CloudTotal = dbMetrics.RoutedTotal, dbMetrics.LocalTotal, dbMetrics.CloudTotal
	out.Efficiency.CloudOffloadRate = ratio(dbMetrics.LocalTotal, dbMetrics.RoutedTotal)
	out.Efficiency.UploadReductionRate = out.Efficiency.CloudOffloadRate
	out.Efficiency.RecentInputBytes = recentBytes
	out.Efficiency.EstimatedAvoidedUploadBytes = int64(float64(recentBytes) * out.Efficiency.UploadReductionRate)
	out.Efficiency.CloudTokenTotal, out.Efficiency.CloudCostTotal = dbMetrics.CloudTokenTotal, dbMetrics.CloudCostTotal

	out.Scalability.DistinctSources, out.Scalability.DistinctEquipment = dbMetrics.DistinctSources, dbMetrics.DistinctEquipment
	out.Scalability.ObservedInferencePaths = len(byPath)
	out.Scalability.ScenarioEvidence = metricStatus{Status: "partial", Reason: "设备/日志源多样性已采集，但两类独立应用场景需单独部署或回放实验"}

	rawTotal, _ := h.Redis.XLen(ctx, h.Cfg.RedisStream).Result()
	inferredTotal, _ := h.Redis.XLen(ctx, h.Cfg.InferredStream).Result()
	dlqTotal, _ := h.Redis.XLen(ctx, h.Cfg.DLQStream).Result()
	out.Stability.CompletionRate = ratio(inferredTotal, rawTotal)
	out.Stability.DLQTotal = dlqTotal
	if len(events) > 0 {
		out.Stability.FallbackRate = float64(fallback) / float64(len(events))
	}
	out.Stability.OutboxSuccessRate = ratio(dbMetrics.OutboxSuccess, dbMetrics.OutboxTotal)
	out.Stability.OutboxRetry, out.Stability.OutboxDLQ = dbMetrics.OutboxRetry, dbMetrics.OutboxDLQ
	out.Stability.NetworkExperiment = metricStatus{Status: "not_available", Reason: "尚未标记网络故障实验区间；普通运行数据不能证明弱网保持率"}

	out.Consistency.PairedSamples = dbMetrics.CloudSamples
	out.Consistency.ConflictCount = dbMetrics.EdgeCloudConflict
	out.Consistency.ConflictRate = ratio(dbMetrics.EdgeCloudConflict, dbMetrics.CloudSamples)
	out.Consistency.ResolvedCount = dbMetrics.ConflictResolved
	out.Consistency.ResolutionSuccessRate = ratio(dbMetrics.ConflictResolved, dbMetrics.EdgeCloudConflict)
	out.Consistency.MultiNodeExperiment = metricStatus{Status: "partial", Reason: "当前指标衡量边云决策分歧；多边缘节点同任务冲突需要实验 task_id/node_id"}
	out.Evidence.Mechanisms = []string{"规则/缓存/ONNX/边缘LLM级联", "RAISE动态云边路由", "Outbox弱网补偿", "云端反馈闭环"}
	out.Evidence.ApplicationSignals = []string{"日志异常检测", "多设备日志采集", "结构化根因诊断"}
	return out, nil
}

func (h *Handler) GetCompetitionMetrics(c *gin.Context) {
	metrics, err := h.collectCompetitionMetrics(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, metrics)
}

func (h *Handler) RunCompetitionMetricsCollector(ctx context.Context) {
	if !h.routerStore().Available() {
		return
	}
	interval := h.Cfg.CompetitionMetricsInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	collect := func() {
		collectCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		metrics, err := h.collectCompetitionMetrics(collectCtx)
		if err != nil {
			return
		}
		payload, err := json.Marshal(metrics)
		if err != nil {
			return
		}
		_ = h.routerStore().SaveCompetitionSnapshot(collectCtx, metrics.Window, string(payload), time.Now().UTC())
		_ = h.routerStore().DeleteCompetitionSnapshotsBefore(collectCtx, time.Now().UTC().Add(-7*24*time.Hour))
	}
	collect()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			collect()
		}
	}
}

func (h *Handler) ListCompetitionMetricSnapshots(c *gin.Context) {
	limit := clampQueryInt(c, "limit", 120, 1, 1000)
	items, err := h.routerStore().ListCompetitionSnapshots(c.Request.Context(), limit)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
		return
	}
	result := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		if json.Valid([]byte(item.PayloadJSON)) {
			result = append(result, json.RawMessage(item.PayloadJSON))
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": result, "total": len(result)})
}

func latencyStats(values []int64) latencyMetric {
	if len(values) == 0 {
		return latencyMetric{}
	}
	copyValues := append([]int64(nil), values...)
	sort.Slice(copyValues, func(i, j int) bool { return copyValues[i] < copyValues[j] })
	var total int64
	for _, value := range copyValues {
		total += value
	}
	return latencyMetric{Count: len(values), AvgMS: math.Round(float64(total)/float64(len(values))*100) / 100, P50MS: percentile(copyValues, 50), P95MS: percentile(copyValues, 95)}
}

func percentile(values []int64, p int) int64 {
	if len(values) == 0 {
		return 0
	}
	index := (p*len(values)+99)/100 - 1
	if index < 0 {
		index = 0
	}
	if index >= len(values) {
		index = len(values) - 1
	}
	return values[index]
}
func ratio(numerator, denominator int64) float64 {
	if denominator <= 0 {
		return 0
	}
	return math.Round(float64(numerator)/float64(denominator)*1e6) / 1e6
}
