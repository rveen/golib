package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"golib/document"
)

func main() {

	flag.Parse()

	b, err := os.ReadFile(flag.Arg(0))
	if err != nil {
		log.Fatal(err.Error())
	}

	doc, err := document.New(string(b))

	if err != nil {
		log.Fatal(err.Error())
	}

	fmt.Println(doc.Data().Text())

}
