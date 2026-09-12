package state

import "math"

type GainModel struct {
	Theta []float64 `json:"theta"`
	A     []float64 `json:"a"`
	B     []float64 `json:"b"`
	Alpha float64   `json:"alpha"`
}

func NewGainModel(dim int, alpha float64) GainModel {
	if dim <= 0 {
		dim = 1
	}
	model := GainModel{
		Theta: make([]float64, dim),
		A:     make([]float64, dim),
		B:     make([]float64, dim),
		Alpha: alpha,
	}
	for i := range model.A {
		model.A[i] = 1
	}
	return model
}

func (m *GainModel) Ensure(dim int) {
	if dim <= 0 {
		dim = 1
	}
	if len(m.Theta) == dim && len(m.A) == dim && len(m.B) == dim {
		for i := range m.A {
			if m.A[i] <= 0 {
				m.A[i] = 1
			}
		}
		return
	}
	next := NewGainModel(dim, m.Alpha)
	copy(next.Theta, m.Theta)
	copy(next.A, m.A)
	copy(next.B, m.B)
	*m = next
}

func (m *GainModel) Predict(features []float64) (gain float64, bonus float64) {
	m.Ensure(len(features))
	dot := 0.0
	uncertainty := 0.0
	for i, value := range features {
		dot += m.Theta[i] * value
		uncertainty += value * value / m.A[i]
	}
	base := 1 / (1 + math.Exp(-dot))
	bonus = m.Alpha * math.Sqrt(math.Max(uncertainty, 0))
	return clamp01(base + bonus), clamp01(bonus)
}

func (m *GainModel) Update(features []float64, reward float64) {
	m.Ensure(len(features))
	reward = clamp01(reward)
	for i, value := range features {
		m.A[i] += value * value
		m.B[i] += reward * value
		m.Theta[i] = m.B[i] / m.A[i]
	}
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
