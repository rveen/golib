// Copyright 2012-2018, Rolf Veen and contributors.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package parser

import (
	"github.com/rveen/ogdl"
)

// Ast reorganizes the expression graph in the form of an abstract syntax tree.

func Ast(g *ogdl.Graph) {
	ast(g)
	clean(g)
}

func clean(g *ogdl.Graph) {
	for i := 0; i < len(g.Out); i++ {
		node := g.Out[i]
		if node.ThisString() == "!g" {
			if node.Len() == 1 {
				g.Out[i] = node.Out[0]
			}
		}
		clean(node)
	}
}

func ast(g *ogdl.Graph) {

	for _, node := range g.Out {
		ast(node)
	}

	if g.Len() < 3 {
		return
	}

	var e1, e2 *ogdl.Graph

	for j := 5; j >= 0; j-- {

		n := len(g.Out)

		for i := 0; i < n; i++ {
			node := g.Out[i]

			if precedence(node.ThisString()) == j {
				e1 = g.Out[i-1]
				e2 = g.Out[i+1]
				g.Out = append(g.Out[:i-1], g.Out[i+1:]...)
				g.Out[i-1] = node
				node.Add(e1)
				node.Add(e2)
				n = len(g.Out)
			}
		}
	}
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
