# CSV reader

**Read(file string) ([]map[string]string, error)**

Reads a CSV file and returns an array of maps, one map per row, except for
the first row, which holds the field names.

Each map is a row of the CSV file: the keys are the field names from the first
row, and the values are those of the row. Values are trimmed and empty values
are left out. Lines starting with `#` are comments.

**ReadTyped(files []string) map[string]map[string]string**

Reads one or more CSV files and returns the items of the first file, by the
value of their `name` field, each with its fields.

The other files define types. An item, or a type, with a `type` field
inherits the fields of the rows whose names it lists there:

- Rows with the same `name`, in the same or in different files, are merged:
  for each field the earlier row wins (the first file before the second, and so
  on), and `tags` and `type` add up.
- An item gets its own fields first, then those of its types that it does not
  have yet. The types are taken in the order the `type` field lists them, and
  each type is resolved the same way, recursively. So the nearest definition
  wins: an item over its type, a type over the type it has itself.
- `tags` (and `type`) collect the words of the whole chain, nearest first,
  without repetitions.

The `type` field can hold more than one name, separated by spaces, for example
`type1 type2`. Names must not contain spaces.

Files that cannot be read are ignored. **ReadTypedErr** returns an error
instead.

**ReadTypedFields(files []string) (\*Typed, error)**

Like ReadTypedErr, but every field value comes with the rows (file and name) it
was taken from, and the result lists warnings for type names that no row
defines and for type cycles.
