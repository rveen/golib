package main

import (
	// "fmt"
	"golib/fs"
)

func main() {

	file := fs.New("/files/go/src/github.com/rveen/golib/fs/test")

	file.Get("svn/aa")
	//fmt.Println(file.Path, file.Type, file.N)

	file.Get("test/section1")
	//fmt.Println(file.Path, file.Type, file.N)
}
