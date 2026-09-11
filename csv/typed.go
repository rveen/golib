package csv

import (
	"fmt"
	"slices"
	"strings"
)

// Source is a row of a CSV file: the file, and the value of the row's name
// field.
type Source struct {
	File string
	Name string
}

// Field is a field of a typed item, with the rows its value comes from: one
// row, or, for the accumulated fields tags and type, every row that added to
// it, nearest first.
type Field struct {
	Value   string
	Sources []Source
}

// Typed is the result of ReadTypedFields.
type Typed struct {
	// Items are the items of the first file, by name.
	Items map[string]map[string]Field

	// Warnings report type names that no row defines, and type cycles.
	Warnings []string

	warned map[string]bool
}

// accumulated reports whether a field collects the values of the whole type
// chain instead of taking the nearest one.
func accumulated(field string) bool {
	return field == "tags" || field == "type"
}

// ReadTyped reads an instance file followed by type files (see the readme).
// Files that cannot be read are ignored; use ReadTypedErr to get an error
// instead.
func ReadTyped(files []string) map[string]map[string]string {
	m, _ := readTyped(files, false)
	return m
}

// ReadTypedErr is ReadTyped, but returns an error if one of the files
// cannot be read.
func ReadTypedErr(files []string) (map[string]map[string]string, error) {
	return readTyped(files, true)
}

func readTyped(files []string, strict bool) (map[string]map[string]string, error) {
	t, err := readTypedFields(files, strict)
	if err != nil || len(t.Items) == 0 {
		return nil, err
	}
	m := make(map[string]map[string]string, len(t.Items))
	for name, fields := range t.Items {
		r := make(map[string]string, len(fields))
		for k, f := range fields {
			r[k] = f.Value
		}
		m[name] = r
	}
	return m, nil
}

// ReadTypedFields is ReadTypedErr, and also tells for every field of every
// item which rows its value comes from.
//
// The first file holds the items; all files, the first included, can define
// types. Rows with the same name, in the same or different files, are merged:
// for each field the earlier row wins, and tags and type add up. An item gets
// its own fields first, then those of its types that it does not have yet,
// the types taken in the order its type field lists them, and each type
// resolved the same way. So the nearest definition wins: an item over its
// type, a type over the type it has itself.
func ReadTypedFields(files []string) (*Typed, error) {
	return readTypedFields(files, true)
}

func readTypedFields(files []string, strict bool) (*Typed, error) {

	rows := map[string]map[string]Field{}
	var items []string // names in the first file, in order
	seen := map[string]bool{}

	for i, file := range files {
		a, err := Read(file)
		if err != nil {
			if strict {
				return nil, err
			}
			continue
		}
		for _, r := range a {
			name := r["name"]
			if name == "" {
				continue
			}
			if i == 0 && !seen[name] {
				seen[name] = true
				items = append(items, name)
			}
			row := rows[name]
			if row == nil {
				row = map[string]Field{}
				rows[name] = row
			}
			src := Source{File: file, Name: name}
			for k, v := range r {
				f, ok := row[k]
				switch {
				case !ok:
					row[k] = Field{v, []Source{src}}
				case accumulated(k):
					row[k] = Field{join(f.Value, v), append(slices.Clone(f.Sources), src)}
				}
			}
		}
	}

	t := &Typed{Items: map[string]map[string]Field{}, warned: map[string]bool{}}
	for _, name := range items {
		t.Items[name] = t.resolve(name, rows, nil)
	}
	return t, nil
}

// resolve returns the fields of row name: its own, then those of its types
// that it does not have yet. path holds the rows being resolved, to stop
// cycles.
func (t *Typed) resolve(name string, rows map[string]map[string]Field, path []string) map[string]Field {

	row := rows[name]
	out := make(map[string]Field, len(row))
	for k, f := range row {
		out[k] = f
	}

	path = append(slices.Clone(path), name)
	for _, typ := range strings.Fields(row["type"].Value) {
		if slices.Contains(path, typ) {
			t.warn("type cycle: %s → %s", strings.Join(path, " → "), typ)
			continue
		}
		if rows[typ] == nil {
			t.warn("%s: type %s is not defined", name, typ)
			continue
		}
		for k, f := range t.resolve(typ, rows, path) {
			if k == "name" {
				continue
			}
			o, ok := out[k]
			switch {
			case !ok:
				out[k] = f
			case accumulated(k):
				out[k] = Field{join(o.Value, f.Value), append(slices.Clone(o.Sources), f.Sources...)}
			}
		}
	}
	return out
}

func (t *Typed) warn(format string, a ...any) {
	w := fmt.Sprintf(format, a...)
	if !t.warned[w] {
		t.warned[w] = true
		t.Warnings = append(t.Warnings, w)
	}
}

// join adds the words of b that a does not have yet.
func join(a, b string) string {
	words := strings.Fields(a)
	for _, w := range strings.Fields(b) {
		if !slices.Contains(words, w) {
			words = append(words, w)
		}
	}
	return strings.Join(words, " ")
}
