package state

import (
	"encoding/json"
	"time"

	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/event"
	"github.com/pangjian-pj/cotriage-dev/router-controller/internal/raise"
)

const (
	DefaultSnapshotType = "router_state"
	DefaultSnapshotKey  = "default"
)

type Manager struct {
	Window             Window        `json:"window"`
	TemplateStats      TemplateStats `json:"template_stats"`
	GainModel          GainModel     `json:"gain_model"`
	ConformalSetSize   int           `json:"conformal_set_size"`
	RecentCloudActions int           `json:"recent_cloud_actions"`
	RecentTotalActions int           `json:"recent_total_actions"`
}

func NewManager(window time.Duration, gainDim int, ucbAlpha float64) *Manager {
	return &Manager{
		Window:           NewWindow(window),
		TemplateStats:    NewTemplateStats(),
		GainModel:        NewGainModel(gainDim, ucbAlpha),
		ConformalSetSize: 1,
	}
}

func (m *Manager) Observe(in event.InferredEvent, action string, now time.Time) {
	m.Window.Add(Observation{
		TemplateID: in.TemplateID,
		Source:     in.Source,
		EdgePred:   in.EdgePred,
		SeenAt:     now,
	}, now)
	m.RecentTotalActions++
	if action == raise.ActionCloud {
		m.RecentCloudActions++
	}
}

func (m *Manager) BuildStats(in event.InferredEvent, now time.Time) raise.Stats {
	stat := m.TemplateStats.Get(in.TemplateID)
	_, bonus := m.GainModel.Predict(GainFeatures(in, raise.Features{}))
	setSize := m.ConformalSetSize
	if setSize <= 0 {
		setSize = 1
	}
	return raise.Stats{
		TemplateFrequency:         m.Window.TemplateFrequency(in.TemplateID, now),
		HistoricalCloudCount:      stat.CloudCount,
		HistoricalCorrectionCount: stat.CorrectionCount,
		AffectedDevices:           m.Window.AffectedDevices(in.TemplateID, now),
		ConformalSetSize:          setSize,
		UCBUncertainty:            bonus,
	}
}

func (m *Manager) RecentCloudRatio() float64 {
	if m.RecentTotalActions <= 0 {
		return 0
	}
	return float64(m.RecentCloudActions) / float64(m.RecentTotalActions)
}

func (m *Manager) ApplyFeedback(in event.InferredEvent, corrected bool, reward float64, features raise.Features) {
	m.TemplateStats.RecordFeedback(in.TemplateID, corrected)
	m.GainModel.Update(GainFeatures(in, features), reward)
}

func (m *Manager) SnapshotJSON() (string, error) {
	body, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func RestoreManager(payload string) (*Manager, error) {
	var manager Manager
	if err := json.Unmarshal([]byte(payload), &manager); err != nil {
		return nil, err
	}
	manager.TemplateStats.Ensure()
	if manager.Window.Duration <= 0 {
		manager.Window.Duration = 10 * time.Minute
	}
	manager.GainModel.Ensure(7)
	if manager.ConformalSetSize <= 0 {
		manager.ConformalSetSize = 1
	}
	return &manager, nil
}

func GainFeatures(in event.InferredEvent, features raise.Features) []float64 {
	return []float64{
		float64(in.Severity) / 100,
		features.CalibratedUncertainty,
		features.Novelty,
		features.HistoricalDisagreement,
		features.FleetImpact,
		features.PathRisk,
		1,
	}
}
