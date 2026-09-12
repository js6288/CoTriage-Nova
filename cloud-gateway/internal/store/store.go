package store

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type Feedback struct {
	ID                  int64  `gorm:"primaryKey"`
	EventID             string `gorm:"size:128;uniqueIndex"`
	RequestID           string
	IdempotencyKey      string
	Source              string
	Equipment           string
	TemplateID          string
	EdgePred            string
	EdgeConfidence      float64
	RouteAction         string
	RouteScore          float64
	RouteThreshold      float64
	CloudModel          string
	CloudPred           string
	RootCause           string `gorm:"type:text"`
	Recommendation      string `gorm:"type:text"`
	Explanation         string `gorm:"type:text"`
	CloudConfidence     float64
	CloudLatencyMS      int64
	TokenUsage          int
	Cost                float64
	Reward              float64
	RewardReason        string
	CorrectedEdge       bool
	AddedActionableInfo bool
	ConfirmedHighRisk   bool
	FeedbackJSON        string `gorm:"type:longtext"`
	ProcessedByRouter   bool
	ProcessedAt         *time.Time
	CreatedAt           time.Time
}

func (Feedback) TableName() string { return "cloud_feedback" }

type Store struct{ DB *gorm.DB }

func (s *Store) Exists(ctx context.Context, eventID string) (bool, error) {
	var n int64
	e := s.DB.WithContext(ctx).Model(&Feedback{}).Where("event_id=?", eventID).Count(&n).Error
	return n > 0, e
}
func (s *Store) Get(ctx context.Context, eventID string) (Feedback, error) {
	var f Feedback
	err := s.DB.WithContext(ctx).Where("event_id = ?", eventID).First(&f).Error
	return f, err
}
func (s *Store) Insert(ctx context.Context, f Feedback) error {
	e := s.DB.WithContext(ctx).Create(&f).Error
	if e != nil && errors.Is(e, gorm.ErrDuplicatedKey) {
		return nil
	}
	return e
}
