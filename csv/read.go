package csv

import (
	"bufio"
	"os"
	"strings"
)

func contains(tags []string, tag string) bool {

	for _, field := range tags {
		if field == tag {
			return true
		}
	}
	return false
}

func Split(s string) []string {
	res := []string{}

	var qstart int
	start := 0
	inString := false
	clean := false

	for i := 0; i < len(s); i++ {
		if s[i] == ',' && !inString {

			// Remove quotes
			if clean {
				res = append(res, s[qstart+1:i-1])
				clean = false
			} else {
				res = append(res, s[start:i])
			}
			start = i + 1
		} else if s[i] == '"' {
			if !inString {
				qstart = i
				inString = true
			} else if i > 0 && s[i-1] != '\\' {
				inString = false
				clean = true
			}
		}
	}
	return append(res, s[start:])
}

// Read a CVS file into and array of maps
func Read(file string) ([]map[string]string, error) {

	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var data [][]string

	scanner := bufio.NewScanner(f)
	// note: resize scanner's capacity if lines are over 64K
	for scanner.Scan() {
		line := scanner.Text()
		if line[0] != '#' {
			data = append(data, Split(line))
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// The first line contains the field names or keys
	keys := data[0]
	for j := 0; j < len(keys); j++ {
		// Clean up
		keys[j] = strings.TrimSpace(keys[j])
	}
	var rr []map[string]string

	for i := 1; i < len(data); i++ {

		l := data[i]
		r := make(map[string]string)

		for j := 0; j < len(l); j++ {
			// Clean up (remove space and convert to lower case)
			// value := strings.ToLower(strings.TrimSpace(l[j]))
			value := strings.TrimSpace(l[j])
			if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				value = value[1 : len(value)-1]
			}
			// Add to map
			if len(value) != 0 {
				r[keys[j]] = value
			}
		}
		rr = append(rr, r)
	}

	return rr, nil
}

// Read a CVS file into and array of maps
func ReadString(in string) ([]map[string]string, error) {

	var data [][]string

	scanner := bufio.NewScanner(strings.NewReader(in))
	// note: resize scanner's capacity if lines are over 64K
	for scanner.Scan() {
		line := scanner.Text()
		if line[0] != '#' {
			data = append(data, Split(line))
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// The first line contains the field names or keys
	keys := data[0]
	for j := 0; j < len(keys); j++ {
		// Clean up
		keys[j] = strings.TrimSpace(keys[j])
	}
	var rr []map[string]string

	for i := 1; i < len(data); i++ {

		l := data[i]
		r := make(map[string]string)

		for j := 0; j < len(l); j++ {
			// Clean up (remove space and convert to lower case)
			// value := strings.ToLower(strings.TrimSpace(l[j]))
			value := strings.TrimSpace(l[j])
			if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
				value = value[1 : len(value)-1]
			}
			// Add to map
			if len(value) != 0 {
				r[keys[j]] = value
			}
		}
		rr = append(rr, r)
	}

	return rr, nil
}
