package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/rveen/golib/document"
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
