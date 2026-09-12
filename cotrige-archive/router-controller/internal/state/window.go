package state

import (
	"strings"
	"time"
)

type Observation struct {
	TemplateID string    `json:"template_id"`
	Source     string    `json:"source"`
	EdgePred   string    `json:"edge_pred"`
	SeenAt     time.Time `json:"seen_at"`
}

type Window struct {
	Duration     time.Duration `json:"duration"`
	Observations []Observation `json:"observations"`
}

func NewWindow(duration time.Duration) Window {
	if duration <= 0 {
		duration = 10 * time.Minute
	}
	return Window{Duration: duration}
}

func (w *Window) Add(obs Observation, now time.Time) {
	if obs.SeenAt.IsZero() {
		obs.SeenAt = now
	}
	obs.TemplateID = norm(obs.TemplateID)
	obs.Source = strings.TrimSpace(obs.Source)
	obs.EdgePred = strings.ToUpper(strings.TrimSpace(obs.EdgePred))
	w.Observations = append(w.Observations, obs)
	w.Prune(now)
}

func (w *Window) Prune(now time.Time) {
	cutoff := now.Add(-w.Duration)
	keep := w.Observations[:0]
	for _, obs := range w.Observations {
		if !obs.SeenAt.Before(cutoff) {
			keep = append(keep, obs)
		}
	}
	w.Observations = keep
}

func (w *Window) TemplateFrequency(templateID string, now time.Time) int {
	w.Prune(now)
	templateID = norm(templateID)
	count := 0
	for _, obs := range w.Observations {
		if obs.TemplateID == templateID {
			count++
		}
	}
	return count
}

func (w *Window) AffectedDevices(templateID string, now time.Time) int {
	w.Prune(now)
	templateID = norm(templateID)
	devices := make(map[string]bool)
	for _, obs := range w.Observations {
		if obs.TemplateID == templateID && obs.Source != "" {
			devices[obs.Source] = true
		}
	}
	return len(devices)
}

func (w *Window) RecentCloudRatio(cloudActions, totalActions int) float64 {
	if totalActions <= 0 {
		return 0
	}
	return float64(cloudActions) / float64(totalActions)
}

func norm(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
