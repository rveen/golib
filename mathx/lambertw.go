package mathx

import "math"

// LambertW computes the principal branch of the Lambert W function using Halley's method.
// The function solves for w in the equation w * exp(w) = z.
// Source: chatgpt
func LambertW(z float64) float64 {
	// Special case for z = 0
	if z == 0 {
		return 0
	}

	// Initial guess: a reasonable start for positive z is log(z) (or z itself for small values)
	var w float64
	if z < math.Exp(-1) {
		w = z
	} else {
		w = math.Log(z)
	}

	// Iterate using Halley's method
	const tolerance = 1e-10
	const maxIter = 100

	for i := 0; i < maxIter; i++ {
		ew := math.Exp(w)
		wew := w * ew
		diff := wew - z
		denom := ew*(w+1) - (w+2)*diff/(2*(w+1))
		wNew := w - diff/denom

		if math.Abs(wNew-w) < tolerance {
			return wNew
		}
		w = wNew
	}

	return w
}
