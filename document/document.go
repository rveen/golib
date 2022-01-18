// A markdown processor. Converts markdown to an internal format that may be
// rendered later as HTML or treated as data
//
// The markdown does follow mostly the de-facto rules, but not all.
//
package document

import (
	"strconv"
	"strings"

	"github.com/rveen/golib/eventhandler"
	"github.com/rveen/golib/parser"
	"github.com/rveen/ogdl"
)

type Document struct {
	stream *eventhandler.EventHandler
	g      *ogdl.Graph
	parts  *ogdl.Graph
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

type headers struct {
	h []string
	n int
}

// Html returnes the Document in HTML format
func (doc *Document) Html() string {

	var sb strings.Builder

	if doc.g == nil {
		return ""
	}

	for _, n := range doc.g.Out {

		s := n.ThisString()

		switch s {
		case "!pre":
			codeToHtml(n, &sb)
		case "!p":
			textToHtml(n, &sb)
		case "!h":
			headerToHtml(n, &sb, nil, "")
		case "!ul":
			listToHtml(n, &sb)
		case "!tb":
			tableToHtml(n, &sb)
		}
	}

	return sb.String()
}

// Html returnes the Document in HTML format
func (doc *Document) HtmlWithLinks(urlbase string) string {

	var sb strings.Builder

	if doc.g == nil {
		return ""
	}

	hh := &headers{}

	for _, n := range doc.g.Out {

		s := n.ThisString()

		switch s {
		case "!pre":
			codeToHtml(n, &sb)
		case "!p":
			textToHtml(n, &sb)
		case "!h":
			headerToHtml(n, &sb, hh, urlbase)
		case "!ul":
			listToHtml(n, &sb)
		case "!tb":
			tableToHtml(n, &sb)
		}
	}

	return sb.String()
}

// Html returnes the Document in HTML format, but skip the first header
func (doc *Document) HtmlNoHeader() string {

	var sb strings.Builder
	header := false

	for _, n := range doc.g.Out {

		s := n.ThisString()

		switch s {
		case "!pre":
			codeToHtml(n, &sb)
		case "!p":
			textToHtml(n, &sb)
		case "!h":
			if header {
				headerToHtml(n, &sb, nil, "")
			}
			header = true
		case "!ul":
			listToHtml(n, &sb)
		case "!tb":
			tableToHtml(n, &sb)
		}
	}

	return sb.String()
}

// Part returns the part of the document indicated by the given path.
func (doc *Document) Part(path string) *Document {

	// This assumes that the Doc is a constant (generated once)
	if doc.parts == nil {
		eh := eventhandler.New()

		for i, n := range doc.g.Out {
			s := n.ThisString()
			switch s {
			case "!h":
				headerToPart(n, eh, i)
			}
		}
		doc.parts = eh.Graph()
	}

	part := doc.parts.Get(path)
	if part == nil || part.Len() == 0 {
		return &Document{nil, nil, nil, 0}
	}

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

	return &Document{nil, g, nil, 0}
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

func (doc *Document) Raw() *ogdl.Graph {
	return doc.g
}

// Return the first paragraph of this document
func (doc *Document) Para1() string {

	for _, n := range doc.g.Out {

		s := n.ThisString()
		if s == "!p" {
			return n.String()
		}
	}

	return ""
}
