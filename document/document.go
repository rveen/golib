// A markdown processor. Converts markdown to an internal format that may be
// rendered later as HTML or treated as data
//
// The markdown does follow mostly the de-facto rules, but not all.
package document

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/rveen/golib/eventhandler"
	"github.com/rveen/golib/parser"
	"github.com/rveen/ogdl"
)

type Document struct {
	stream  *eventhandler.EventHandler
	g       *ogdl.Graph
	parts   *ogdl.Graph
	ix      int
	Context *ogdl.Graph // Context to solver variables
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
	h  []string
	ix []int // used for numbering the headers
	n  int
}

// Html returnes the Document in HTML format
func (doc *Document) Html() string {

	var sb strings.Builder

	if doc == nil || doc.g == nil {
		return ""
	}

	hh := &headers{}
	numbered := false

	for _, n := range doc.g.Out {

		s := n.ThisString()

		switch s {
		case "!pre":
			codeToHtml(n, &sb)
		case "!p":
			textToHtml(n, &sb)
		case "!h":
			if isNumbered(n.GetAt(1).ThisString()) {
				numbered = true
				n.Out[1].This = n.GetAt(1).ThisString()[3:]
			}
			headerToHtml(n, &sb, hh, "", numbered)
		case "!ul":
			listToHtml(n, &sb, false)
		case "!ol":
			listToHtml(n, &sb, true)
		case "!tb":
			tableToHtml(n, &sb)
		case ".nh":
			numbered = true
		case "!var":
			sb.WriteString(variable(n.String(), doc.Context))
		default:
			sb.WriteString(s)
		}
	}

	return sb.String()
}

func variable(v string, ctx *ogdl.Graph) string {

	if ctx == nil {
		return ""
	}

	e := ogdl.NewExpression(v)
	r, _ := ctx.Eval(e)
	return _string(r)
}

func _string(i interface{}) string {
	if i == nil {
		return ""
	}
	if v, ok := i.([]byte); ok {
		return string(v)
	}
	if v, ok := i.(string); ok {
		return v
	}
	if v, ok := i.(*ogdl.Graph); ok {
		return v.String()
	}
	return fmt.Sprint(i)
}

// Html returnes the Document in HTML format
func (doc *Document) HtmlWithLinks(urlbase string) string {

	var sb strings.Builder

	if doc.g == nil {
		return ""
	}

	hh := &headers{}
	numbered := false

	for _, n := range doc.g.Out {

		s := n.ThisString()

		switch s {
		case "!pre":
			codeToHtml(n, &sb)
		case "!p":
			textToHtml(n, &sb)
		case "!h":
			if isNumbered(n.GetAt(1).ThisString()) {
				numbered = true
				n.Out[1].This = n.GetAt(1).ThisString()[3:]
			}
			headerToHtml(n, &sb, hh, urlbase, numbered)
		case "!ul":
			listToHtml(n, &sb, false)
		case "!ol":
			listToHtml(n, &sb, true)
		case "!tb":
			tableToHtml(n, &sb)
		case ".nh":
			numbered = true
		case "!var":
			sb.WriteString(variable(n.String(), doc.Context))
		}
	}

	return sb.String()
}

// TODO: Re-visit numbering
func isNumbered(text string) bool {
	// return strings.HasPrefix(text, "1. ")
	return false
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
				headerToHtml(n, &sb, nil, "", false)
			}
			header = true
		case "!ul":
			listToHtml(n, &sb, false)
		case "!ol":
			listToHtml(n, &sb, true)
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
		return &Document{}
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

	return &Document{g: g}
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

// Structure returns (only) the headers as OGDL data
func (doc *Document) Structure() *ogdl.Graph {

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
		}
	}

	return eh.Graph()
}

// Compare the structure of two documents.
// All headers of ref need to be in doc. Order is not important, but nesting is.
func (doc *Document) CompareStructure(ref *Document) (bool, string) {

	d := doc.Structure()
	dr := ref.Structure()

	return _compare(d, dr, "")
}

func _compare(d, dr *ogdl.Graph, s string) (bool, string) {

	r := true
	// Check that at this level all headers in dr are in d
	for _, h := range dr.Out {
		hs := h.ThisString()
		hr := d.Get(hs)
		if hr == nil {
			s += hs + " not found\n"
			r = false
		} else {
			r, s = _compare(hr, h, s)
		}
	}

	return r, s
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
