package raise

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func clampProbability(value float64) float64 {
	if value <= 0 {
		return 0.000001
	}
	if value >= 1 {
		return 0.999999
	}
	return value
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
