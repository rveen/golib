package main

import (
	ss "golib/spreadsheet"
	"os"
	"path"
)

func main() {

	fields := make(map[string]string)

	fields["Code"] = "1298347"
	fields["VariantName"] = "V1"

	var table []map[string]string

	t1 := make(map[string]string)
	t1["Code"] = "1298347"
	t1["Quantity"] = "2"

	table = append(table, t1)

	// Locate the template directory
	ex, _ := os.Executable()
	// dir := path.Dir(ex) + "/templates/"
	dir := path.Dir(ex) + "/"

	ss.FillTemplate(dir+"template.xlsx", dir+"result.xlsx", fields, table)
}
