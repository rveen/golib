package gosql

import (
	"database/sql"
	"log"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"

	"github.com/rveen/ogdl"
)

type Db struct {
	Db *sql.DB
}

func (db *Db) Open(typ, uri string) error {
	var err error
	if db.Db == nil {
		db.Db, err = sql.Open(typ, uri)
		return err
	}
	return nil
}

func (db *Db) Close() {
	db.Db.Close()
	db.Db = nil
}

func (db *Db) Exec(g *ogdl.Graph) error {

	var err error

	f := g.Node("f").String()
	tb := g.Node("tb").String()

	// fields that are int: ints="a,b,c"
	ints := g.Node("ints").String()
	ii := strings.Split(ints, ",")

	obj := g.Node("obj")

	fields := ""
	values := ""
	for _, f := range obj.Out {

		fname := f.ThisString()

		fields += ", " + fname

		// is the field an 'int' ?
		isInt := false
		for _, i := range ii {
			if i == fname {
				isInt = true
			}
		}

		if isInt {
			values += ", " + f.String()
		} else {
			values += ", '" + f.String() + "'"
		}

	}
	fields = fields[2:]
	values = values[2:]

	switch f {
	case "add":
		fallthrough
	case "replace":
		_, err = db.Db.Exec("replace into " + tb + " (" + fields + ") values (" + values + ")")
		//row := db.db.QueryRow("SELECT LAST_INSERT_ID();")
	case "insert":
		_, err = db.Db.Exec("insert into " + tb + " (" + fields + ") values (" + values + ")")
	case "delete":
	case "del":
	case "update":
	}

	return err
}

func (db *Db) Query(q string) *ogdl.Graph {

	if db.Db == nil {
		return nil
	}

	rows, err := db.Db.Query(q)
	if err != nil {
		log.Println("Error reading rows: " + err.Error())
		return nil
	}
	defer rows.Close()

	r := ogdl.New(nil)

	if err != nil {
		log.Println("Error reading rows: " + err.Error())
		return nil
	}
	defer rows.Close()

	cols, _ := rows.Columns()

	c := r.Add("columns")
	for _, col := range cols {
		c.Add(col)
	}

	values := make([]interface{}, len(cols))

	rr := r.Add("rows")
	for rows.Next() {

		for i := 0; i < len(cols); i++ {
			values[i] = new(string)
		}

		err := rows.Scan(values...)

		if err != nil {
			log.Println("Error reading rows: " + err.Error())
		}

		n := rr.Add("-")
		for i := 0; i < len(cols); i++ {
			s := *(values[i].(*string))
			if s == "" {
				s = "-"
			}
			n.Add(s)
		}
	}

	return r
}

func (db *Db) Query2(q string) *ogdl.Graph {

	if db.Db == nil {
		return nil
	}

	rows, err := db.Db.Query(q)
	if err != nil {
		log.Println("Error reading rows: " + err.Error())
		return nil
	}
	defer rows.Close()

	r := ogdl.New(nil)

	if err != nil {
		log.Println("Error reading rows: " + err.Error())
		return nil
	}
	defer rows.Close()

	cols, _ := rows.Columns()

	c := r.Add("columns")
	for _, col := range cols {
		c.Add(col)
	}

	values := make([]interface{}, len(cols))

	rr := r.Add("rows")
	for rows.Next() {

		for i := 0; i < len(cols); i++ {
			values[i] = new(string)
		}

		err := rows.Scan(values...)

		if err != nil {
			log.Println("Error reading rows: " + err.Error())
		}

		n := rr.Add("-")
		for i := 0; i < len(cols); i++ {
			s := *(values[i].(*string))
			if s == "" {
				s = "-"
			}
			n.Add(cols[i]).Add(s)
		}
	}

	return r
}
