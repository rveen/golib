package files

import (
	"io/ioutil"
	"log"

	"github.com/rveen/ogdl"
)

type Files struct {
	Config *ogdl.Graph
	base   string
}

func (f *Files) init() {
	f.base, _ = f.Config.GetString("path")
}

func (f *Files) List(path string) *ogdl.Graph {

	log.Println("Files.List", path)
	files, err := ioutil.ReadDir(path)

	if err != nil {
		return nil
	}

	r := ogdl.New(nil)
	for _, f := range files {

		if !f.IsDir() {
			continue
		}

		n := r.Add("d")
		n.Add("n").Add(f.Name())
		n.Add("b").Add(f.Size())
		n.Add("t").Add(f.ModTime())
	}

	for _, f := range files {

		if f.IsDir() {
			continue
		}
		n := r.Add("f")

		n.Add("n").Add(f.Name())
		n.Add("b").Add(f.Size())
		n.Add("t").Add(f.ModTime())
	}

	return r
}
