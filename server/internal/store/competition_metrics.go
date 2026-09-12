package store

import (
	"context"
	"time"
)

type CompetitionMetricSnapshotModel struct {
	ID          int64     `gorm:"primaryKey;autoIncrement" json:"id"`
	Window      string    `gorm:"size:32;index;not null" json:"window"`
	PayloadJSON string    `gorm:"type:longtext;not null" json:"-"`
	CollectedAt time.Time `gorm:"index;not null" json:"collected_at"`
}

func (CompetitionMetricSnapshotModel) TableName() string { return "competition_metric_snapshots" }

type CompetitionDBMetrics struct {
	RoutedTotal       int64
	LocalTotal        int64
	CloudTotal        int64
	DeferTotal        int64
	DistinctSources   int64
	DistinctEquipment int64
	CloudSamples      int64
	CloudLatencyAvgMS float64
	CloudTokenTotal   int64
	CloudCostTotal    float64
	EdgeCloudAgree    int64
	EdgeCloudConflict int64
	ConflictResolved  int64
	ActionableTotal   int64
	CorrectedTotal    int64
	RewardAverage     float64
	OutboxTotal       int64
	OutboxSuccess     int64
	OutboxRetry       int64
	OutboxDLQ         int64
}

func (s *RouterStore) CompetitionMetrics(ctx context.Context) (CompetitionDBMetrics, error) {
	var out CompetitionDBMetrics
	if !s.Available() {
		return out, nil
	}
	var routing struct {
		RoutedTotal       int64
		LocalTotal        int64
		CloudTotal        int64
		DeferTotal        int64
		DistinctSources   int64
		DistinctEquipment int64
	}
	if err := s.DB.WithContext(ctx).Raw(`
		SELECT COUNT(*) routed_total,
		SUM(action = 'LOCAL') local_total,
		SUM(action = 'CLOUD') cloud_total,
		SUM(action = 'DEFER') defer_total,
		COUNT(DISTINCT NULLIF(source, '')) distinct_sources,
		COUNT(DISTINCT NULLIF(equipment, '')) distinct_equipment
		FROM router_decisions`).Scan(&routing).Error; err != nil {
		return out, degradeTableErr(err)
	}
	out.RoutedTotal, out.LocalTotal, out.CloudTotal, out.DeferTotal = routing.RoutedTotal, routing.LocalTotal, routing.CloudTotal, routing.DeferTotal
	out.DistinctSources, out.DistinctEquipment = routing.DistinctSources, routing.DistinctEquipment
	var feedback struct {
		CloudSamples      int64
		CloudLatencyAvgMS float64
		CloudTokenTotal   int64
		CloudCostTotal    float64
		EdgeCloudAgree    int64
		EdgeCloudConflict int64
		ConflictResolved  int64
		ActionableTotal   int64
		CorrectedTotal    int64
		RewardAverage     float64
	}
	if err := s.DB.WithContext(ctx).Raw(`
		SELECT COUNT(*) cloud_samples,
		COALESCE(AVG(cloud_latency_ms), 0) cloud_latency_avg_ms,
		COALESCE(SUM(token_usage), 0) cloud_token_total,
		COALESCE(SUM(cost), 0) cloud_cost_total,
		SUM(UPPER(edge_pred) = UPPER(cloud_pred)) edge_cloud_agree,
		SUM(UPPER(edge_pred) <> UPPER(cloud_pred)) edge_cloud_conflict,
		SUM(UPPER(edge_pred) <> UPPER(cloud_pred) AND processed_by_router = 1) conflict_resolved,
		SUM(added_actionable_info = 1) actionable_total,
		SUM(corrected_edge = 1) corrected_total,
		COALESCE(AVG(reward), 0) reward_average
		FROM cloud_feedback`).Scan(&feedback).Error; err != nil {
		return out, degradeTableErr(err)
	}
	out.CloudSamples, out.CloudLatencyAvgMS = feedback.CloudSamples, feedback.CloudLatencyAvgMS
	out.CloudTokenTotal, out.CloudCostTotal = feedback.CloudTokenTotal, feedback.CloudCostTotal
	out.EdgeCloudAgree, out.EdgeCloudConflict, out.ConflictResolved = feedback.EdgeCloudAgree, feedback.EdgeCloudConflict, feedback.ConflictResolved
	out.ActionableTotal, out.CorrectedTotal, out.RewardAverage = feedback.ActionableTotal, feedback.CorrectedTotal, feedback.RewardAverage
	var outbox struct{ OutboxTotal, OutboxSuccess, OutboxRetry, OutboxDLQ int64 }
	if err := s.DB.WithContext(ctx).Raw(`
		SELECT COUNT(*) outbox_total,
		SUM(status IN ('SENT','DONE')) outbox_success,
		SUM(status = 'RETRY') outbox_retry,
		SUM(status = 'DLQ') outbox_dlq
		FROM cloud_outbox`).Scan(&outbox).Error; err != nil {
		return out, degradeTableErr(err)
	}
	out.OutboxTotal, out.OutboxSuccess, out.OutboxRetry, out.OutboxDLQ = outbox.OutboxTotal, outbox.OutboxSuccess, outbox.OutboxRetry, outbox.OutboxDLQ
	return out, nil
}

func (s *RouterStore) SaveCompetitionSnapshot(ctx context.Context, window, payload string, collectedAt time.Time) error {
	if !s.Available() {
		return nil
	}
	return s.DB.WithContext(ctx).Create(&CompetitionMetricSnapshotModel{
		Window: window, PayloadJSON: payload, CollectedAt: collectedAt,
	}).Error
}

func (s *RouterStore) ListCompetitionSnapshots(ctx context.Context, limit int) ([]CompetitionMetricSnapshotModel, error) {
	if !s.Available() {
		return []CompetitionMetricSnapshotModel{}, nil
	}
	var items []CompetitionMetricSnapshotModel
	err := s.DB.WithContext(ctx).Order("collected_at DESC").Limit(limit).Find(&items).Error
	return items, err
}

func (s *RouterStore) DeleteCompetitionSnapshotsBefore(ctx context.Context, cutoff time.Time) error {
	if !s.Available() {
		return nil
	}
	return s.DB.WithContext(ctx).Where("collected_at < ?", cutoff).Delete(&CompetitionMetricSnapshotModel{}).Error
}
