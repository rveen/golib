// Package emit defines the common interfaces and types for schematic emitters.
// Each emitter (KiCad, SVG, BOM, …) implements the Emitter interface and
// produces one or more Artifacts plus a Report of warnings and errors.
package emit

import (
	"fmt"

	"github.com/rveen/golib/formats/altium/schema"
)

// Artifact is an in-memory output file produced by an emitter.
type Artifact struct {
	Name string // suggested filename, e.g. "root.kicad_sch"
	Data []byte
}

// Severity classifies a Report note.
type Severity int

const (
	Info Severity = iota
	Warn
	Error
)

// Note is a single diagnostic message attached to a provenance location.
type Note struct {
	Severity Severity
	Message  string
	Prov     schema.Provenance
}

// Report accumulates diagnostic notes during mapping or emission.
type Report struct {
	Notes []Note
}

// Add appends a formatted note to the report.
func (r *Report) Add(s Severity, prov schema.Provenance, format string, args ...any) {
	r.Notes = append(r.Notes, Note{
		Severity: s,
		Message:  fmt.Sprintf(format, args...),
		Prov:     prov,
	})
}

// HasErrors reports whether the report contains any Error-severity notes.
func (r *Report) HasErrors() bool {
	for _, n := range r.Notes {
		if n.Severity == Error {
			return true
		}
	}
	return false
}

// Emitter produces output artifacts from a Schematic.
type Emitter interface {
	// Name returns the short identifier for this emitter, e.g. "kicad" or "svg".
	Name() string
	// Emit converts the schematic to artifacts.  opts is emitter-specific
	// (pass nil for defaults).
	Emit(s *schema.Schematic, opts any) ([]Artifact, *Report, error)
}
