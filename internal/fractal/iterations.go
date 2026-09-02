package fractal

import "math"

const (
	androidProgressiveBase       = 40
	androidProgressiveMultiplier = 1.2
	minimumIterations            = 10
	maximumIterations            = 1_048_576
)

// AndroidProgressiveIterations reproduces Android's Scale-with-zoom defaults
// for a CLI zoom and the width of the sampled viewport.
func AndroidProgressiveIterations(zoom float64, viewportWidth int) int {
	if zoom <= 0 || viewportWidth <= 0 || math.IsNaN(zoom) || math.IsInf(zoom, 0) {
		return androidProgressiveBase
	}
	referenceZoom := 105000 / (float64(viewportWidth) * float64(viewportWidth))
	value := androidProgressiveBase * math.Pow(androidProgressiveMultiplier, math.Log2(zoom/referenceZoom))
	if !isFinite(value) || value >= maximumIterations {
		return maximumIterations
	}
	value = max(float64(minimumIterations), min(float64(maximumIterations), value))
	return int(math.Floor(value + .5))
}

func isFinite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
