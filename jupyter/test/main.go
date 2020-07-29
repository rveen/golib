package main

import (
	"fmt"
	"io/ioutil"

	"github.com/rveen/golib/jupyter"
)

func main() {

	buf, _ := ioutil.ReadFile("julia.ipynb")

	g, _ := jupyter.FromJupyter(buf)

	b, _ := jupyter.ToHTML(g)

	fmt.Println(string(b))
}
