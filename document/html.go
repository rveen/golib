package document

import (
	"log"
	"strconv"
	"strings"

	"github.com/rveen/ogdl"
)

// level
// text
// key
/*
func (doc *Document) headerToHtml(sb *strings.Builder) {

	h, _ := doc.stream.Item(doc.ix)
	doc.ix++
	text, _ := doc.stream.Item(doc.ix)
	doc.ix++
	k, _ := doc.stream.Item(doc.ix)
	doc.ix++

	text = inLine(text)

	// TODO What is faster? many sb.WriteString's, Sprintf or this:
	sb.WriteString("<h" + h + " id=\"" + k + "\">" + text + "</h" + h + ">\n")
}*/

func headerToHtml(n *ogdl.Graph, sb *strings.Builder, hh *headers, urlbase string, numbered bool) {

	if n.Len() < 2 {
		return
	}

	h := n.GetAt(0).ThisString()
	text := n.GetAt(1).ThisString()
	text = inLine(text)

	// If title, no links, no anchors
	if h == "0" {
		sb.WriteString("<div class='title'>" + text + "</div>\n")
		return
	}

	if n.Len() == 2 {
		// TODO What is faster? many sb.WriteString's, Sprintf or this:
		sb.WriteString("<h" + h + "\">" + text + "</h" + h + ">\n")
		return
	}

	// If hh exists, keep header hierarchy. It is used to create the href
	// link to this part of the doc.
	// We do that before creating the html content so that we can include a HREF.
	key := n.GetAt(2).ThisString()

	if hh == nil {
		sb.WriteString("<h" + h + " id=\"" + key + "\">" + text + "</h" + h + ">\n")
		return
	}

	level, _ := strconv.Atoi(h)
	level -= 1

	if level > 9 {
		return
	}
	if len(hh.h) == 0 {
		hh.h = make([]string, 10)
		hh.ix = make([]int, 10)
	}

	hh.n = level
	hh.h[level] = key
	hh.ix[level] += 1

	key = ""
	number := ""
	for i := 0; i <= level; i++ {
		key += hh.h[i] + "/"
		number += strconv.Itoa(hh.ix[i]) + "."
	}

	if numbered {
		text = number + " " + text
	}

	if urlbase == "" {
		sb.WriteString("<h" + h + " id=\"" + key + "\">" + text + "</h" + h + ">\n")
	} else {
		sb.WriteString("<h" + h + " id=\"" + key + "\"><a href='" + urlbase + "/" + key + "'>" + text + "</a></h" + h + ">\n")
	}
}

// TODO nested lists
//
// 1 !li
// 2 1
// 2 "item 1"
// 2 item1
// 1 !li
// 2 1
// 2 "item 2"
// 2 i2
// 1 !li
// 2 2
// 2 "item 2.1"
// 2 item21
// 1 !li
// 2 1
// 2 "item 3"
// 2 item3
//
func (doc *Document) listToHtml(sb *strings.Builder, level int) {

	sb.WriteString("<ul>\n")
	for {

		// !li
		// Stop reading elements if n is 0 or -1
		_, n := doc.stream.Item(doc.ix)
		if n < 1 {
			break
		}
		doc.ix++

		// !li is followed by 3 elements: level, text, key

		// level of item
		// if the level is higher than the current one, a new list needs to
		// be included.
		s, m := doc.stream.Item(doc.ix)
		m, _ = strconv.Atoi(s)

		if m > level {
			doc.ix--
			sb.WriteString("<li>\n")
			doc.listToHtml(sb, m)
			sb.WriteString("</li>\n")
			continue
		} else if m < level {
			doc.ix--
			sb.WriteString("</ul>\n")
			return
		}

		// text of item
		doc.ix++
		text, _ := doc.stream.Item(doc.ix)
		text = inLine(text)
		doc.ix++
		//key is not used
		doc.ix++

		sb.WriteString("<li>" + text + "</li>\n")
	}

	sb.WriteString("</ul>\n")
}

func listToHtml(n *ogdl.Graph, sb *strings.Builder, ordered bool) {

	if ordered {
		sb.WriteString("<ol>\n")
	} else {
		sb.WriteString("<ul>\n")
	}

	level := 1

	for _, li := range n.Out {
		if li.Len() < 2 {
			continue
		}
		lv, _ := strconv.Atoi(li.GetAt(0).ThisString())
		text := li.GetAt(1).ThisString()

		text = inLine(text)

		// Task lists

		tasklist := false
		if len(text) >= 3 && text[0] == '[' && text[2] == ']' {
			switch text[1] {
			case 'x':
				text = "<span class='ballot-no'>☒</span>" + text[3:]
				tasklist = true
			case ' ':
				text = "☐" + text[3:]
				tasklist = true
			case '/':
				text = "<span class='ballot-yes'>☑</span>" + text[3:]
				tasklist = true
			}
		}

		if lv > level {
			sb.WriteString("<ul>\n")
		} else if lv < level {
			sb.WriteString("</ul>\n")
		}

		if tasklist {
			sb.WriteString("<li class='tasklist'>" + text + "</li>\n")
		} else {
			sb.WriteString("<li>" + text + "</li>\n")
		}

		level = lv
	}

	for {
		if ordered {
			sb.WriteString("</ol>\n")
		} else {
			sb.WriteString("</ul>\n")
		}
		level--
		if level < 1 {
			break
		}
	}
}

func (doc *Document) tableToHtml(sb *strings.Builder) {

	hcol := false
	hrow := false

	// What type of table is it
	for i := doc.ix; i < doc.stream.Len(); i++ {
		s, n := doc.stream.Item(i)
		if n < 1 {
			break
		}
		if s == "!hrow" {
			hrow = true
		} else if s == "!hcol" {
			hcol = true
		}
	}

	if hcol && hrow {
		sb.WriteString("<table class='hboth'>\n")
	} else if hcol {
		sb.WriteString("<table class='hcol'>\n")
	} else if hrow {
		sb.WriteString("<table class='hrow'>\n")
	} else {
		sb.WriteString("<table>\n")
	}

	// Go through the rows. If hrow is true, first row is header
	row := 0
	for {
		text, n := doc.stream.Item(doc.ix)
		if n < 1 {
			break
		}
		doc.ix++

		// Each tr has rows at level 2
		if text != "!tr" {
			continue
		}

		sb.WriteString("<tr>\n")

		col := 0
		for {
			text, n = doc.stream.Item(doc.ix)
			if n < 2 {
				break
			}
			doc.ix++
			if n > 2 {
				continue
			}
			if (hrow && row == 0) || (hcol && col == 0) {
				sb.WriteString("<th>")
				sb.WriteString(inLine(text))
				sb.WriteString("</th>")
			} else {
				sb.WriteString("<td>")
				sb.WriteString(inLine(text))
				sb.WriteString("</td>")
			}
			col++
		}

		sb.WriteString("</tr>\n")
		row++
	}

	sb.WriteString("</table>\n")
}

func tableToHtml(n *ogdl.Graph, sb *strings.Builder) {

	hcol := false
	hrow := false

	// What type of table is it
	for _, g := range n.Out {
		s := g.ThisString()
		if s == "!hrow" {
			hrow = true
		} else if s == "!hcol" {
			hcol = true
		}
	}

	if hcol && hrow {
		sb.WriteString("<table class='hboth'>\n")
	} else if hcol {
		sb.WriteString("<table class='hcol'>\n")
	} else if hrow {
		sb.WriteString("<table class='hrow'>\n")
	} else {
		sb.WriteString("<table>\n")
	}

	// Go through the rows. If hrow is true, first row is header
	row := 0
	for _, g := range n.Out {
		text := g.ThisString()

		// Each tr has rows at level 2
		if text != "!tr" {
			continue
		}

		sb.WriteString("<tr>\n")

		col := 0
		for _, r := range g.Out {
			text := r.ThisString()

			if (hrow && row == 0) || (hcol && col == 0) {
				sb.WriteString("<th>")
				sb.WriteString(inLine(text))
				sb.WriteString("</th>")
			} else {
				sb.WriteString("<td>")
				sb.WriteString(inLine(text))
				sb.WriteString("</td>")
			}
			col++
		}

		sb.WriteString("</tr>\n")
		row++
	}

	sb.WriteString("</table>\n")
}

func (doc *Document) textToHtml(sb *strings.Builder) {

	sb.WriteString("<p>")
	for {
		t, n := doc.stream.Item(doc.ix)
		if n < 1 {
			break
		}
		doc.ix++

		sb.WriteString(inLine(t))
	}

	sb.WriteString("</p>\n")
}

func textToHtml(n *ogdl.Graph, sb *strings.Builder) {

	sb.WriteString("<p>")
	for _, g := range n.Out {
		sb.WriteString(inLine(g.ThisString()))
	}
	sb.WriteString("</p>\n")
}

func codeToHtml(n *ogdl.Graph, sb *strings.Builder) {

	// First node is class

	for n, g := range n.Out {
		if n == 0 {
			sb.WriteString("<pre class='")
			sb.WriteString(g.ThisString())
			sb.WriteString("'>\n")
			continue
		}
		sb.WriteString(g.ThisString())
		sb.WriteByte('\n')
	}
	sb.WriteString("</pre>\n")
}

func (doc *Document) codeToHtml(sb *strings.Builder) {

	doc.ix++

	s, _ := doc.stream.Item(doc.ix)
	log.Println("codeToHtml", s)

	sb.WriteString("<pre>")
	for {
		t, n := doc.stream.Item(doc.ix)
		if n < 1 {
			break
		}
		doc.ix++

		sb.WriteString(t)
		sb.WriteByte('\n')
	}

	sb.WriteString("</pre>\n")
}
