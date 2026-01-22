package main

import (
	"fmt"
	"golib/mathx"
)

// Example usage
func main() {
	hourlyTemps := []float64{20.0, 22.0, 21.0, 23.5, 24.0}
	minutelyTemps := mathx.Upsample(hourlyTemps, 60)

	for i := 0; i < len(minutelyTemps); i += 1 {
		fmt.Printf("Minute %3d: %.2f°C\n", i, minutelyTemps[i])
	}
}
