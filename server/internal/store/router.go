package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type RouterStore struct {
	DB *gorm.DB
}

type RouterSummary struct {
	RoutedTotal                int64   `json:"routed_total"`
	LocalTotal                 int64   `json:"local_total"`
	CloudTotal                 int64   `json:"cloud_total"`
	DeferTotal                 int64   `json:"defer_total"`
	LocalRatio                 float64 `json:"local_ratio"`
	CloudRatio                 float64 `json:"cloud_ratio"`
	DeferRatio                 float64 `json:"defer_ratio"`
	OutboxPending              int64   `json:"outbox_pending"`
	OutboxRetry                int64   `json:"outbox_retry"`
	OutboxDLQ                  int64   `json:"outbox_dlq"`
	P95RouteLatencyMS          int64   `json:"p95_route_latency_ms"`
	AverageScore               float64 `json:"average_score"`
	AverageThreshold           float64 `json:"average_threshold"`
	LastAction                 string  `json:"last_action"`
	LastRoutedAt               string  `json:"last_routed_at,omitempty"`
	LatestStateSnapshotAt      string  `json:"latest_state_snapshot_at,omitempty"`
	LatestStateSnapshotVersion string  `json:"latest_state_snapshot_version,omitempty"`
}

type RouterDecisionFilter struct {
	Action     string
	TemplateID string
	Source     string
	Limit      int
	Offset     int
}

func NewRouterStore(db *gorm.DB) *RouterStore {
	return &RouterStore{DB: db}
}

func (s *RouterStore) Available() bool {
	return s != nil && s.DB != nil
}

func (s *RouterStore) Summary(ctx context.Context) (RouterSummary, error) {
	var summary RouterSummary
	if !s.Available() {
		return summary, nil
	}
	if err := s.DB.WithContext(ctx).Model(&RouterDecisionModel{}).Count(&summary.RoutedTotal).Error; err != nil {
		return summary, degradeTableErr(err)
	}
	countAction := func(action string) int64 {
		var count int64
		if err := s.DB.WithContext(ctx).Model(&RouterDecisionModel{}).Where("action = ?", action).Count(&count).Error; err != nil {
			return 0
		}
		return count
	}
	summary.LocalTotal = countAction("LOCAL")
	summary.CloudTotal = countAction("CLOUD")
	summary.DeferTotal = countAction("DEFER")
	if summary.RoutedTotal > 0 {
		summary.LocalRatio = float64(summary.LocalTotal) / float64(summary.RoutedTotal)
		summary.CloudRatio = float64(summary.CloudTotal) / float64(summary.RoutedTotal)
		summary.DeferRatio = float64(summary.DeferTotal) / float64(summary.RoutedTotal)
	}
	var avg struct {
		Score     float64
		Threshold float64
	}
	_ = s.DB.WithContext(ctx).Model(&RouterDecisionModel{}).Select("AVG(score) AS score, AVG(threshold) AS threshold").Scan(&avg).Error
	summary.AverageScore = avg.Score
	summary.AverageThreshold = avg.Threshold
	summary.P95RouteLatencyMS = s.p95RouteLatency(ctx)
	summary.OutboxPending = s.countOutbox(ctx, "PENDING")
	summary.OutboxRetry = s.countOutbox(ctx, "RETRY")
	summary.OutboxDLQ = s.countOutbox(ctx, "DLQ")

	var last RouterDecisionModel
	if err := s.DB.WithContext(ctx).Order("routed_at DESC").Order("id DESC").First(&last).Error; err == nil {
		summary.LastAction = last.Action
		summary.LastRoutedAt = last.RoutedAt.Format(time.RFC3339Nano)
	}
	var snapshot RouterStateSnapshotModel
	if err := s.DB.WithContext(ctx).Where("snapshot_type IN ?", []string{"router_state", "system_state"}).
		Order("created_at DESC").Order("id DESC").First(&snapshot).Error; err == nil {
		summary.LatestStateSnapshotAt = snapshot.CreatedAt.Format(time.RFC3339Nano)
		summary.LatestStateSnapshotVersion = snapshot.Version
	}
	return summary, nil
}

func (s *RouterStore) ListDecisions(ctx context.Context, filter RouterDecisionFilter) ([]RouterDecisionModel, int64, error) {
	if !s.Available() {
		return []RouterDecisionModel{}, 0, nil
	}
	query := s.DB.WithContext(ctx).Model(&RouterDecisionModel{})
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.TemplateID != "" {
		query = query.Where("template_id = ?", filter.TemplateID)
	}
	if filter.Source != "" {
		query = query.Where("source = ?", filter.Source)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		if degraded := degradeTableErr(err); degraded != nil {
			return nil, 0, degraded
		}
		return []RouterDecisionModel{}, 0, nil
	}
	var items []RouterDecisionModel
	err := query.Order("routed_at DESC").Order("id DESC").Limit(filter.Limit).Offset(filter.Offset).Find(&items).Error
	if err != nil {
		if degraded := degradeTableErr(err); degraded != nil {
			return nil, 0, degraded
		}
		return []RouterDecisionModel{}, 0, nil
	}
	return items, total, nil
}

func (s *RouterStore) GetDecision(ctx context.Context, eventID string) (RouterDecisionModel, bool, error) {
	if !s.Available() {
		return RouterDecisionModel{}, false, nil
	}
	var item RouterDecisionModel
	err := s.DB.WithContext(ctx).First(&item, "event_id = ?", eventID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return RouterDecisionModel{}, false, nil
	}
	if err != nil {
		return RouterDecisionModel{}, false, degradeTableErr(err)
	}
	return item, true, nil
}

func (s *RouterStore) ListOutbox(ctx context.Context, status string, limit, offset int) ([]CloudOutboxModel, int64, error) {
	if !s.Available() {
		return []CloudOutboxModel{}, 0, nil
	}
	query := s.DB.WithContext(ctx).Model(&CloudOutboxModel{})
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		if degraded := degradeTableErr(err); degraded != nil {
			return nil, 0, degraded
		}
		return []CloudOutboxModel{}, 0, nil
	}
	var items []CloudOutboxModel
	err := query.Order("priority_score DESC").Order("created_at DESC").Limit(limit).Offset(offset).Find(&items).Error
	if err != nil {
		if degraded := degradeTableErr(err); degraded != nil {
			return nil, 0, degraded
		}
		return []CloudOutboxModel{}, 0, nil
	}
	return items, total, nil
}

func (s *RouterStore) ListCloudFeedback(ctx context.Context, limit, offset int) ([]CloudFeedbackModel, int64, error) {
	if !s.Available() {
		return []CloudFeedbackModel{}, 0, nil
	}
	query := s.DB.WithContext(ctx).Model(&CloudFeedbackModel{})
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, degradeTableErr(err)
	}
	var items []CloudFeedbackModel
	if err := query.Order("created_at DESC").Order("id DESC").Limit(limit).Offset(offset).Find(&items).Error; err != nil {
		return nil, 0, degradeTableErr(err)
	}
	return items, total, nil
}

func (s *RouterStore) LatestState(ctx context.Context, snapshotType string) (RouterStateSnapshotModel, bool, error) {
	if !s.Available() {
		return RouterStateSnapshotModel{}, false, nil
	}
	query := s.DB.WithContext(ctx).Model(&RouterStateSnapshotModel{})
	if snapshotType != "" {
		query = query.Where("snapshot_type = ?", snapshotType)
	}
	var item RouterStateSnapshotModel
	err := query.Order("is_active DESC").Order("created_at DESC").Order("id DESC").First(&item).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return RouterStateSnapshotModel{}, false, nil
	}
	if err != nil {
		return RouterStateSnapshotModel{}, false, degradeTableErr(err)
	}
	return item, true, nil
}

func (s *RouterStore) countOutbox(ctx context.Context, status string) int64 {
	var count int64
	if err := s.DB.WithContext(ctx).Model(&CloudOutboxModel{}).Where("status = ?", status).Count(&count).Error; err != nil {
		return 0
	}
	return count
}

func (s *RouterStore) p95RouteLatency(ctx context.Context) int64 {
	var values []int64
	if err := s.DB.WithContext(ctx).Model(&RouterDecisionModel{}).
		Where("latency_ms IS NOT NULL").
		Order("routed_at DESC").Limit(500).Pluck("latency_ms", &values).Error; err != nil || len(values) == 0 {
		return 0
	}
	for i := 1; i < len(values); i++ {
		key := values[i]
		j := i - 1
		for j >= 0 && values[j] > key {
			values[j+1] = values[j]
			j--
		}
		values[j+1] = key
	}
	index := (95*len(values) + 99) / 100
	if index < 1 {
		index = 1
	}
	return values[index-1]
}

func degradeTableErr(err error) error {
	if err == nil {
		return nil
	}
	msg := strings.ToLower(err.Error())
	if strings.Contains(msg, "doesn't exist") || strings.Contains(msg, "does not exist") || strings.Contains(msg, "unknown table") {
		return nil
	}
	return fmt.Errorf("router store query: %w", err)
}
