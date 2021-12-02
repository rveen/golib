// Copyright 2012-2018, Rolf Veen and contributors.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parser

import "github.com/rveen/ogdl"

// Ast reorganizes the expression graph in the form of an abstract syntax tree.
func Ast(g *ogdl.Graph) *ogdl.Graph {

	if g.Len() < 3 {
		return nil
	}

	r := ogdl.New(nil)

	for j := 5; j >= 0; j-- {

		for i := 0; i < len(g.Out); i++ {
			node := g.Out[i]
			if precedence(node.ThisString()) == j {
				n := r.Add(node.This)
				n.Out = append(n.Out, g.Out[i-1])
				n.Out = append(n.Out, g.Out[i+1])
			}
		}
	}

	return r
}

// Precedence is same as in Go, except for the missing operators (| << >> & ^ &^)
//
// Assignment operators are given the lowest precedence.
func precedence(s string) int {

	switch s {

	case "+":
		return 4
	case "-":
		return 4
	case "*":
		return 5
	case "/":
		return 5
	case "%":
		return 5

	case "=":
		return 0
	case "+=":
		return 0
	case "-=":
		return 0
	case "*=":
		return 0
	case "/=":
		return 0
	case "%=":
		return 0

	case "==":
		return 3
	case "!=":
		return 3
	case ">=":
		return 3
	case "<=":
		return 3
	case ">":
		return 3
	case "<":
		return 3

	case "||":
		return 1
	case "&&":
		return 2
	}

	return -1
}
