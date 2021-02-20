package parser

import (
	"fmt"
	"testing"

	"github.com/rveen/golib/parser/eventhandler"
)

func TestByte(t *testing.T) {

	p := New([]byte("hola"), eventhandler.New())

	for {
		c, ok := p.Byte()
		fmt.Printf("%c %t\n", c, ok)
		if !ok {
			break
		}
	}
}

func TestRune(t *testing.T) {

	p := New([]byte("höla"), nil)

	c, ok := p.Rune()
	if c != 'h' || ok == false {
		t.Error()
	}
	c, ok = p.Rune()
	if c != 'ö' || ok == false {
		t.Error()
	}
	c, ok = p.Rune()
	if c != 'l' || ok == false {
		t.Error()
	}
	c, ok = p.Rune()
	if c != 'a' || ok == false {
		t.Error()
	}
	c, ok = p.Rune()
	if ok == true {
		t.Error()
	}

	p.UnreadRune('a')
	p.UnreadRune('l')
	p.UnreadRune('ö')
	c, ok = p.Rune()
	if c != 'ö' || ok == false {
		t.Error()
	}

}

func TestWhiteSpace(t *testing.T) {

	p := New([]byte("hola"), nil)
	yes := p.WhiteSpace()
	if yes != false {
		t.Error("A")
	}

	p = New([]byte(" hola"), nil)
	yes = p.WhiteSpace()
	if yes != true {
		t.Error("B")
	}

	c, ok := p.Byte()
	if c != 'h' || ok == false {
		t.Error("C")
	}
}

func TestBreak(t *testing.T) {
	p := New([]byte("\r \n-\r\nx"), nil)
	yes := p.Break()
	if yes == false {
		t.Error()
	}
	c, ok := p.Byte()
	if c != ' ' || ok == false {
		t.Error()
	}

	yes = p.Break()
	if yes == false {
		t.Error()
	}
	c, ok = p.Byte()
	if c != '-' || ok == false {
		t.Error()
	}

	yes = p.Break()
	if yes == false {
		t.Error()
	}
	c, ok = p.Byte()
	if c != 'x' || ok == false {
		t.Error()
	}

	// EoS is also a break
	yes = p.Break()
	if yes == false {
		t.Error()
	}

	yes = p.End()
	if yes == false {
		t.Error()
	}
}

func TestString(t *testing.T) {

	p := New([]byte("test a"), nil)
	s := p.String()
	if s != "test" {
		t.Error()
	}
	s = p.String()
	if s != "" {
		t.Error()
	}
	p.Space()
	s = p.String()
	if s != "a" {
		t.Error()
	}
	s = p.String()
	if s != "" {
		t.Error()
	}
}

func TestQuoted(t *testing.T) {

	p := New([]byte("`test` a"), nil)
	s := p.Quoted(0)
	if s != "test" {
		t.Error()
	}
}

/*

func TestUnreadByte(t *testing.T) {
	r := strings.NewReader("h")

	p := NewParser(r)

	// read letter
	c, err := p.Byte()
	fmt.Printf("%c %v\n", c, err)

	// read EOS
	c, err = p.Byte()
	fmt.Printf("%c %v\n", c, err)

	// unread EOS
	p.UnreadByte()

	// read EOS
	c, err = p.Byte()
	fmt.Printf("%c %v\n", c, err)

	// 2 x unread: read letter
	p.UnreadByte()
	p.UnreadByte()

	// read letter
	c, err = p.Byte()
	fmt.Printf("%c %v\n", c, err)

}

func TestComment1(t *testing.T) {
	r := strings.NewReader("hola")

	p := NewParser(r)

	p.Comment()

	for {
		c, err := p.Byte()
		fmt.Printf("%c %v\n", c, err)
		if err != nil {
			break
		}
	}
}

func TestEnd(t *testing.T) {
	r := strings.NewReader("hola")

	p := NewParser(r)

	p.End()

	for {
		c, err := p.Byte()
		fmt.Printf("%c %v\n", c, err)
		if err != nil {
			break
		}
	}

	fmt.Printf("end2? %v\n", p.End())

}

func TestBreak(t *testing.T) {
	r := strings.NewReader("hola\nmundo")

	p := NewParser(r)

	s, _ := p.String()
	fmt.Printf("[%s]\n", s)

	c := p.PeekByte()
	fmt.Printf("%c\n", c)

	fmt.Printf("break? %v\n", p.Break())

}

func TestPeekByte(t *testing.T) {
	r := strings.NewReader("hola")

	p := NewParser(r)

	c := p.PeekByte()
	fmt.Printf("%c\n", c)

	for {
		c, err := p.Byte()
		fmt.Printf("%c %v\n", c, err)
		if err != nil {
			break
		}
	}

	p.UnreadByte()
	p.UnreadByte()

	for {
		c, err := p.Byte()
		fmt.Printf("%c %v\n", c, err)
		if err != nil {
			break
		}
	}
}

func TestString(t *testing.T) {
	r := strings.NewReader("hola")

	p := NewParser(r)

	s, b := p.String()

	fmt.Println("string", s, b)

	fmt.Printf("end? %v\n", p.End())
}

func TestScalar(t *testing.T) {
	r := strings.NewReader("hola")

	p := NewParser(r)

	s, b := p.Scalar(0)

	fmt.Println("string", s, b)

	fmt.Printf("end? %v\n", p.End())
}

func TestBlockLex(t *testing.T) {

	r := strings.NewReader("\\\n  hola\n  ")
	p := NewParser(r)

	s, b := p.Block(0)

	if s != "hola" || b != true {
		t.Error()
	}
}

func TestQuoted(t *testing.T) {
	r := strings.NewReader("'hola'")

	p := NewParser(r)

	s, b, _ := p.Quoted(0)

	fmt.Println("string", s, b)

	fmt.Printf("end? %v\n", p.End())
}

func TestProd1(t *testing.T) {

	r := strings.NewReader("\t\t\nhola\n")

	p := NewParser(r)

	i, n := p.Space()
	fmt.Printf("%d %d\n", i, n)

	b := p.Break()
	fmt.Println(b)

}

func TestToken1(t *testing.T) {
	r := strings.NewReader("a")

	p := NewParser(r)

	p.Byte()
	p.UnreadByte()

	s, b := p.Token8()
	fmt.Println(s, b)

	s, b = p.Token8()
	fmt.Println(s, b)
}
*/
