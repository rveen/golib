package main

import (
	"fmt"
	"regexp"
)

var (
	anchor = regexp.MustCompile(`{#\w+}`)
	typ    = regexp.MustCompile(`{!\w+}`)
	link   = regexp.MustCompile(`\[([^\]]+)\]\(([^\)]+)\)`)
	link2  = regexp.MustCompile(`\[\]\(([^\)]+)\)`)
)

func main() {

	s := "[implements](a/b)"

	/*s = link.ReplaceAllString(s, "<a href=\"$2\">$1</a>")
	s = link2.ReplaceAllString(s, "<a href=\"$1\">$1</a>")

	fmt.Println(s)*/

	ss := link.FindStringSubmatch(s)
	fmt.Printf("%q\n", ss)

	if len(ss) == 3 {
		switch ss[1] {
		case "implements":
			fmt.Printf("<a href='%s'><button class='btn btn-sm btn-info'>📄⚙️</button></a>", ss[2])
		default:
			fmt.Printf("<a href='%s'>%s</a>", ss[2], ss[1])
		}
	}
	fmt.Println("")
}
