package main

import (
	"fmt"
	"github.com/rveen/golib/csv"
)

func main() {

	files := []string{"bom.csv", "db.csv"}
	bom := csv.CsvTyped(files)
	for k, v := range bom {
		fmt.Printf("%s\n", k)
		for kk, vv := range v {
			fmt.Printf("    %s  %s\n", kk, vv)
		}
	}
}
