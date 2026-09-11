package csv

import (
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func writeFiles(t *testing.T, files map[string]string) string {
	dir := t.TempDir()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestReadTypedErr(t *testing.T) {

	dir := writeFiles(t, map[string]string{
		"bom.csv": "name, type, tags\nU1, OPAMP, interface\nU2, OPAMP,\n",
		"db.csv":  "name, class, tags, type\nOPAMP, U, analog, SOIC\nSOIC, , smd,\n",
	})
	files := []string{filepath.Join(dir, "bom.csv"), filepath.Join(dir, "db.csv")}

	m, err := ReadTypedErr(files)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 2 {
		t.Fatalf("got %d items, want 2 (only the items of the first file)", len(m))
	}

	for name, want := range map[string][]string{
		"U1": {"analog", "interface", "smd"},
		"U2": {"analog", "smd"},
	} {
		if m[name]["class"] != "U" {
			t.Errorf("%s: class = %q, want U (inherited)", name, m[name]["class"])
		}
		tags := strings.Fields(m[name]["tags"])
		sort.Strings(tags)
		if !reflect.DeepEqual(tags, want) {
			t.Errorf("%s: tags = %q, want %q", name, tags, want)
		}
	}
}

func TestReadTypedMissingFile(t *testing.T) {

	dir := writeFiles(t, map[string]string{
		"bom.csv": "name, class\nR1, R\n",
	})
	files := []string{filepath.Join(dir, "bom.csv"), filepath.Join(dir, "missing.csv")}

	if _, err := ReadTypedErr(files); err == nil {
		t.Error("ReadTypedErr: no error for a missing file")
	}

	// ReadTyped keeps ignoring unreadable files
	m := ReadTyped(files)
	if m["R1"]["class"] != "R" {
		t.Errorf("ReadTyped: R1 class = %q, want R", m["R1"]["class"])
	}
}
