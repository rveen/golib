package record

import "testing"

func TestStrIntBool(t *testing.T) {
	r := Record{
		Type:  TypeComponent,
		Index: 3,
		Props: map[string]string{
			"RECORD":       "1",
			"ORIENTATION":  "2",
			"ISMIRRORED":   "T",
			"LIBREFERENCE": "R1k",
		},
	}

	if got := r.Str("LIBREFERENCE"); got != "R1k" {
		t.Errorf("Str: got %q want %q", got, "R1k")
	}
	if got := r.Str("ABSENT"); got != "" {
		t.Errorf("Str absent: got %q want %q", got, "")
	}

	if v, ok := r.Int("ORIENTATION"); !ok || v != 2 {
		t.Errorf("Int: got %d,%v want 2,true", v, ok)
	}
	if _, ok := r.Int("ABSENT"); ok {
		t.Error("Int absent: expected false")
	}
	if _, ok := r.Int("LIBREFERENCE"); ok {
		t.Error("Int non-numeric: expected false")
	}

	if !r.Bool("ISMIRRORED") {
		t.Error("Bool T: expected true")
	}
	if r.Bool("RECORD") {
		t.Error("Bool '1': expected false (only 'T' is true)")
	}
}

func TestIntDef(t *testing.T) {
	r := Record{Props: map[string]string{"X": "42"}}
	if got := r.IntDef("X", 0); got != 42 {
		t.Errorf("IntDef: got %d want 42", got)
	}
	if got := r.IntDef("MISSING", 99); got != 99 {
		t.Errorf("IntDef missing: got %d want 99", got)
	}
}

func TestTypeNameCoverage(t *testing.T) {
	// Every SkipType must appear in TypeName.
	for id := range SkipTypes {
		if _, ok := TypeName[id]; !ok {
			t.Errorf("SkipType %d missing from TypeName", id)
		}
	}
}
