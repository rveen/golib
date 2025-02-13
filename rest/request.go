package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/rveen/ogdl"
)

type Rest struct {
	user   string
	passwd string
	Token  string
}

// Set the connection parameters, in this order: server URL, user, password.
// If no parameters are given, the hardcoded set is used.
// The usual server URL has the form https://server:1443.
func New(user, passwd string) *Rest {
	return &Rest{user, passwd, ""}
}

// Rest GET request
func (rest *Rest) Get(url string) (*ogdl.Graph, error) {

	body := []byte(`{}`)

	r, err := http.NewRequest("GET", url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	r.Header.Add("accept", "application/json")
	r.Header.Add("Content-Type", "application/json")

	fmt.Println(rest.Token)

	if rest.Token != "" {
		r.Header.Add("Authorization", rest.Token)
	}

	if rest.user != "" {
		r.SetBasicAuth(rest.user, rest.passwd)
	}

	client := &http.Client{}
	res, err := client.Do(r)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()

	body, err = ioutil.ReadAll(res.Body)

	var result map[string]any

	err = json.Unmarshal(body, &result)
	if err != nil {
		log.Printf("%s\n", string(body))
		return nil, err
	}

	g, err := ogdl.FromJSON(body)

	return g, err
}

// Windchill POST request
func (rest *Rest) Post(url, token string) error {

	body := []byte(`{}`)

	r, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	r.SetBasicAuth(rest.user, rest.passwd)
	r.Header.Add("accept", "application/json")
	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("CSRF_NONCE", token)

	client := &http.Client{}
	res, err := client.Do(r)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	body, err = ioutil.ReadAll(res.Body)

	var result map[string]any

	err = json.Unmarshal(body, &result)
	if err != nil {
		return err
	}

	g, err := ogdl.FromJSON(body)

	fmt.Println(g.Text())

	return err
}
