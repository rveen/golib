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

// TestTypePrecedence: the nearest definition wins, multiple types are taken
// in order, tags add up, and every value knows its rows.
func TestTypePrecedence(t *testing.T) {

	dir := writeFiles(t, map[string]string{
		"bom.csv": "name, type, tags\nR1, R0603, analog\nR2, RT50 R0603,\n",
		"db.csv": "name, type, class, tmax, tags, package\n" +
			"R0603, SMD, R, 155, thick, 0603\nRT50, , R, 125, , 0805\nSMD, , X, 100, smd, 9999\n",
	})
	bom, db := filepath.Join(dir, "bom.csv"), filepath.Join(dir, "db.csv")

	typed, err := ReadTypedFields([]string{bom, db})
	if err != nil {
		t.Fatal(err)
	}
	if len(typed.Warnings) != 0 {
		t.Errorf("warnings %v", typed.Warnings)
	}

	for name, want := range map[string]map[string]string{
		// R0603 over its own type SMD
		"R1": {"class": "R", "tmax": "155", "package": "0603", "tags": "analog thick smd", "type": "R0603 SMD"},
		// RT50 before R0603, and R0603 before SMD
		"R2": {"class": "R", "tmax": "125", "package": "0805", "tags": "thick smd", "type": "RT50 R0603 SMD"},
	} {
		for k, v := range want {
			if got := typed.Items[name][k].Value; got != v {
				t.Errorf("%s: %s = %q, want %q", name, k, got, v)
			}
		}
	}

	r1 := typed.Items["R1"]
	if got, want := r1["class"].Sources, []Source{{db, "R0603"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("R1 class sources %v, want %v", got, want)
	}
	if got, want := r1["tags"].Sources, []Source{{bom, "R1"}, {db, "R0603"}, {db, "SMD"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("R1 tags sources %v, want %v", got, want)
	}

	// ReadTyped gives the same values
	if m := ReadTyped([]string{bom, db}); m["R1"]["class"] != "R" || m["R2"]["tmax"] != "125" {
		t.Errorf("ReadTyped: %v", m)
	}
}

// TestRowMerge: rows with the same name in two files are merged, the earlier
// file winning field by field, with tags adding up.
func TestRowMerge(t *testing.T) {

	dir := writeFiles(t, map[string]string{
		"bom.csv": "name, type\nX1, P1\n",
		"db.csv":  "name, val, tags\nP1, 10, a\n",
		"pkg.csv": "name, val, extra, tags\nP1, 20, e, b\n",
	})
	db, pkg := filepath.Join(dir, "db.csv"), filepath.Join(dir, "pkg.csv")

	typed, err := ReadTypedFields([]string{filepath.Join(dir, "bom.csv"), db, pkg})
	if err != nil {
		t.Fatal(err)
	}
	x1 := typed.Items["X1"]
	if x1["val"].Value != "10" || !reflect.DeepEqual(x1["val"].Sources, []Source{{db, "P1"}}) {
		t.Errorf("val %+v, want 10 from db.csv", x1["val"])
	}
	if x1["extra"].Value != "e" || !reflect.DeepEqual(x1["extra"].Sources, []Source{{pkg, "P1"}}) {
		t.Errorf("extra %+v, want e from pkg.csv", x1["extra"])
	}
	if x1["tags"].Value != "a b" {
		t.Errorf("tags %q, want \"a b\"", x1["tags"].Value)
	}
}

func TestTypeWarnings(t *testing.T) {

	dir := writeFiles(t, map[string]string{
		"bom.csv": "name, type\nX1, A\nX2, NOPE\n",
		"db.csv":  "name, type, v\nA, B, 1\nB, A, 2\n",
	})
	typed, err := ReadTypedFields([]string{filepath.Join(dir, "bom.csv"), filepath.Join(dir, "db.csv")})
	if err != nil {
		t.Fatal(err)
	}
	if v := typed.Items["X1"]["v"].Value; v != "1" {
		t.Errorf("X1 v = %q, want 1 (from A)", v)
	}
	w := strings.Join(typed.Warnings, "\n")
	if !strings.Contains(w, "type cycle: X1 → A → B → A") || !strings.Contains(w, "X2: type NOPE is not defined") {
		t.Errorf("warnings:\n%s", w)
	}
}

// TestEmptyFile: a file without rows is no error.
func TestEmptyFile(t *testing.T) {

	dir := writeFiles(t, map[string]string{
		"bom.csv":      "name\nR1\n",
		"empty.csv":    "",
		"comments.csv": "# only a comment\n",
	})
	m, err := ReadTypedErr([]string{
		filepath.Join(dir, "bom.csv"), filepath.Join(dir, "empty.csv"), filepath.Join(dir, "comments.csv"),
	})
	if err != nil || m["R1"] == nil {
		t.Errorf("items %v, error %v", m, err)
	}
}
