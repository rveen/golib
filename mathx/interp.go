package mathx

// Line interpolation
// x and y increase monotonically
//

import (
	"math"
)

func Interpolate(xval float64, x []float64, y []float64) float64 {

	// If xval is outside the range of the x table, do not interpolate
	if xval > x[len(x)-1] || xval < x[0] {
		return math.NaN()
	}

	// Find the interval where xval belongs to
	var i int
	found := false
	for i = len(x) - 1; i >= 0; i-- {
		if xval >= x[i] {
			found = true
			break
		}
	}

	if !found {
		return math.NaN()
	}

	return (y[i+1]-y[i])/(x[i+1]-x[i])*(xval-x[i]) + y[i]
}
