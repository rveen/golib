package document

import (
	"strconv"
	"strings"
)

// level
// text
// key
func (doc *Document) headerToHtml(sb *strings.Builder) {

	h, _ := doc.stream.Item(doc.ix)
	doc.ix++
	text, _ := doc.stream.Item(doc.ix)
	doc.ix++
	k, _ := doc.stream.Item(doc.ix)
	doc.ix++

	text = InLine(text)

	// TODO What is faster? many sb.WriteString's, Sprintf or this:
	sb.WriteString("<a name=\"" + k + "\"/>\n")
	sb.WriteString("<h" + h + ">" + text + "</h" + h + ">\n")
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
		text = InLine(text)
		doc.ix++
		//key is not used
		doc.ix++

		sb.WriteString("<li>" + text + "</li>\n")
	}

	sb.WriteString("</ul>\n")
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
				sb.WriteString(InLine(text))
				sb.WriteString("</th>")
			} else {
				sb.WriteString("<td>")
				sb.WriteString(InLine(text))
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

		sb.WriteString(InLine(t))
	}

	sb.WriteString("</p>\n")
}

func (doc *Document) codeToHtml(sb *strings.Builder) {

	doc.ix++

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
