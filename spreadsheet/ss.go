// This pachage defines a simple spreadsheet object and
// different exporters
//
// Styles:
//   ** bold
//   __ border bottom
//   _* bold + border bottom
//   -- border top
//   %nn set column width
//   #xxxxxx set background color
//
// Example:
//
// var fields = [][]string{
//	{"**Title1", "_*Title2"},
//	{"hola1", "hola2"},
//	{"hola3", "hola4"},
//	{"hola5", "hola6"},
// }
//
// var fields2 = [][]string{
//	{"**Title3", "_*Title4"},
//	{"hola10", "hola20"},
//	{"hola3", "hola4"},
//	{"hola5", "hola6"},
// }
//
// func main() {
//
//	ss := spreadsheet.New("s1")
//	ss.Fields = fields
//
//	ss2 := spreadsheet.New("s2")
//	ss2.Fields = fields2
//
//	b, _ := ss.ToExcel(ss2)
//
//	os.WriteFile("test.xlsx", b, 0644)
// }

package spreadsheet

import (
	// "fmt"
	"log"
	"strconv"
	"strings"

	// Check if we can get rid of excelize: https://github.com/tealeg/xlsx
	"github.com/xuri/excelize/v2"
)

type SpreadSheet struct {
	Name   string
	Fields [][]string
}

var (
	DefStyle, Ul, Bold, BoldLarge, BoldUl, Light, Wrap, TopBorder int
)

func New(name string) *SpreadSheet {
	return &SpreadSheet{Name: name}
}

func ToExcel(sheets ...*SpreadSheet) ([]byte, error) {
	f := excelize.NewFile()
	initStyles(f)

	deleteDefault := true

	for _, sheet := range sheets {
		toExcel(sheet, f)
		if sheet.Name == "Sheet1" {
			deleteDefault = false
		}
	}

	// Excelice creates a default sheet that we don't want
	if deleteDefault {
		f.DeleteSheet("Sheet1")
	}

	b, err := f.WriteToBuffer()
	return b.Bytes(), err
}

// Return an Excel file in []byte, combining the receiving spreadsheet with
// any other optional sheets given as arguments
func (ss *SpreadSheet) ToExcel(sheets ...*SpreadSheet) ([]byte, error) {
	f := excelize.NewFile()
	initStyles(f)

	toExcel(ss, f)

	deleteDefault := true

	for _, sheet := range sheets {
		toExcel(sheet, f)
		if sheet.Name == "Sheet1" {
			deleteDefault = false
		}
	}

	// Excelice creates a default sheet that we don't want
	if deleteDefault {
		f.DeleteSheet("Sheet1")
	}

	b, err := f.WriteToBuffer()
	return b.Bytes(), err
}

// Return an Excel file in []byte, combining the receiving spreadsheet with
// any other optional sheets given as arguments
func (ss *SpreadSheet) ToExcelAdd(file string, sheets ...*SpreadSheet) ([]byte, error) {
	f, err := excelize.OpenFile(file)
	if err != nil {
		log.Println(err.Error())
		f = excelize.NewFile()
	}
	initStyles(f)

	toExcel(ss, f)

	deleteDefault := true

	for _, sheet := range sheets {
		toExcel(sheet, f)
		if sheet.Name == "Sheet1" {
			deleteDefault = false
		}
	}

	// Excelice creates a default sheet that we don't want
	if deleteDefault {
		f.DeleteSheet("Sheet1")
	}

	b, err := f.WriteToBuffer()
	return b.Bytes(), err
}

func (sheet *SpreadSheet) ToCSV() string {

	s := ""

	for i := 0; i < len(sheet.Fields); i++ {

		row := sheet.Fields[i]

		for j := 0; j < len(row); j++ {

			cell := row[j]

			if strings.HasPrefix(cell, "**") {
				cell = cell[2:]
			} else if strings.HasPrefix(cell, "__") {
				cell = cell[2:]
			} else if strings.HasPrefix(cell, "_*") {
				cell = cell[2:]
			}

			s += cell + ","
		}

		s += "\n"

	}
	return s
}

var (
	size        = 9
	orientation = "landscape"
	margin      = 0.5
)

func toExcel(sheet *SpreadSheet, f *excelize.File) {

	f.NewSheet(sheet.Name)

	f.SetPageLayout(sheet.Name, &excelize.PageLayoutOptions{
		Size:        &size,
		Orientation: &orientation,
	})

	f.SetHeaderFooter(sheet.Name, &excelize.HeaderFooterOptions{
		DifferentFirst:   false,
		DifferentOddEven: false,
		OddFooter:        "&C&P",
	})

	f.SetPageMargins(sheet.Name, &excelize.PageLayoutMarginsOptions{
		Bottom: &margin,
		Top:    &margin,
		Left:   &margin,
		Right:  &margin,
	})

	var col byte

	for i := 0; i < len(sheet.Fields); i++ {

		row := sheet.Fields[i]

		for j := 0; j < len(row); j++ {
			col = 'A' + byte(j)
			cellname := string(col) + strconv.Itoa(i+1)

			cell := row[j]
			if strings.HasPrefix(cell, "%") {
				w, _ := strconv.Atoi(cell[1:3])
				f.SetColWidth(sheet.Name, string(col), string(col), float64(w))
				cell = cell[3:]
			}

			if strings.HasPrefix(cell, "#") {
				c := cell[1:7]
				cell = cell[7:]

				sty := colorStyle(c, f)
				f.SetCellStyle(sheet.Name, cellname, cellname, sty)
				f.SetCellValue(sheet.Name, cellname, cell)

			} else if strings.HasPrefix(cell, "**") {
				f.SetCellStyle(sheet.Name, cellname, cellname, Bold)
				f.SetCellValue(sheet.Name, cellname, cell[2:])
			} else if strings.HasPrefix(cell, "*2*") {
				f.SetCellStyle(sheet.Name, cellname, cellname, BoldLarge)
				f.SetCellValue(sheet.Name, cellname, cell[2:])
			} else if strings.HasPrefix(cell, "__") {
				f.SetCellStyle(sheet.Name, cellname, cellname, Ul)
				f.SetCellValue(sheet.Name, cellname, cell[2:])
			} else if strings.HasPrefix(cell, "--") {
				f.SetCellStyle(sheet.Name, cellname, cellname, TopBorder)
				f.SetCellValue(sheet.Name, cellname, cell[2:])
			} else if strings.HasPrefix(cell, "_*") {
				f.SetCellStyle(sheet.Name, cellname, cellname, BoldUl)
				f.SetCellValue(sheet.Name, cellname, cell[2:])
			} else {
				f.SetCellStyle(sheet.Name, cellname, cellname, DefStyle)
				f.SetCellValue(sheet.Name, cellname, cell)
			}
		}

	}

}

func initStyles(f *excelize.File) {
	// Excelize is stupid and idiot. Writing CSS in another fucking language.

	DefStyle, _ = f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
	})
	Ul, _ = f.NewStyle(&excelize.Style{
		Border:    []excelize.Border{{Type: "bottom", Color: "cccccc", Style: 1}},
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
	})
	TopBorder, _ = f.NewStyle(&excelize.Style{
		Border:    []excelize.Border{{Type: "top", Color: "cccccc", Style: 1}},
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
	})
	BoldUl, _ = f.NewStyle(&excelize.Style{
		Border: []excelize.Border{{Type: "bottom", Color: "cccccc", Style: 1}},
		Font:   &excelize.Font{Bold: true},
	})
	Bold, _ = f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})
	BoldLarge, _ = f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 16},
	})
}

func colorStyle(color string, f *excelize.File) int {

	style, _ := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "top", WrapText: true},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{color}},
	})
	return style
}
