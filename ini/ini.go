package ini

import (
	"bufio"
	"os"
	"strings"

	"github.com/rveen/ogdl"
)

func Load(file string) (*ogdl.Graph, error) {

	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	g := ogdl.New(nil)

	scanner := bufio.NewScanner(f)

	section := ""
	og := ""
	gs := g

	raw := false
	rawTest := false

	for scanner.Scan() {
		line := scanner.Text()

		// Empty line
		if line == "" {
			continue
		}

		// Comment
		if strings.HasPrefix(line, "# ") || line[0] == ';' {
			continue
		}

		if line[0] == '[' {
			s := strings.TrimSpace(line[1:])
			if s[len(s)-1] == ']' {
				s = s[0 : len(s)-1]
			}

			section = s
			gs = g.Add(section)
			gs = gs
			rawTest = true

			if og != "" {
				n := ogdl.FromString(og)
				gs.AddNodes(n)
				og = ""
			}

			continue
		}

		i := strings.Index(line, " # ")
		if i > -1 {
			// TODO trime space right
			line = line[0:i]
		}

		// line has 'var = value' format ?
		if rawTest {
			raw = !strings.Contains(line, " = ")
			rawTest = false
		}

		if raw {
			og += line + "\n"
		} else {
			ff := strings.Split(line, " = ")
			if len(ff) == 2 {
				if strings.HasPrefix(ff[1], "\"") {
					ff[1] = ff[1][1:]
				}
				if strings.HasSuffix(ff[1], "\"") {
					ff[1] = ff[1][0 : len(ff[1])-1]
				}
				gs.Add(ff[0]).Add(ff[1])
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return g, nil
}
