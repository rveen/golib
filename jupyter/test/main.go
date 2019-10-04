package main

import (
	"fmt"
	"golib/jupyter"
	"io/ioutil"
	//"github.com/rveen/ogdl"
)

func main() {

	buf, _ := ioutil.ReadFile("julia.ipynb")

	g, _ := jupyter.FromJupyter(buf)

	// fmt.Printf("%s\n", g.Show())

	b, _ := jupyter.ToHTML(g)

	fmt.Println(string(b))
}
