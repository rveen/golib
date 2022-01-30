package main

import (
	"embed"
	"fmt"
)

//go:embed static/*
var fs embed.FS

func main() {

	f, err := fs.Open(".")
	defer f.Close()

	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fi, err := f.Stat()
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	fmt.Println(fi.Name(), fi.IsDir())

	// printDir(".", 0)
}

func printDir(path string, level int) {

	ff, err := fs.ReadDir(path)

	if err != nil {
		fmt.Println(err.Error())
		return
	}

	for _, f := range ff {
		for i := 0; i < level; i++ {
			fmt.Printf(" ")
		}

		fmt.Println(f.Name(), f.Type())
		if f.IsDir() {
			if path != "." {
				printDir(path+"/"+f.Name(), level+1)
			} else {
				printDir(f.Name(), level+1)
			}

		}
	}
}
