package csv

import (
	"fmt"
	// "strings"
)

// First file is the instance list of items, the rest of the
// files are types that add fields to the instances if they match
// Matching happens when an item has a type.
func ReadTyped(files []string) map[string]map[string]string {

	var aa [][]map[string]string
	// var map[string]map[string]string

	for _, file := range files {

		// Read each file into a list of maps
		// Each map (each line in the CSV file) is an item
		// If an item has a 'type' field, that is used later to
		// build the type inheritance
		a, _ := Read(file)

		if len(a) != 0 {
			aa = append(aa, a)
		}
	}

	if len(aa) == 0 {
		return nil
	}

	// Before flattening we want to remember what are the (main) items,
	// those that appear in the first file
	items := make(map[string]bool)
	for _, o := range aa[0] {
		items[o["name"]] = true
	}

	// This returns all rows of all files in one list.
	// If there where rows with the same name, data is merged (in order, that is
	// data in lower rows has precedence over higher rows).
	all := flatten(aa)

	// For each item, do a recursive type augmentation

	m := make(map[string]map[string]string)

	for _, o := range *all {

		name := o["name"]
		if items[name] == false {
			continue
		}

		typ := o["type"]

		addTypeInfo(typ, o, all)
		m[o["name"]] = o
	}

	return m
}

// Flatten aa[1...]
// Preserve aa[0]
func flatten(aa [][]map[string]string) *[]map[string]string {

	if len(aa) < 2 {
		return &aa[0]
	}

	f1 := aa[len(aa)-1]

	// Fields from lower indexes into aa take precedence
	// So merge down
	for i := len(aa) - 1; i > 0; i-- {
		f0 := aa[i-1]

		for _, row := range f0 {
			name := row["name"]

			// add to or merge into file0
			found := false
			for _, row1 := range f1 {
				name1 := row1["name"]
				if name == name1 {
					found = true

					for k, v := range row {
						if k == "tags" {
							row1[k] += " " + v
						} else {
							row1[k] = v
						}
					}

					break
				}
			}
			if !found {
				f1 = append(f1, row)
			}
		}
	}
	return &f1
}

func addTypeInfo(typ string, o map[string]string, tt *[]map[string]string) {

	for _, t := range *tt {
		name := t["name"]
		if typ == name {

			ttyp := t["type"]
			if ttyp != "" {
				addTypeInfo(ttyp, o, tt)
			}

			for k, v := range t {
				if k == "tags" || k == "type" {
					o[k] += o[k] + " " + v
				} else if o[k] == "" {
					o[k] = v
				}
			}
		}
	}
}

func printlm(aa *[][]map[string]string) {

	for i, file := range *aa {
		fmt.Printf("file %d\n", i)
		for j, row := range file {
			fmt.Printf("  row %d\n", j)
			for k, v := range row {
				fmt.Printf("    %s = %s\n", k, v)
			}
		}
	}
	fmt.Println("------")
}

func printll(aa *[]map[string]string) {

	for j, row := range *aa {
		fmt.Printf("  row %d\n", j)
		for k, v := range row {
			fmt.Printf("    %s = %s\n", k, v)
		}
	}
	fmt.Println("------")
}
