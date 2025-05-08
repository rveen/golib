package id

import (
	"fmt"
	"testing"
)

func TestIsUniqueID(t *testing.T) {

	s := UniqueID()

	fmt.Printf("%t\n", IsUniqueID(s))
	fmt.Printf("%f\n", Entropy(s))

	/*
		min := 1000.0
		for i := 0; i < 10000000; i++ {
			s := UniqueID()
			e := Entropy(s)
			if e < min {
				min = e
			}
		}

		fmt.Printf("min(e) = %f\n", min)
	*/
}
