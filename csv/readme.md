# CSV reader

There are two functions here:

**Read(file string) []map[string]string**

This function reads a CSV file and returns an array
of maps, one map per row, except for the first row that
is assumed to contain the field names. 

Thus, each map is a row or line of the CSV file, where the keys
are the field names taken from the first row, and the values
are those found in the row being processed.

**ReadTyped(files []string) map[string]map[string]string**

This function reads one or more CSV files. A map is returned
where each key is a value of the 'name' field in the first
file, and the value is the row in the form of a map of fields 
and their values.

The rest of the files (2...N) are merged to one map of the same
type as the one for the first file, and act as types for the 
items in the first file. It works as follows: any item with a
'type' field (in any file) inherits the fields of the item whose
'name' appears in that field, recursively. That means that the final
map returned contains the items in the first file augmented
with fields from the rest of the files.

The 'type' field can contain more than one name, for example "type1 type2".
Spaces should be used as separators. Names should not contain spaces.


