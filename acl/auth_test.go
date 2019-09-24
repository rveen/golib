package acl

import (
	"fmt"
	"testing"
)

func TestAcl(t *testing.T) {

	acl, _ := New("")
	acl.AddRule("*", "*", "*", false)
	acl.AddRule("fuelcell", "/prj/*", "*", true)
	acl.AddRule("automotive", "/secret/*", "*", true)

	acl.AddGroup("rolf", "fuelcell")
	acl.AddGroup("fuelcell", "automotive")

	b := acl.Enforce("rolf", "/prj/ssb", "")
	fmt.Println(b)

	b = acl.Enforce("fuelcell", "/prj/ssb", "")
	fmt.Println(b)

	b = acl.Enforce("automotive", "/secret/", "")
	fmt.Println(b)
}

func TestInGroup(t *testing.T) {
	acl, _ := New("")

	acl.AddGroup("rolf", "fuelcell")
	acl.AddGroup("fuelcell", "automotive")

	println(acl.InGroup("rolf", "fuelcell"))
	println(acl.InGroup("rolf", "automotive"))
	println(acl.InGroup("rolf", "automotives"))
}

func TestConfFile(t *testing.T) {
	acl, err := New("test.conf")

	if err != nil {
		println(err)
		return
	}

	println("test.conf:", len(acl.groups), len(acl.rules))
}
