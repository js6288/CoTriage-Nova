package state

import "strings"

type TemplateStat struct {
	CloudCount      int `json:"cloud_count"`
	CorrectionCount int `json:"correction_count"`
}

type TemplateStats struct {
	Items map[string]TemplateStat `json:"items"`
}

func NewTemplateStats() TemplateStats {
	return TemplateStats{Items: make(map[string]TemplateStat)}
}

func (s *TemplateStats) Ensure() {
	if s.Items == nil {
		s.Items = make(map[string]TemplateStat)
	}
}

func (s *TemplateStats) RecordFeedback(templateID string, corrected bool) {
	s.Ensure()
	key := norm(templateID)
	item := s.Items[key]
	item.CloudCount++
	if corrected {
		item.CorrectionCount++
	}
	s.Items[key] = item
}

func (s *TemplateStats) Get(templateID string) TemplateStat {
	s.Ensure()
	return s.Items[norm(templateID)]
}

func (s *TemplateStats) Merge(other TemplateStats) {
	s.Ensure()
	for key, value := range other.Items {
		s.Items[strings.ToLower(strings.TrimSpace(key))] = value
	}
}
