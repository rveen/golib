// Copyright 2012-2021, Rolf Veen and contributors.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// This eventhandler implementation is optional for the parser package, in
// order to avoid a mutual dependency with ogdl.
package eventhandler

import "github.com/rveen/ogdl"

// SimpleEventHandler receives events and produces a tree.
type SimpleEventHandler struct {
	current int      // Current level
	max     int      // Max level
	levels  []int    // Level of each item
	items   []string // Items
}

func New() *SimpleEventHandler {
	return &SimpleEventHandler{}
}

func (e *SimpleEventHandler) Len() int {
	return len(e.levels)
}

func (e *SimpleEventHandler) Item(i int) (string, int) {
	if i < 0 || i >= len(e.levels) {
		return "", -1
	}
	return e.items[i], e.levels[i]
}

// Add creates a string node at the current level.
func (e *SimpleEventHandler) Add(s string) {
	e.items = append(e.items, s)
	e.levels = append(e.levels, e.current)
}

// AddAt creates a string node at the specified level.
func (e *SimpleEventHandler) AddAt(s string, lv int) {
	e.items = append(e.items, s)
	e.levels = append(e.levels, lv)
	if e.max < lv {
		e.max = lv
	}
}

// Delete removes the last node added
func (e *SimpleEventHandler) Delete() {
	e.items = e.items[0 : len(e.items)-1]
	e.levels = e.levels[0 : len(e.levels)-1]
}

// Level returns the current level
func (e *SimpleEventHandler) Level() int {
	return e.current
}

// SetLevel sets the current level
func (e *SimpleEventHandler) SetLevel(l int) {
	e.current = l
	if e.max < l {
		e.max = l
	}
}

// Inc increments the current level by 1.
func (e *SimpleEventHandler) Inc() {
	e.current++
	if e.max < e.current {
		e.max = e.current
	}
}

// Dec decrements the current level by 1.
func (e *SimpleEventHandler) Dec() {
	if e.current > 0 {
		e.current--
	}
}

// Tree returns the Graph object built from
// the events sent to this event handler.
//
func (e *SimpleEventHandler) Graph() *ogdl.Graph {

	g := make([]*ogdl.Graph, e.max+2)
	g[0] = ogdl.New(nil)

	for i := 0; i < len(e.items); i++ {
		lv := e.levels[i] + 1
		item := e.items[i]

		n := ogdl.New(item)
		g[lv] = n
		g[lv-1].Add(n)
	}

	return g[0]
}
