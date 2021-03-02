// Copyright 2012-2018, Rolf Veen and contributors.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parser

import (
	"errors"
)

// Nodes containing these strings are special
const (
	TypeExpression = "!e"
	TypePath       = "!p"
	TypeVariable   = "!v"
	TypeSelector   = "!s"
	TypeIndex      = "!i"
	TypeGroup      = "!g"
	TypeArguments  = "!a"
	TypeTemplate   = "!t"
	TypeString     = "!string"

	TypeIf    = "!if"
	TypeEnd   = "!end"
	TypeElse  = "!else"
	TypeFor   = "!for"
	TypeBreak = "!break"
)

var (
	// ErrInvalidUnread reports an unsuccessful UnreadByte or UnreadRune
	ErrInvalidUnread = errors.New("invalid use of UnreadByte or UnreadRune")

	// ErrEOS indicates the end of the stream
	ErrEOS = errors.New("EOS")

	// ErrSpaceNotUniform indicates mixed use of spaces and tabs for indentation
	ErrSpaceNotUniform = errors.New("space has both tabs and spaces")

	// ErrUnterminatedQuotedString is obvious.
	ErrUnterminatedQuotedString = errors.New("quoted string not terminated")

	ErrNotANumber       = errors.New("not a number")
	ErrNotFound         = errors.New("not found")
	ErrIncompatibleType = errors.New("incompatible type")
	ErrNilReceiver      = errors.New("nil function receiver")
	ErrInvalidIndex     = errors.New("invalid index")
	ErrFunctionNoGraph  = errors.New("functions doesn't return *Graph")
	ErrInvalidArgs      = errors.New("invalid arguments or nil receiver")
)

// Parser exposes Ix and Buf making it easier to use it outside of this package.
type Parser struct {
	Ix  int
	Buf []byte
	ev  eventHandler
}

type eventHandler interface {
	Add(string)
	Delete()
	SetLevel(int)
	Inc()
	Dec()
	Level() int
}

// New creates a Parser. An event handler needs to be supplied if productions
// that emit events are used.
func New(buf []byte, eh eventHandler) *Parser {
	return &Parser{0, buf, eh}
}

// Some usefull functions to extended the Parser and use it in other places

// Emit outputs a string event at the current level. This will show up in the graph
func (p *Parser) Emit(s string) {
	p.ev.Add(s)
}

// Inc increases the event handler level by one
func (p *Parser) Inc() {
	p.ev.Inc()
}

// Dec decreses the event handler level by one
func (p *Parser) Dec() {
	p.ev.Dec()
}

func (p *Parser) Level() int {
	return p.ev.Level()
}

func (p *Parser) SetLevel(i int) {
	p.ev.SetLevel(i)
}

func (p *Parser) Reset() {
	p.Ix = 0
}
