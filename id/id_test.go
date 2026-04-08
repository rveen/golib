package id

import (
	"testing"
)

func TestIsUniqueID(t *testing.T) {
	s := UniqueID()
	if !IsUniqueID(s) {
		t.Errorf("IsUniqueID(%q) = false, want true", s)
	}
}

func TestIsUniqueID_UUIDv4v7(t *testing.T) {
	cases := []struct {
		s    string
		want bool
	}{
		// UUID v4
		{"550e8400-e29b-41d4-a716-446655440000", true},
		// UUID v4 uppercase
		{"550E8400-E29B-41D4-A716-446655440000", true},
		// UUID v7 (lowercase)
		{"01926b3a-f1c0-7e4d-a832-3f5d1b2c9e0a", true},
		// UUID v7 (uppercase)
		{"01926B3A-F1C0-7E4D-A832-3F5D1B2C9E0A", true},
		// variant nibble b
		{"01926b3a-f1c0-7e4d-b832-3f5d1b2c9e0a", true},
		// variant nibble 9
		{"01926b3a-f1c0-7e4d-9832-3f5d1b2c9e0a", true},
		// wrong version (v1)
		{"01926b3a-f1c0-1e4d-a832-3f5d1b2c9e0a", false},
		// wrong variant (c is not 8/9/a/b)
		{"01926b3a-f1c0-7e4d-c832-3f5d1b2c9e0a", false},
		// native 32-char hex (no dashes) — still a valid UniqueID
		{"a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6", true},
		// plain word
		{"hello", false},
	}

	for _, c := range cases {
		got := IsUniqueID(c.s)
		if got != c.want {
			t.Errorf("IsUniqueID(%q) = %v, want %v", c.s, got, c.want)
		}
	}
}
