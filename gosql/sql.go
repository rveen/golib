package gosql

import (
	"database/sql"
	"log"
	"strings"

	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"

	"github.com/rveen/ogdl"
)

type Db struct {
	Db *sql.DB
}

func (db *Db) Open(typ, uri string) error {
	var err error

	if db.Db != nil {
		db.Close()
	}

	db.Db, err = sql.Open(typ, uri)
	return err
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
	upd := ""
	for _, f := range obj.Out {

		fname := f.ThisString()

		fields += ", `" + fname + "`"
		upd += ", `" + fname + "` = "

		// is the field an 'int' ?
		isInt := false
		for _, i := range ii {
			if i == fname {
				isInt = true
			}
		}

		if isInt {
			values += ", " + f.String()
			upd += f.String()
		} else {
			values += ", '" + f.String() + "'"
			upd += "'" + f.String() + "'"
		}
	}
	fields = fields[2:]
	values = values[2:]
	upd = upd[2:]

	switch f {
	case "add":
		fallthrough
	case "replace":
		q := "replace into " + tb + " (" + fields + ") values (" + values + ")"
		log.Printf("gosql.Exec: %s\n", q)

		_, err = db.Db.Exec(q)
	case "insert":
		q := "insert into " + tb + " (" + fields + ") values (" + values + ")"
		log.Printf("gosql.Exec: %s\n", q)
		_, err = db.Db.Exec(q)

	case "delete":
		where := g.Node("where").String()
		q := "delete from " + tb + " where " + where
		log.Printf("gosql.Exec: %s\n", q)
		_, err = db.Db.Exec(q)

	case "update":
		where := g.Node("where").String()
		q := "UPDATE " + tb + " SET " + upd + " WHERE " + where
		log.Printf("gosql.Exec: %s\n", q)
		_, err = db.Db.Exec(q)
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
			values[i] = new(sql.NullString)
		}

		err := rows.Scan(values...)

		if err != nil {
			log.Println("Error reading rows: " + err.Error())
		}

		n := rr.Add("-")
		for i := 0; i < len(cols); i++ {

			ns, ok := values[i].(*sql.NullString)
			v := ""
			if !ok {
				s, ok := values[i].(*string)
				if !ok {
					log.Printf("error: failed to convert type %T [%d]\n", values[i], i)
					continue
				}

				v = *s
			} else {
				v = ns.String
			}

			n.Add(v)
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
			values[i] = new(sql.NullString)
		}

		err := rows.Scan(values...)

		if err != nil {
			log.Println("Error reading rows: " + err.Error())
		}

		n := rr.Add("-")
		for i := 0; i < len(cols); i++ {
			ns, ok := values[i].(*sql.NullString)
			v := ""
			if !ok {
				s, ok := values[i].(*string)
				if !ok {
					log.Printf("error: failed to convert type %T [%d]\n", values[i], i)
					continue
				}

				v = *s
			} else {
				v = ns.String
			}

			n.Add(cols[i]).Add(v)
		}
	}

	return r
}
