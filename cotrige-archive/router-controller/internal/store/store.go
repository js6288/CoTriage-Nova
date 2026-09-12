package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrNotFound = errors.New("not found")

type Store struct {
	DB *gorm.DB
}

func New(db *gorm.DB) *Store {
	return &Store{DB: db}
}

func (s *Store) CreateDecisionIfAbsent(ctx context.Context, decision RouterDecisionModel) (RouterDecisionModel, bool, error) {
	now := time.Now().UTC()
	if decision.CreatedAt.IsZero() {
		decision.CreatedAt = now
	}
	if decision.UpdatedAt.IsZero() {
		decision.UpdatedAt = now
	}
	if decision.RoutedAt.IsZero() {
		decision.RoutedAt = now
	}
	res := s.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&decision)
	if res.Error != nil {
		return RouterDecisionModel{}, false, fmt.Errorf("create router decision: %w", res.Error)
	}
	if res.RowsAffected > 0 {
		return decision, true, nil
	}
	existing, err := s.GetDecisionByEventID(ctx, decision.EventID)
	return existing, false, err
}

func (s *Store) GetDecisionByEventID(ctx context.Context, eventID string) (RouterDecisionModel, error) {
	var decision RouterDecisionModel
	err := s.DB.WithContext(ctx).First(&decision, "event_id = ?", eventID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return RouterDecisionModel{}, ErrNotFound
	}
	if err != nil {
		return RouterDecisionModel{}, fmt.Errorf("get router decision: %w", err)
	}
	return decision, nil
}

func (s *Store) CreateOutboxIfAbsent(ctx context.Context, item CloudOutboxModel) (CloudOutboxModel, bool, error) {
	now := time.Now().UTC()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = now
	}
	if item.Status == "" {
		item.Status = OutboxStatusPending
	}
	if item.MaxAttempts == 0 {
		item.MaxAttempts = 5
	}
	res := s.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&item)
	if res.Error != nil {
		return CloudOutboxModel{}, false, fmt.Errorf("create cloud outbox: %w", res.Error)
	}
	if res.RowsAffected > 0 {
		return item, true, nil
	}
	existing, err := s.GetOutboxByEventID(ctx, item.EventID)
	return existing, false, err
}

func (s *Store) GetOutboxByEventID(ctx context.Context, eventID string) (CloudOutboxModel, error) {
	var item CloudOutboxModel
	err := s.DB.WithContext(ctx).First(&item, "event_id = ?", eventID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CloudOutboxModel{}, ErrNotFound
	}
	if err != nil {
		return CloudOutboxModel{}, fmt.Errorf("get cloud outbox: %w", err)
	}
	return item, nil
}

func (s *Store) CreateFeedbackIfAbsent(ctx context.Context, feedback CloudFeedbackModel) (CloudFeedbackModel, bool, error) {
	if feedback.CreatedAt.IsZero() {
		feedback.CreatedAt = time.Now().UTC()
	}
	res := s.DB.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&feedback)
	if res.Error != nil {
		return CloudFeedbackModel{}, false, fmt.Errorf("create cloud feedback: %w", res.Error)
	}
	if res.RowsAffected > 0 {
		return feedback, true, nil
	}
	var existing CloudFeedbackModel
	err := s.DB.WithContext(ctx).First(&existing, "event_id = ?", feedback.EventID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CloudFeedbackModel{}, false, ErrNotFound
	}
	if err != nil {
		return CloudFeedbackModel{}, false, fmt.Errorf("get cloud feedback: %w", err)
	}
	return existing, false, nil
}

func (s *Store) ListUnprocessedFeedback(ctx context.Context, limit int) ([]CloudFeedbackModel, error) {
	if limit <= 0 {
		limit = 20
	}
	var items []CloudFeedbackModel
	err := s.DB.WithContext(ctx).
		Where("processed_by_router = ? OR processed_by_router IS NULL", false).
		Order("created_at ASC").
		Order("id ASC").
		Limit(limit).
		Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("list unprocessed feedback: %w", err)
	}
	return items, nil
}

func (s *Store) TryClaimFeedback(ctx context.Context, id int64, now time.Time) (bool, error) {
	res := s.DB.WithContext(ctx).Model(&CloudFeedbackModel{}).
		Where("id = ?", id).
		Where("processed_by_router = ? OR processed_by_router IS NULL", false).
		Updates(map[string]any{
			"processed_by_router": true,
			"processed_at":        now,
		})
	if res.Error != nil {
		return false, fmt.Errorf("claim feedback: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

func (s *Store) FinalizeFeedbackReward(ctx context.Context, id int64, reward float64, reason string, now time.Time) error {
	res := s.DB.WithContext(ctx).Model(&CloudFeedbackModel{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"reward":              reward,
			"reward_reason":       reason,
			"processed_by_router": true,
			"processed_at":        now,
		})
	if res.Error != nil {
		return fmt.Errorf("finalize feedback reward: %w", res.Error)
	}
	return nil
}

func (s *Store) SaveStateSnapshot(ctx context.Context, snapshot RouterStateSnapshotModel) (RouterStateSnapshotModel, error) {
	if snapshot.CreatedAt.IsZero() {
		snapshot.CreatedAt = time.Now().UTC()
	}
	if err := s.DB.WithContext(ctx).Create(&snapshot).Error; err != nil {
		return RouterStateSnapshotModel{}, fmt.Errorf("create router state snapshot: %w", err)
	}
	return snapshot, nil
}

func (s *Store) CountPendingOutbox(ctx context.Context) (int64, error) {
	var count int64
	err := s.DB.WithContext(ctx).Model(&CloudOutboxModel{}).
		Where("status IN ?", []string{OutboxStatusPending, OutboxStatusRetry, OutboxStatusSending}).
		Count(&count).Error
	if err != nil {
		return 0, fmt.Errorf("count pending outbox: %w", err)
	}
	return count, nil
}

func (s *Store) ListDueOutbox(ctx context.Context, limit int, now time.Time) ([]CloudOutboxModel, error) {
	if limit <= 0 {
		limit = 20
	}
	var items []CloudOutboxModel
	err := s.DB.WithContext(ctx).
		Where("status IN ?", []string{OutboxStatusPending, OutboxStatusRetry}).
		Where("next_retry_at IS NULL OR next_retry_at <= ?", now).
		Where("locked_until IS NULL OR locked_until <= ?", now).
		Order("priority_score DESC").
		Order("deadline_at IS NULL").
		Order("deadline_at ASC").
		Order("created_at ASC").
		Limit(limit).
		Find(&items).Error
	if err != nil {
		return nil, fmt.Errorf("list due outbox: %w", err)
	}
	return items, nil
}

func (s *Store) TryLockOutbox(ctx context.Context, id int64, worker string, now, lockUntil time.Time) (CloudOutboxModel, bool, error) {
	res := s.DB.WithContext(ctx).Model(&CloudOutboxModel{}).
		Where("id = ?", id).
		Where("status IN ?", []string{OutboxStatusPending, OutboxStatusRetry}).
		Where("next_retry_at IS NULL OR next_retry_at <= ?", now).
		Where("locked_until IS NULL OR locked_until <= ?", now).
		Updates(map[string]any{
			"status":       OutboxStatusSending,
			"attempt":      gorm.Expr("COALESCE(attempt, 0) + 1"),
			"locked_by":    worker,
			"locked_until": lockUntil,
			"updated_at":   now,
		})
	if res.Error != nil {
		return CloudOutboxModel{}, false, fmt.Errorf("lock outbox: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return CloudOutboxModel{}, false, nil
	}
	var item CloudOutboxModel
	if err := s.DB.WithContext(ctx).First(&item, "id = ?", id).Error; err != nil {
		return CloudOutboxModel{}, false, fmt.Errorf("get locked outbox: %w", err)
	}
	return item, true, nil
}

func (s *Store) RequeueExpiredSending(ctx context.Context, now time.Time, reason string) (int64, error) {
	res := s.DB.WithContext(ctx).Model(&CloudOutboxModel{}).
		Where("status = ?", OutboxStatusSending).
		Where("locked_until IS NOT NULL AND locked_until <= ?", now).
		Updates(map[string]any{
			"status":        OutboxStatusRetry,
			"next_retry_at": now,
			"locked_by":     "",
			"locked_until":  nil,
			"last_error":    reason,
			"last_error_at": now,
			"updated_at":    now,
		})
	if res.Error != nil {
		return 0, fmt.Errorf("requeue expired sending outbox: %w", res.Error)
	}
	return res.RowsAffected, nil
}

func (s *Store) MarkOutboxSent(ctx context.Context, id int64, now time.Time) error {
	res := s.DB.WithContext(ctx).Model(&CloudOutboxModel{}).
		Where("id = ? AND status = ?", id, OutboxStatusSending).
		Updates(map[string]any{
			"status":       OutboxStatusSent,
			"sent_at":      now,
			"done_at":      now,
			"locked_by":    "",
			"locked_until": nil,
			"updated_at":   now,
		})
	if res.Error != nil {
		return fmt.Errorf("mark outbox sent: %w", res.Error)
	}
	return nil
}

func (s *Store) MarkOutboxRetry(ctx context.Context, id int64, nextRetry time.Time, reason string, now time.Time) error {
	res := s.DB.WithContext(ctx).Model(&CloudOutboxModel{}).
		Where("id = ? AND status = ?", id, OutboxStatusSending).
		Updates(map[string]any{
			"status":        OutboxStatusRetry,
			"next_retry_at": nextRetry,
			"locked_by":     "",
			"locked_until":  nil,
			"last_error":    reason,
			"last_error_at": now,
			"updated_at":    now,
		})
	if res.Error != nil {
		return fmt.Errorf("mark outbox retry: %w", res.Error)
	}
	return nil
}

func (s *Store) MarkOutboxDLQ(ctx context.Context, id int64, reason string, now time.Time) error {
	res := s.DB.WithContext(ctx).Model(&CloudOutboxModel{}).
		Where("id = ? AND status = ?", id, OutboxStatusSending).
		Updates(map[string]any{
			"status":        OutboxStatusDLQ,
			"done_at":       now,
			"locked_by":     "",
			"locked_until":  nil,
			"last_error":    reason,
			"last_error_at": now,
			"dlq_reason":    reason,
			"updated_at":    now,
		})
	if res.Error != nil {
		return fmt.Errorf("mark outbox dlq: %w", res.Error)
	}
	return nil
}

func (s *Store) LatestStateSnapshot(ctx context.Context, snapshotType, snapshotKey string) (RouterStateSnapshotModel, error) {
	var snapshot RouterStateSnapshotModel
	err := s.DB.WithContext(ctx).
		Where("snapshot_type = ? AND snapshot_key = ?", snapshotType, snapshotKey).
		Order("is_active DESC").
		Order("created_at DESC").
		Order("id DESC").
		First(&snapshot).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return RouterStateSnapshotModel{}, ErrNotFound
	}
	if err != nil {
		return RouterStateSnapshotModel{}, fmt.Errorf("get latest router state snapshot: %w", err)
	}
	return snapshot, nil
}
