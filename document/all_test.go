package document

import (
	"testing"
	//"github.com/rveen/ogdl"
	"fmt"
)

/*
func Test1(t *testing.T) {
	g, _ := New("hola\ncaracola\n>holas\ncara\n\npa\\b(ra)rafo\n# Requirements\ntexto\n\n## Functional requirements {#functional}\nSome functional requirements here\n- item 1\n- item 2")

	fmt.Println(g.Show())

	fmt.Println(ToHtml(g))

	fmt.Println(ToData(g).Show())
}

func TestTable1(t *testing.T) {
	g, _ := New("|| Milestones | A | B |\n|---\n| Start date | 2021 | 2022 |\n| End date | 2023 | 2024 |\n")

	fmt.Println(g.Show())

	fmt.Println(ToData(g).Show())
}

func TestTable2(t *testing.T) {
	g, _ := New("|| Milestones | Start date | End data |\n| A | 2021 | 2022 |\n| B | 2023 | 2024 |\n")

	fmt.Println(g.Show())

	fmt.Println(ToData(g).Show())
}

func TestTable3(t *testing.T) {
	g, _ := New("|| Version | 1.0 |\n| Date | 2022 |\n| Type | X |\n")

	fmt.Println(g.Show())

	fmt.Println(ToData(g).Show())
}

func TestTable4(t *testing.T) {
	g, _ := New("| Version | Date | Type |\n| 1 | 2021 | X |\n| 2 | 2023 | Y |\n")

	fmt.Println(g.Show())

	fmt.Println(ToData(g).Show())
}

func TestNormalize(t *testing.T) {
	s := Normalize("àtomar por el cúlö")
	fmt.Println(s)
}

func TestCode(t *testing.T) {
	g, _ := New("Hola\n\n```go\nEsto es código\n`\n")
	fmt.Println(g.Show())
}

func TestHeader(t *testing.T) {
	doc, _ := New("# Requirements \n## Functional requirements {#functional}\n")

	fmt.Println(doc.Data().Show())
}

func TestList(t *testing.T) {
	doc, _ := New("- item 1\n- item 2 {#i2}\n  - item 2.1\n- item 3")

	fmt.Println(doc.Html())
}

func TestTable_A(t *testing.T) {
	// Table without heading, just a matrix
	doc, _ := New("| Version | Date | Type |\n| 1 | 2021 | X |\n| 2 | 2023 | Y |\n")

	for i := 0; i < doc.stream.Len(); i++ {
		s, n := doc.stream.Item(i)
		fmt.Println(n, s)
	}
}

func TestTable_B(t *testing.T) {
	// Table with first row as header
	doc, _ := New("| Version | Date | Type |\n|---|---|\n| 1 | 2021 | X |\n| 2 | 2023 | Y |\n")

	for i := 0; i < doc.stream.Len(); i++ {
		s, n := doc.stream.Item(i)
		fmt.Println(n, s)
	}

	fmt.Println(doc.Data().Show())

}


func TestTable_C(t *testing.T) {
	// Table with first column as header
	doc, _ := New("|| Version | 1 | 2 |\n| Date | 2021 | 2022 |\n| Type | X | Y |\n")

	for i := 0; i < doc.stream.Len(); i++ {
		s, n := doc.stream.Item(i)
		fmt.Println(n, s)
	}

	fmt.Println(doc.Data().Show())
}

func TestTable_D(t *testing.T) {
	// Table with first row and column as header
	doc, _ := New("|| Parameter | Min | Max |\n|---|---|\n| Vbat | 6 | 18 |\n| Idd | 0.03 | 0.1 |\n")

	for i := 0; i < doc.stream.Len(); i++ {
		s, n := doc.stream.Item(i)
		fmt.Println(n, s)
	}

	fmt.Println(doc.Data().Show())
}

func TestText(t *testing.T) {
	doc, _ := New("hola\ncaracola\n y hola y hola\n\nOtro párrafo")

	fmt.Println(doc.Html())

}
*/

func BenchmarkDocNew(b *testing.B) {

	for i := 0; i < b.N; i++ {
		doc, _ := New("|| Parameter | Min | Max |\n|---|---|\n| Vbat | 6 | 18 |\n| Idd | 0.03 | 0.1 |\n")
		doc.Html()
	}
}

func BenchmarkDocNew2(b *testing.B) {

	for i := 0; i < b.N; i++ {
		doc, _ := New("|| Parameter | Min | Max |\n|---|---|\n| Vbat | 6 | 18 |\n| Idd | 0.03 | 0.1 |\n")
		doc.Html2()
	}
}

func TestHtml2(t *testing.T) {

	doc, _ := New("- item 1\n-item 2\n - item 2.1\n- item 3")
	s := doc.Html2()
	fmt.Println(s)
}
