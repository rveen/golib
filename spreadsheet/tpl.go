package spreadsheet

import (
	"fmt"
	"strconv"
	"strings"

	// Check if we can get rid of excelize: https://github.com/tealeg/xlsx
	"github.com/xuri/excelize/v2"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

func FillTemplate(template, result string, fields map[string]string, table []map[string]string) error {

	// Open the template
	f, err := excelize.OpenFile(template)
	if err != nil {
		return err
	}

	// Get first sheet name
	m := f.GetSheetMap()
	for _, sh := range m {

		// Range over rows
		rows, _ := f.GetRows(sh)
		//fmt.Printf("Total rows: %d\n", len(rows))

		thead := -1
		tfirst := -1
		tlast := -1

		// Fill 'Field=' (a single field)

		for i := 0; i < len(rows); i++ {
			r := rows[i]
			// fmt.Printf("%d ", i)
			for j := 0; j < len(r); j++ {
				c := r[j]

				if strings.HasPrefix(c, "Field=") {
					k := c[6:]
					r[j] = fields[k]
					if r[j] == "" {
						r[j] = k + "?"
					}
					// fmt.Printf("set %s = %s\n", xy(j, i), r[j])
					f.SetCellValue(sh, xy(j, i), r[j])
				}
				// fmt.Printf("%s ", r[j])

				if strings.HasPrefix(c, "Column=") {
					if tfirst == -1 {
						tfirst = j
					}
					thead = i
					tlast = j
				}
			}
			// fmt.Println("")
		}

		// Fill 'Column=' (a table)
		if thead >= 0 {

			// Header of table is thead if we count from 0 (add 1 as Excel starts at 1)
			//
			// One empty table row is at thead+1. We insert n-1 empty rows before that one
			// and copy its style
			//
			// Get the style of the first cell in the table

			c1 := xy(tfirst, thead+1)
			cn := xy(tlast+1, thead+len(table))
			nsty, _ := f.GetCellStyle(sh, c1)
			f.InsertRows(sh, thead+2, len(table)-1)
			// fmt.Printf("insert into %s %d rows before %d\n", sh, len(table)-1, thead+2)
			f.SetCellStyle(sh, c1, cn, nsty)

			rows, _ := f.GetRows(sh)
			// fmt.Printf("Total rows after insert: %d\n", len(rows))
			// fmt.Printf("c1 %s cn %s\n", c1, cn)

			// Fill in the values
			// Number of rows in table is len(table)
			for i := 0; i < len(table); i++ {
				for j := 0; j < len(rows[thead]); j++ {
					c := rows[thead][j]
					if strings.HasPrefix(c, "Column=") {
						name := c[7:]

						f.SetCellValue(sh, xy(j, thead+1+i), table[i][name])
					}
				}
			}

			for j := 0; j < len(rows[thead]); j++ {
				c := rows[thead][j]
				if strings.HasPrefix(c, "Column=") {
					name := cases.Title(language.English, cases.Compact).String(c[7:])
					f.SetCellValue(sh, xy(j, thead), name)
				}
			}
		}

	}

	if err := f.SaveAs(result); err != nil {
		fmt.Println(err)
	}

	return nil
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
