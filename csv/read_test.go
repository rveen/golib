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
