package rest

// fromJSON is FromJSON from github.com/rveen/ogdl v1, on ogdl v2 nodes.

import (
	"bytes"
	"encoding/json"
	"math"

	"github.com/rveen/ogdl/v2"
)

func fromJSON(buf []byte) (*ogdl.Node, error) {

	var v interface{}

	// Use Decoder, since we want to treat integers as integers.
	dec := json.NewDecoder(bytes.NewReader(buf))
	dec.UseNumber()

	// We expect here only one map or list
	err := dec.Decode(&v)
	if err != nil {
		return nil, err
	}

	return toGraph(v).GetAt(0), nil
}

func toGraph(v interface{}) *ogdl.Node {

	g := ogdl.New("_")

	switch v.(type) {

	case []interface{}:
		// n := g.Add("_")
		for _, i := range v.([]interface{}) {
			g.Add(toGraph(i))
		}
	case map[string]interface{}:
		n := g.Add("_")
		for k, i := range v.(map[string]interface{}) {
			n.Add(k).AddNodes(toGraph(i))
		}
	case json.Number:
		// try first to decode the number as an integer.
		i, err := v.(json.Number).Int64()
		if err != nil {
			f, err := v.(json.Number).Float64()
			if err != nil {
				f = math.NaN()
			}
			g.Add(f)
		} else {
			g.Add(i)
		}

	default:
		g.Add(v)
	}
	return g
}
