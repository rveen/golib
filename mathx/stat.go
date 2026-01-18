package mathx

import "math"

func NormalCdf(x float64) float64 {
	return math.Abs(1+math.Erf(x/math.Sqrt2)) / 2
}

// Inverted CDF, by successive approximation
func NormalInvCdf(x float64) float64 {

	e := 0.00000000001
	is := 1.0
	s := 1.0
	last := true

	i := 1000
	for {
		y := NormalCdf(s)

		y = 2 * (1 - y)

		if math.Abs(y-x) < e {
			return s
		}

		current := y > x

		if last != current {
			is /= 2
		}

		if !current {
			s -= is
		} else {
			s += is
		}

		i--
		if i == 0 {
			break
		}
	}
	return math.NaN()
}
