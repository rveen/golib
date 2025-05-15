package csv

import (
	"fmt"
	"testing"
)

func TestSplit(t *testing.T) {

	// csv := "a,b,  \"c, d\", 'a, c', d, 123#"
	csv := "PAS-FLM-DE-01, \"Changes of termination, surface finish, shape, color, appearance or dimension structure - Lead Diameter / Thickness\""

	ss := Split(csv)

	for i, s := range ss {
		fmt.Printf("%d [%s]\n", i, s)
	}

}

func TestSplit_2(t *testing.T) {

	csv := `C3, B43268, 100u, "0 0 0 0 0 0 0 0 0 0", "0 0 0 0 0 0.9 0.9 0.9 0.6 0.6" , "40 40 95 135 155 40 40 95 135 155" , "7884 26280 85410 10512 1314 480 1600 5200 640 80"`

	ss := Split(csv)

	for i, s := range ss {
		fmt.Printf("%d [%s]\n", i, s)
	}

}
