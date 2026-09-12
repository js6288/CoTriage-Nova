package feedback

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/config"
	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/event"
	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/raise"
	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/state"
	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/store"
)

const (
	SnapshotVersion = "raise-feedback-v1"
)

type Processor struct {
	Store    *store.Store
	Manager  *state.Manager
	Interval time.Duration
	Batch    int
	Now      func() time.Time
	Logger   *log.Logger
}

func New(cfg config.Config, st *store.Store, manager *state.Manager) *Processor {
	interval := cfg.FeedbackProcessorInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	batch := cfg.FeedbackProcessorBatch
	if batch <= 0 {
		batch = 20
	}
	if manager == nil {
		manager = state.NewManager(cfg.StateWindow, 7, cfg.GainUCBAlpha)
	}
	return &Processor{
		Store:    st,
		Manager:  manager,
		Interval: interval,
		Batch:    batch,
		Now:      func() time.Time { return time.Now().UTC() },
	}
}

func (p *Processor) Run(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	ticker := time.NewTicker(p.Interval)
	defer ticker.Stop()
	p.logf("feedback processor started interval=%s batch=%d", p.Interval, p.Batch)
	for {
		if err := p.ProcessOnce(ctx); err != nil {
			p.logf("feedback processor tick failed: %v", err)
		}
		select {
		case <-ctx.Done():
			p.logf("feedback processor stopped")
			return
		case <-ticker.C:
		}
	}
}

func (p *Processor) ProcessOnce(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if p.Store == nil {
		return errors.New("store unavailable")
	}
	if p.Manager == nil {
		return errors.New("state manager unavailable")
	}
	items, err := p.Store.ListUnprocessedFeedback(ctx, p.Batch)
	if err != nil {
		return err
	}
	var firstErr error
	for _, item := range items {
		now := p.now()
		claimed, err := p.Store.TryClaimFeedback(ctx, item.ID, now)
		if err != nil {
			firstErr = keepFirst(firstErr, err)
			continue
		}
		if !claimed {
			continue
		}
		result := ClassifyReward(item)
		in := inferredFromFeedback(item)
		features := featuresFromFeedback(item, result)
		p.Manager.ApplyFeedback(in, result.Corrected, result.Reward, features)
		if err := p.Store.FinalizeFeedbackReward(ctx, item.ID, result.Reward, result.Reason, p.now()); err != nil {
			firstErr = keepFirst(firstErr, err)
			continue
		}
		if err := p.SaveSnapshot(ctx); err != nil {
			firstErr = keepFirst(firstErr, err)
		}
	}
	return firstErr
}

func (p *Processor) SaveSnapshot(ctx context.Context) error {
	payload, err := p.Manager.SnapshotJSON()
	if err != nil {
		return err
	}
	meta, err := json.Marshal(map[string]any{
		"component": "feedback_processor",
		"version":   SnapshotVersion,
	})
	if err != nil {
		return err
	}
	_, err = p.Store.SaveStateSnapshot(ctx, store.RouterStateSnapshotModel{
		SnapshotType: state.DefaultSnapshotType,
		SnapshotKey:  state.DefaultSnapshotKey,
		PayloadJSON:  payload,
		Version:      SnapshotVersion,
		IsActive:     true,
		MetadataJSON: string(meta),
		CreatedAt:    p.now(),
	})
	if err != nil {
		return fmt.Errorf("save feedback state snapshot: %w", err)
	}
	return nil
}

func inferredFromFeedback(item store.CloudFeedbackModel) event.InferredEvent {
	severity := int(item.RouteScore * 100)
	if severity <= 0 {
		severity = int(item.RouteThreshold * 100)
	}
	if severity <= 0 {
		severity = 50
	}
	if severity > 100 {
		severity = 100
	}
	return event.InferredEvent{
		EventID:    item.EventID,
		Source:     item.Source,
		Equipment:  item.Equipment,
		TemplateID: item.TemplateID,
		EdgePred:   item.EdgePred,
		Severity:   severity,
		Confidence: item.EdgeConfidence,
	}
}

func featuresFromFeedback(item store.CloudFeedbackModel, result RewardResult) raise.Features {
	uncertainty := 1 - item.EdgeConfidence
	if uncertainty < 0 {
		uncertainty = 0
	}
	if uncertainty > 1 {
		uncertainty = 1
	}
	disagreement := 0.0
	if result.Corrected {
		disagreement = 1
	}
	fleetImpact := item.RouteScore
	if fleetImpact < 0 {
		fleetImpact = 0
	}
	if fleetImpact > 1 {
		fleetImpact = 1
	}
	pathRisk := item.RouteThreshold
	if pathRisk < 0 {
		pathRisk = 0
	}
	if pathRisk > 1 {
		pathRisk = 1
	}
	return raise.Features{
		CalibratedUncertainty:  uncertainty,
		Novelty:                0,
		HistoricalDisagreement: disagreement,
		FleetImpact:            fleetImpact,
		PathRisk:               pathRisk,
	}
}

func (p *Processor) now() time.Time {
	if p.Now == nil {
		return time.Now().UTC()
	}
	return p.Now().UTC()
}

func (p *Processor) logf(format string, args ...any) {
	if p.Logger != nil {
		p.Logger.Printf(format, args...)
		return
	}
	log.Printf("[router-controller] "+format, args...)
}

func keepFirst(first, next error) error {
	if first != nil {
		return first
	}
	return next
}
