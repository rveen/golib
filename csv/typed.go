package csv

import (
	"strings"
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

	if len(aa) < 2 {
		return nil
	}


	// First: flatten all aa[] except aa[0]
	flatten(aa)

	// We have items in aa[0] and augmented info in aa[1]
	// Now we want to augment items in aa[0] with items with
	// the same name in aa[1]
	merge(aa)

	ix := make(map[string]map[string]string)

	// Index all items and types by name
	for i:=0; i<2; i++ {
		for _, item := range aa[i] {
			name := item["name"]
			if name == "" {
				continue
			}
			ix[name] = item
		}
	}

	// Type inheritance.
	// Walk through the instances. If a type attribute is found,
	// look up that type, recursively.
	
	items := aa[0]
	ix2 := make(map[string]map[string]string)

	for _, item := range items {
		addTypeData(item, ix)
	}

	for _, item := range items {
		ix2[item["name"]] = item
	}

	return ix2
}

// Flatten aa[1...]
// Preserve aa[0]
func flatten(aa [][]map[string]string) {

	if len(aa)<3 {
		return
	}

	// First: merge items with same name into aa[1]
	// 
	// Fields from lower indexes into aa take precedence
	for i:=len(aa)-1;i>1;i-- {
		for k,vv := range aa[i] {
			item := aa[1][k]
			if item == nil {
				aa[1][k] = vv
				continue
			}
			// range over all the fields in the higher aa
			// if non existent in lower aa, add
			for k2,v := range vv {
				if item[k2] == "" {
					item[k2] = v
				}
			}
		}
	}
}

func merge(aa [][]map[string]string) {

	if len(aa)<2 {
		return
	}

	for k,vv := range aa[1] {
			item := aa[0][k]
			if item == nil {
				continue
			}
			// range over all the fields in the higher aa
			// if non existent in lower aa, add
			for k2,v := range vv {
				if item[k2] == "" {
					item[k2] = v
				}
			}
	}
}

func addTypeData(item map[string]string, ix map[string]map[string]string) {

	types := strings.Fields(item["type"])

	for _, typ := range types {

		typeObj := ix[typ]
		if typeObj == nil {
			// Item has a type but there is no further info on that type
			// Type info is not mandatory.
			continue
		}
		addTypeData(typeObj, ix)

		// Add all *new* fields of typeObj to item, except name and type
		// tags are appended.
		for k, v := range typeObj {
			if k == "name" || k == "type" {
				continue
			}
			if k == "tags" {
				tags := item["tags"]
				if tags != "" {
					item[k] = tags + " " + v
				} else {
					item[k] = v
				}
			} else {
				if item[k] == "" {
					item[k] = v
				}
			}
		}
	}
}
