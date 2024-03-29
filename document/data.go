package document

import (
	"strconv"
	"strings"

	"github.com/rveen/golib/eventhandler"
	"github.com/rveen/ogdl"
)

func (doc *Document) listToData(eh *eventhandler.EventHandler) {

}

func (doc *Document) textToData(eh *eventhandler.EventHandler) {

	var sb strings.Builder

	for {
		t, n := doc.stream.Item(doc.ix)
		if n < 1 {
			break
		}
		doc.ix++

		sb.WriteString(inLine(t))
		sb.WriteByte('\n')
	}

	eh.Add("_text")
	eh.Inc()
	eh.Add(sb.String())
	eh.Dec()
}

func (doc *Document) headerToData(eh *eventhandler.EventHandler) {

	level, _ := doc.stream.Item(doc.ix)
	doc.ix++
	//text, _ := doc.stream.Item(doc.ix)
	doc.ix++
	key, _ := doc.stream.Item(doc.ix)
	doc.ix++
	typ, _ := doc.stream.Item(doc.ix)
	doc.ix++

	if strings.HasPrefix(key, "#") {
		key = key[1:]
	}

	if strings.HasPrefix(typ, "!") {
		typ = typ[1:]
	}

	n, _ := strconv.Atoi(level)
	eh.SetLevel(n - 1)
	eh.Add(key)
	eh.Inc()
	if typ != "" {
		eh.Add("_type")
		eh.Inc()
		eh.Add(typ)
		eh.Dec()
	}
}

func headerToPart(n *ogdl.Graph, eh *eventhandler.EventHandler, ix int) {
	if n.Len() < 3 {
		return
	}

	level := n.GetAt(0).ThisString()
	key := n.GetAt(2).ThisString()

	if strings.HasPrefix(key, "#") {
		key = key[1:]
	}

	lv, _ := strconv.Atoi(level)
	eh.SetLevel(lv - 1)
	eh.Add(key)
	eh.Inc()
	eh.Add("_start")
	eh.Inc()
	eh.Add(strconv.Itoa(ix))
	eh.Dec()
	eh.Add("_level")
	eh.Inc()
	eh.Add(strconv.Itoa(lv))
	eh.Dec()
}

//  | a | b | c |
//  |---|---|---|
//  | 1 | 2 | 3 |
//  | 8 | 9 | 0 |
//
//  a
//    1
//    8

func (doc *Document) tableToData(eh *eventhandler.EventHandler) {

	hcol := false
	hrow := false

	nrows := 0
	ncols := 0

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
		if s == "!tr" {
			nrows++
		}
	}

	if !hcol && !hrow {
		return
	}

	table := [][]string{}

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

		table = append(table, []string{})

		col := 0
		for {
			text, n = doc.stream.Item(doc.ix)
			if n < 2 {
				break
			}

			if ((hrow && row == 0) || (hcol && col == 0)) && n == 2 {
				doc.ix++
				text, n = doc.stream.Item(doc.ix)
			} else if n == 3 {
				doc.ix++
				continue
			}
			table[row] = append(table[row], text)
			col++
			doc.ix++
		}
		if row == 0 {
			ncols = col
		}
		row++
	}

	if hrow && !hcol {
		// Header is the first row, and makes up the keys
		for i := 0; i < ncols; i++ {
			eh.Add(table[0][i])
			eh.Inc()
			for j := 1; j < nrows; j++ {
				eh.Add(table[j][i])
			}
			eh.Dec()
		}
	} else if !hrow && hcol {
		// Header is the first columns, and makes up the keys
		for i := 0; i < nrows; i++ {
			eh.Add(table[i][0])
			eh.Inc()
			for j := 1; j < ncols; j++ {
				if len(table[i]) <= j {
					break
				}
				eh.Add(table[i][j])
			}
			eh.Dec()
		}
	} else {
		// Keys are in first row and first column
		// row 0, col 0 is main key
		eh.Add(table[0][0])
		eh.Inc()

		for i := 1; i < nrows; i++ {
			eh.Add(table[i][0])
			eh.Inc()
			for j := 1; j < ncols; j++ {
				eh.Add(table[0][j])
				if len(table[i]) <= j {
					continue
				}
				eh.Inc()
				eh.Add(table[i][j])
				eh.Dec()
			}
			eh.Dec()
		}

		eh.Dec()
	}
}
