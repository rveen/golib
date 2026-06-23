package schema_test

// Architecture invariant: schema must not import altium or emit packages.
// Verified at compile time — if someone adds a forbidden import, schema_test
// itself will fail to build because those packages would try to import schema,
// creating a cycle.  This file also asserts the rule explicitly via go/build.

import (
	"go/build"
	"strings"
	"testing"
)

func TestSchemaImports(t *testing.T) {
	ctx := build.Default
	pkg, err := ctx.Import("golib/formats/altium/schema", "", 0)
	if err != nil {
		t.Fatalf("cannot import schema package: %v", err)
	}
	forbidden := []string{"altium/altium", "altium/emit"}
	for _, imp := range pkg.Imports {
		for _, f := range forbidden {
			if strings.Contains(imp, f) {
				t.Errorf("schema imports forbidden package %q (violates IR independence)", imp)
			}
		}
	}
}
