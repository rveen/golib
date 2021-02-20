package main

import (
	"fmt"
	"io/ioutil"

	"github.com/rveen/golib/document"
)

func main() {

	b, _ := ioutil.ReadFile("doc.mdp")

	doc, _ := document.New(string(b))

	fmt.Println(doc.Html())

}
