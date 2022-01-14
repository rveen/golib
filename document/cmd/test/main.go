package main

import (
	"fmt"
	"io/ioutil"

	"github.com/rveen/golib/document"
)

func main() {

	b, err := ioutil.ReadFile("doc.md")
	if err != nil {
		fmt.Println(err.Error())
		return
	}

	doc, _ := document.New(string(b))

	fmt.Println(doc.Data().Text())

	fmt.Println("------------")

	fmt.Println(doc.Part("intro.parameter").Html())

}
