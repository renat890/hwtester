package tests

func isPassValue(expected, actual int, threshold float64) bool {
	minMb := float64(expected) * (1 - threshold)
	maxMb := float64(expected) * (1 + threshold)
	return float64(actual) >= minMb && float64(actual) <= maxMb
}
