package main

import (
	"fmt"
	"strings"

	"github.com/xuri/excelize/v2"
)

func rowName(n int) string {

	var b []byte

	a := n / 26
	r := n % 26

	if n > 25 {
		b = append(b, byte('A'+a-1))
	}

	b = append(b, byte('A'+r))
	return string(b)
}

func main() {

	f, err := excelize.OpenFile("test.xlsx")
	if err != nil {
		return
	}

	m := f.GetSheetMap()
	for _, sh := range m {
		fmt.Printf("[%s]\n", sh)
	}

	rows, err := f.Rows(m[1])
	if err != nil {
		fmt.Println(err)
		return
	}

	nr := 1
	for rows.Next() {
		row, err := rows.Columns()
		if err != nil {
			fmt.Println(err)
		}

		fmt.Printf("ROW %d: ", nr)
		nr++

		for i, colCell := range row {
			if colCell != "" {
				colCell = strings.ReplaceAll(colCell, "\n", "_")
				fmt.Printf("[%s] %s, ", rowName(i), colCell)
			}
		}
		fmt.Println("")
	}
	if err = rows.Close(); err != nil {
		fmt.Println(err)
	}

}
