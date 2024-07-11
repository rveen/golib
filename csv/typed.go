package csv

import "log"

// First file is the instance list of items, the rest of the
// files are types that add fields to the instances if they match
// Matching happens when an item has a type.
// The 'name' field is the key in all CSV files.
func CsvTyped(files []string) map[string]map[string]string {

	var aa [][]map[string]string
	// var map[string]map[string]string

	for _, file := range files {

		// Read each file into a list of maps
		// Each map (each line in the CSV file) is an item
		// If an item has a 'type' field, that is used later to
		// build the type inheritance
		a, _ := csvRead(file)
		if len(a) != 0 {
			aa = append(aa, a)
		}
	}

	ix := make(map[string]map[string]string)

	// Index all items and types by name
	for _, a := range aa {
		for _, item := range a {
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

	if len(aa) == 0 {
		return nil
	}

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

func addTypeData(item map[string]string, ix map[string]map[string]string) {

	typ := item["type"]
	if typ == "" {
		return
	}
	typeObj := ix[typ]
	if typeObj == nil {
		log.Println("type not found", typ)
	}
	addTypeData(typeObj, ix)

	// Add all fields of typeObj to item, except name and type
	// tags are added!
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
			item[k] = v
		}
	}
}
