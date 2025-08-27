package main

import (
	"fmt"

	"github.com/rveen/golib/ini"
)

func main() {

	g, _ := ini.Load("test.ini")
	fmt.Println(g.Text())
}
