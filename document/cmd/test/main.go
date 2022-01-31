package main

import (
	"fmt"
	"os"

	"github.com/rveen/golib/document"
)

func main() {

	b, err := os.ReadFile("doc.md")
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	d, _ := document.New(string(b))

	fmt.Println(d.Html())
}
