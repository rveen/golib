// Acl implements a simple role based access control
//
// Configuration file format:
//
// # A comment
// [rules]
// * * * -  # deny all to all, to start with
// purchasing /purchasing/* * +
//
// [groups]
// name group1 group2 ...
//
// All rules are checked in order.
//
package acl

import (
	"bufio"
	"log"
	"os"
	"strings"
)

// ACL contains rules and groups for implementing
// an access control list
type ACL struct {
	file   string
	rules  []Rule
	groups map[string][]string
}

type Rule struct {
	Subject   string
	Object    string
	Operation string
	Polarity  bool
	Prefix    bool
}

// New creates a new Acl object, either from a configuration
// file, or empty if an empty string is given as argument.
func New(filename string) (*ACL, error) {

	if filename == "" {
		return &ACL{}, nil
	}

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	section := ""
	acl := &ACL{}
	acl.file = filename

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {

		line := scanner.Text()

		switch line {
		case "[rules]":
			section = "r"

		case "[groups]":
			section = "g"

		default:

			tk := strings.Fields(line)

			if len(tk) == 0 || tk[0] == "#" {
				continue
			}

			if section == "r" {
				// sub obj op [pol]
				switch len(tk) {
				case 3:
					acl.AddRule(tk[0], tk[1], tk[2], true)
				case 4:
					acl.AddRule(tk[0], tk[1], tk[2], tk[3] == "+")
				default:
					println("rule: error in number of attributes", len(tk))
				}

			} else if section == "g" {
				if len(tk) > 1 {
					for i := 1; i < len(tk); i++ {
						acl.AddGroup(tk[i], tk[0])
					}
				}
			}
		}

	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	log.Println(filename, "loaded", len(acl.rules), "rules,", len(acl.groups), "groups")

	return acl, nil
}

// Reload is meant to be called by a file watcher that monitors the ACL definition
// file for changes.
func (acl *ACL) Reload() error {

	file, err := os.Open(acl.file)
	if err != nil {
		return err
	}
	defer file.Close()

	section := ""
	acl2 := &ACL{}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {

		line := scanner.Text()

		switch line {
		case "[rules]":
			section = "r"

		case "[groups]":
			section = "g"

		default:

			tk := strings.Fields(line)

			if len(tk) == 0 || tk[0] == "#" {
				continue
			}

			if section == "r" {
				// sub obj op [pol]
				switch len(tk) {
				case 3:
					acl2.AddRule(tk[0], tk[1], tk[2], true)
				case 4:
					acl2.AddRule(tk[0], tk[1], tk[2], tk[3] == "+")
				default:
					println("rule: error in number of attributes", len(tk))
				}

			} else if section == "g" {
				if len(tk) > 1 {
					for i := 1; i < len(tk); i++ {
						acl2.AddGroup(tk[i], tk[0])
					}
				}
			}
		}

	}

	if err := scanner.Err(); err != nil {
		return err
	}

	// TODO: mutex
	acl.groups = acl2.groups
	acl.rules = acl2.rules

	return nil
}

func (acl *ACL) AddRule(sub, obj, op string, pol bool) {
	r := Rule{sub, obj, op, pol, false}

	if strings.HasSuffix(obj, "*") {
		r.Object = obj[0 : len(obj)-1]
		r.Prefix = true
	}

	println("AddRule", r.Subject, r.Object, r.Operation, r.Polarity, r.Prefix)

	acl.rules = append(acl.rules, r)
}

func (acl *ACL) AddGroup(sub, group string) {
	g := acl.groups
	if g == nil {
		acl.groups = make(map[string][]string)
	}

	acl.groups[sub] = append(acl.groups[sub], group)
	// TODO: fill in multiple levels
	println("addGroup", group, sub, len(acl.groups[sub]))
}

// Enforce checks the ACL for a specific resource and user, and returns true
// if access is granted.
func (acl *ACL) Enforce(sub, obj, op string) bool {

	result := true

	for _, r := range acl.rules {

		// operation in rule ?
		if op != r.Operation && r.Operation != "*" {
			continue
		}

		// Object in rule ?
		if len(obj) < len(r.Object) {
			continue
		}

		if strings.HasPrefix(obj, r.Object) {
			if len(obj) > len(r.Object) && obj[len(r.Object)] != '/' {
				// Check that this works for any encoding
				continue
			}
		} else {
			continue
		}

		// Subject in rule
		if r.Subject != "*" && sub != r.Subject {
			if !acl.InGroup(sub, r.Subject) {
				continue
			}
		}

		// Rule applies !
		result = r.Polarity

	}

	return result
}

// InGroup checks whether the first argument is part of the group given
// as second argument
func (acl *ACL) InGroup(sub, group string) bool {
	gg := acl.groups[sub]

	if gg == nil {
		return false
	}

	for _, g := range gg {
		if g == group {
			return true
		}
		if acl.InGroup(g, group) {
			return true
		}
	}

	return false
}
