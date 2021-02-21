package document

import (
	"strconv"
	"strings"

	"github.com/rveen/golib/parser/eventhandler"
)

func (doc *Document) listToData(eh *eventhandler.SimpleEventHandler) {

	eh.Add("-")
	eh.Inc()

	eh.Dec()
}

func (doc *Document) textToData(eh *eventhandler.SimpleEventHandler) {

	var sb strings.Builder

	for {
		t, n := doc.stream.Item(doc.ix)
		if n < 1 {
			break
		}
		doc.ix++

		sb.WriteString(InLine(t))
		sb.WriteByte('\n')
	}

	eh.Add("_text")
	eh.Inc()
	eh.Add(sb.String())
	eh.Dec()
}

func (doc *Document) headerToData(eh *eventhandler.SimpleEventHandler) {

	level, _ := doc.stream.Item(doc.ix)
	doc.ix++
	//text, _ := doc.stream.Item(doc.ix)
	doc.ix++
	key, _ := doc.stream.Item(doc.ix)
	doc.ix++

	n, _ := strconv.Atoi(level)
	eh.SetLevel(n - 1)
	eh.Add(key)
	eh.Inc()
}

//  | a | b | c |
//  |---|---|---|
//  | 1 | 2 | 3 |
//  | 8 | 9 | 0 |
//
//  a
//    1
//    8

func (doc *Document) tableToData(eh *eventhandler.SimpleEventHandler) {

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

	if !hcol && !hrow {
		return
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

		col := 0
		for {
			text, n = doc.stream.Item(doc.ix)
			if n < 2 {
				break
			}
			doc.ix++

			if n == 3 {
				eh.Add(text)
				eh.Inc()
				continue
			}

			if (hrow && row == 0) || (hcol && col == 0) {

			} else {

			}
			col++

			eh.Dec()
		}

		row++
	}

}
