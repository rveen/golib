package main

import (
	"fmt"
	"github.com/rveen/golib/csv"
)

func main() {

	files := []string{"bom.csv", "db.csv", "pkg.csv"}
	bom := csv.ReadTyped(files)
	for k, v := range bom {
		fmt.Printf("%s\n", k)
		for kk, vv := range v {
			fmt.Printf("    %s  %s\n", kk, vv)
		}
	}
}
