package main

import (
	// "fmt"
	"github.com/rveen/golib/fn"
)

func main() {

	file := fn.New("/files/go/src/github.com/rveen/golib/fn/test")

	file.Get("svn/aa")
	//fmt.Println(file.Path, file.Type, file.N)

	file.Get("test/section1")
	//fmt.Println(file.Path, file.Type, file.N)
}
