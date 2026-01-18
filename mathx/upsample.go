package mathx

// CubicSpline represents the spline coefficients for a segment
type cubicSpline struct {
	A, B, C, D float64
	X          float64 // starting x value of the segment
}

// NaturalCubicSpline calculates the spline coefficients
func naturalCubicSpline(x, y []float64) []cubicSpline {
	n := len(x)
	h := make([]float64, n-1)
	alpha := make([]float64, n-1)
	for i := 0; i < n-1; i++ {
		h[i] = x[i+1] - x[i]
	}

	for i := 1; i < n-1; i++ {
		alpha[i] = (3/h[i])*(y[i+1]-y[i]) - (3/h[i-1])*(y[i]-y[i-1])
	}

	l := make([]float64, n)
	mu := make([]float64, n)
	z := make([]float64, n)

	l[0] = 1
	mu[0] = 0
	z[0] = 0

	for i := 1; i < n-1; i++ {
		l[i] = 2*(x[i+1]-x[i-1]) - h[i-1]*mu[i-1]
		mu[i] = h[i] / l[i]
		z[i] = (alpha[i] - h[i-1]*z[i-1]) / l[i]
	}

	l[n-1] = 1
	z[n-1] = 0

	c := make([]float64, n)
	b := make([]float64, n-1)
	d := make([]float64, n-1)
	a := make([]float64, n-1)

	for j := n - 2; j >= 0; j-- {
		c[j] = z[j] - mu[j]*c[j+1]
		b[j] = (y[j+1]-y[j])/h[j] - h[j]*(c[j+1]+2*c[j])/3
		d[j] = (c[j+1] - c[j]) / (3 * h[j])
		a[j] = y[j]
	}

	splines := make([]cubicSpline, n-1)
	for i := 0; i < n-1; i++ {
		splines[i] = cubicSpline{
			A: a[i],
			B: b[i],
			C: c[i],
			D: d[i],
			X: x[i],
		}
	}

	return splines
}

// EvaluateSpline evaluates the spline at given x
func evaluateSpline(splines []cubicSpline, x float64) float64 {
	// Find the correct interval
	for _, s := range splines {
		if x >= s.X {
			if x <= s.X+1.0 { // assuming uniform spacing (e.g., hours)
				dx := x - s.X
				return s.A + s.B*dx + s.C*dx*dx + s.D*dx*dx*dx
			}
		}
	}
	// If outside range, return last known value (or extrapolate if you prefer)
	return splines[len(splines)-1].A
}

// Upsample resamples data to a higher resolution (factor times)
func Upsample(data []float64, factor int) []float64 {
	n := len(data)
	x := make([]float64, n)
	for i := 0; i < n; i++ {
		x[i] = float64(i)
	}

	splines := naturalCubicSpline(x, data)

	result := make([]float64, (n-1)*factor+1)
	for i := 0; i < (n-1)*factor+1; i++ {
		t := float64(i) / float64(factor)
		result[i] = evaluateSpline(splines, t)
	}

	return result
}
