package graphs

import (
	"fmt"
	"log"
	"math"

	"github.com/montanaflynn/stats"
)

const (
	t1   string = "<text x='%f' y='%d' text-anchor='left' alignment-baseline='bottom' style='font-size:10; font-family: arial'>%s</text>\n"
	t2   string = "<text x='%f' y='%d' text-anchor='left' alignment-baseline='bottom' style='font-size:10; font-family: arial; font-weight: bold'>%f</text>\n"
	t2pc string = "<text x='%f' y='%d' text-anchor='left' alignment-baseline='bottom' style='font-size:10; font-family: arial; font-weight: bold'>%f ppm</text>\n"
)

func Histogram(v []float64, lsl, usl float64, n, width, height int) string {

	// n is the number of bars
	if n == 0 {
		n = 50.0
	}

	checkLow := !math.IsNaN(lsl) && !math.IsInf(lsl, -1)
	checkHigh := !math.IsNaN(usl) && !math.IsInf(usl, 1)
	failed := 0

	// set min, max if not set

	min, _ := stats.Min(v)
	max, _ := stats.Max(v)
	mean, _ := stats.Mean(v)

	if lsl < min {
		min = lsl
	}

	if usl > max {
		max = usl
	}

	step := (max - min) / float64(n)

	h := make([]float64, int(n)+1)
	for i := 0; i < len(v); i++ {
		e := (v[i] - min) / step
		h[int(e)]++

		if checkLow {
			if v[i] < lsl {
				failed++
			}
		}
		if checkHigh {
			if v[i] > usl {
				failed++
			}
		}
	}

	// normalize height
	hmax, _ := stats.Max(h)
	hmax = float64(height-100) / hmax
	for i := 0; i < len(h); i++ {
		h[i] *= hmax
	}

	// SVG header and viewport
	s := fmt.Sprintf(`<?xml version="1.0"?><svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0,0,%d,%d"><desc>R SVG Plot!</desc><rect width="100%%" height="100%%" style="fill:#FFFFFF"/>`, width, height, width, height)
	s += "\n"

	// Histogram bars

	w := (float64(width) - 2.0) / float64(n)
	// log.Printf("width of bar: %f, number of bars %d, w*n %f\n", w, n, w*float64(n))

	hh := height - 100
	y0 := hh
	x1 := 1.0
	for i := 0; i < n; i++ {
		y1 := int(hh - int(h[i]))
		s += fmt.Sprintf("<polygon points='%f,%d %f,%d %f,%d, %f,%d' style='stroke-width:1;stroke:#999;fill:#ADD8E6;stroke-opacity:1.000000;fill-opacity:1.000000' />\n", x1, y0, x1, y1, x1+w, y1, x1+w, y0)
		x1 += w
	}

	low, high, s1 := Xaxis(min, max, height, width)

	s += s1

	s += fmt.Sprintf("<line x1='0' y1='%d' x2='%d' y2='%d' style='stroke:rgb(100,100,100);stroke-width:0.5px' />\n", height-80, width, height-80)

	// Text
	x := 10.0
	y := height - 80 + 20
	s += fmt.Sprintf(t1, x, y, "mean")
	s += fmt.Sprintf(t2, x+50, y, mean)
	y += 14
	s += fmt.Sprintf(t1, x, y, "min")
	s += fmt.Sprintf(t2, x+50, y, min)
	y += 14
	s += fmt.Sprintf(t1, x, y, "max")
	s += fmt.Sprintf(t2, x+50, y, max)

	x = float64(width) / 2.0
	y = height - 80 + 20
	s += fmt.Sprintf(t1, x, y, "out of spec")
	s += fmt.Sprintf(t2pc, x+70, y, float64(failed)/float64(len(v))*1000000)
	if checkLow {
		y += 14
		s += fmt.Sprintf(t1, x, y, "lsl")
		s += fmt.Sprintf(t2, x+70, y, lsl)
	}
	if checkHigh {
		y += 14
		s += fmt.Sprintf(t1, x, y, "usl")
		s += fmt.Sprintf(t2, x+70, y, usl)
	}

	// Limits, if any
	log.Printf("width %d, high %f, low %f, lsl %f\n", width, high, low, lsl)
	if !math.IsNaN(lsl) && lsl >= low {
		xpos := (lsl - low) / (high - low) * float64(width)
		s += fmt.Sprintf("<line x1='%f' y1='%d' x2='%f' y2='%d' style='stroke:rgb(255,100,100);stroke-width:2px' stroke-dasharray='4'/>\n", xpos, height-100, xpos, 0)
	}
	if !math.IsNaN(usl) && usl < high {
		xpos := (usl - low) / (high - low) * float64(width)
		s += fmt.Sprintf("<line x1='%f' y1='%d' x2='%f' y2='%d' style='stroke:rgb(255,100,100);stroke-width:2px' stroke-dasharray='4' />\n", xpos, height-100, xpos, 0)
	}

	return s + "</svg>"
}

func Tick(step float64) float64 {

	m := 1.0
	for {
		if step > 10 {
			step = step / 10
			m *= 10
		} else if step < 1 {
			step = step * 10
			m /= 10
		} else {
			break
		}
	}

	return math.Round(step) * m
}

// Lowest tick, lower than min.
func TickMin(min, tick float64) float64 {
	n := int(min / tick)
	if min < float64(n)*tick {
		n--
	}
	return tick * float64(n)
}

// Lowest tick, lower than min.
func TickMax(max, tick float64) float64 {
	n := int(max / tick)
	if max > float64(n)*tick {
		n++
	}
	return tick * float64(n)
}

func Xaxis(min, max float64, height, width int) (float64, float64, string) {

	// Determine the x scale, with a clean tick interval
	step := (max - min) / float64(9)
	tick := Tick(step)
	// log.Printf("Tick %f (%f)\n", tick, step)

	tickMin := TickMin(min, tick)
	// log.Printf("TickMin %f (%f)\n", tickMin, min)
	tickMax := TickMax(max, tick)
	// log.Printf("TickMax %f (%f)\n", tickMax, max)

	n := int((tickMax - tickMin) / tick)

	// xscale := tickMax - tickMin
	// log.Printf("x axis %f\n", xscale)

	yval := tickMin + tick

	xi := float64(width) / float64(n)
	x := xi

	s := ""

	y := height - 100
	for i := 1; i < 11; i++ {

		s += fmt.Sprintf("<line x1='%f' y1='%d' x2='%f' y2='%d' style='stroke:rgb(100,100,100);stroke-width:1' />\n", x, y, x, y+5)
		x += xi
	}
	x = xi
	y = height - 86
	for i := 1; i < 11; i++ {

		s += fmt.Sprintf("<text x='%f' y='%d' text-anchor='middle' alignment-baseline='bottom' style='font-size:10; font-family: arial'>%.2f</text>\n", x, y, yval)
		x += xi
		yval += tick
	}

	s += fmt.Sprintf("<line x1='0' y1='%d' x2='%d' y2='%d' style='stroke:rgb(100,100,100);stroke-width:1px' />\n", height-100, width, height-100)

	return tickMin, tickMax, s
}
