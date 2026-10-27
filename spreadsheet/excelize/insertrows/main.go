package main

import (
	"fmt"
	"strconv"

	"github.com/xuri/excelize/v2"
)

func main() {

	f, err := excelize.OpenFile("template.xlsx")
	if err != nil {
		return
	}

	m := f.GetSheetMap()
	sh := m[1]
	rows, _ := f.GetRows(sh)
	fmt.Printf("Total rows: %d\n", len(rows))

	f.InsertRows(sh, 13, 255)

	//f.InsertRows(sh, thead+2, 200)
	fmt.Printf("insert into %s %d rows before %d\n", sh, 200, 13)

	rows, _ = f.GetRows(sh)
	fmt.Printf("Total rows after insert: %d\n", len(rows))

	if err := f.SaveAs("result.xlsx"); err != nil {
		fmt.Println(err)
	}

}

func xy(x, y int) string {
	var r []byte
	if x < 26 {
		r = make([]byte, 1, 5)
		r[0] = 'A' + byte(x)
	} else if x < 27*26 {
		r = make([]byte, 2, 5)
		r[0] = 'A' - 1 + byte(x/26)
		r[1] = 'A' + byte(x%26)
	} else if x < 16384 {
		r = make([]byte, 3, 5)
		r[2] = 'A' + byte(x%26)
		x /= 26
		r[0] = 'A' - 1 + byte(x/26)
		r[1] = 'A' - 1 + byte(x%26)
	} else {
		return ""
	}
	return string(strconv.AppendUint(r, uint64(y+1), 10))
}
