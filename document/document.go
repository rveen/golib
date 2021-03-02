// A markdown processor. Converts markdown to an internal format that may be
// rendered later as HTML or treated as data
//
// The markdown does follow mostly the de-facto rules, but not all.
//
package document

import (
	// "bytes"
	"strings"

	"github.com/rveen/golib/eventhandler"
	"github.com/rveen/golib/parser"
	"github.com/rveen/ogdl"
)

type Document struct {
	stream *eventhandler.EventHandler
	ix     int
}

// New parses a markdown+ text string and returnes a Document object.
func New(s string) (*Document, error) {

	doc := &Document{}

	doc.stream = eventhandler.New()
	p := parser.New([]byte(s), doc.stream)

	for block(p) {
	}

	return doc, nil
}

// Graph returns the event stream produced by the parser as a Graph.
func (doc *Document) Graph() *ogdl.Graph {
	return doc.stream.Graph()
}

// Html returnes the Document in HTML format
func (doc *Document) Html() string {

	var sb strings.Builder

	doc.ix = 0

	for {
		s, n := doc.stream.Item(doc.ix)

		if n < 0 {
			break
		}

		doc.ix++

		switch s {
		case "!pre":
			doc.codeToHtml(&sb)
		case "!p":
			doc.textToHtml(&sb)
		case "!h":
			doc.headerToHtml(&sb)
		case "!ul":
			doc.listToHtml(&sb, 1)
		case "!tb":
			doc.tableToHtml(&sb)
		}
	}

	return sb.String()
}

// Part returns the part of the document indicated by the given path.
func (doc *Document) Part(path string) *Document {
	return nil
}

// Data returns the Document as OGDL data
func (doc *Document) Data() *ogdl.Graph {

	eh := eventhandler.New()
	doc.ix = 0

	for {
		s, n := doc.stream.Item(doc.ix)

		if n < 0 {
			break
		}

		doc.ix++

		switch s {
		case "!h":
			doc.headerToData(eh)
		case "!p":
			// doc.textToData(eh)
		case "!tb":
			doc.tableToData(eh)
		}
	}

	return eh.Graph()
}
