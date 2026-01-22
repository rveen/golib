package mathx

import (
	"math"
)

// Evaluate a polynomial at point x
func PolyVal(x float64, coeff []float64) float64 {
	r := 0.0

	for i, c := range coeff {
		r += c * math.Pow(x, float64(i))
	}

	return r
}
