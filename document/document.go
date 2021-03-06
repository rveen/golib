// A markdown processor. Converts markdown to an internal format that may be
// rendered later as HTML or treated as data
//
// The markdown does follow mostly the de-facto rules, but not all.
//
package document

import (
	// "bytes"
	"strconv"
	"strings"

	"github.com/rveen/golib/eventhandler"
	"github.com/rveen/golib/parser"
	"github.com/rveen/ogdl"
)

type Document struct {
	stream *eventhandler.EventHandler
	g      *ogdl.Graph
	ix     int
}

// New parses a markdown+ text string and returnes a Document object.
func New(s string) (*Document, error) {

	doc := &Document{}

	doc.stream = eventhandler.New()
	p := parser.New([]byte(s), doc.stream)

	for block(p) {
	}

	doc.g = doc.stream.Graph()

	return doc, nil
}

// Graph returns the event stream produced by the parser as a Graph.
func (doc *Document) Graph() *ogdl.Graph {
	return doc.g
}

// Html returnes the Document in HTML format
func (doc *Document) Html() string {

	var sb strings.Builder

	for _, n := range doc.g.Out {

		s := n.ThisString()

		switch s {
		case "!pre":
			codeToHtml(n, &sb)
		case "!p":
			textToHtml(n, &sb)
		case "!h":
			headerToHtml(n, &sb)
		case "!ul":
			listToHtml(n, &sb)
		case "!tb":
			tableToHtml(n, &sb)
		}
	}

	return sb.String()
}

// Html returnes the Document in HTML format
func (doc *Document) Html2() string {

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

	eh := eventhandler.New()

	for i, n := range doc.g.Out {

		s := n.ThisString()

		switch s {

		case "!h":
			headerToPart(n, eh, i)
		}
	}

	parts := eh.Graph()
	part := parts.Get(path)
	start := int(part.Get("_start").Int64())
	level := int(part.Get("_level").Int64())
	end := doc.g.Len()

	// End is next header with same 'level'
	for i := start + 1; i < doc.g.Len(); i++ {

		n := doc.g.Out[i]
		if n.ThisString() != "!h" {
			continue
		}
		lv, _ := strconv.Atoi(n.GetAt(0).ThisString())
		if lv <= level {
			end = i
			break
		}
	}

	g := ogdl.New(nil)
	g.Out = doc.g.Out[start:end]

	return &Document{nil, g, 0}
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
