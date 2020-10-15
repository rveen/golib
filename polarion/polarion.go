// This package implements an API over a working copy of a Subversion repository
// of a Polarion project. It qualifies as a hack.
//
// NOTE To build a local copy with revisions, svnrdump can be used, and then a
// local SVN or Git server repo created. But: Git doesn't support files > 100 MB !
//
// NOTE https://gist.github.com/mbohun/1448d44901372e1fb1b5
//
package polarion

import (
	"bytes"
	"encoding/xml"
	"io/ioutil"

	"log"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/rveen/ogdl"
	"golang.org/x/net/html"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var PolarionBasePath = ""

// Polarion holds information common to all projects
type Polarion struct {
	// folders refers to project folders, like /var/polarion, with working copies
	// of projects. In a future version, SVN server folders could be supported.
	folders []string

	projects  []*Project
	documents []*Document

	// items contain all the workitems present in all the projects in []folders
	Items map[string]*Item
	Links []*Link
}

// Project is a container in RAM of parts of the project on disk.
//
// The project ID used is the base name of the path in upper case. If the ID
// differs from the folder name it has to be changed manually after the call to Open.
type Project struct {
	ID   string
	Path string

	documents []*Document
	Items     map[string]*Item
	links     []*Link
}

type Document struct {
	ID    string
	Space string
	Prj   *Project
	Path  string

	Items []string
	Tree  *ogdl.Graph
	Toc   *ogdl.Graph
}

type Item struct {
	ID   string
	Type string
	Tree *ogdl.Graph
	Path string
}

// Link holds the relation between two workitems, which may possibly reside in
// another project.
type Link struct {
	Source      *Item
	Destination *Item
	Type        string
}

func (P *Polarion) AddFolder(path string) {

	path, _ = filepath.Abs(path)
	path += "/"
	P.folders = append(P.folders, path)

	// Load project info
	dd := ListDirectories(path)

	for _, d := range dd {

		p := getProject(path, d)
		P.projects = append(P.projects, p)
		P.documents = append(P.documents, p.documents...)
		for _, it := range p.Items {
			if P.Items == nil {
				P.Items = make(map[string]*Item)
			}
			P.Items[it.ID] = it
		}
	}

	P.findLinks()
}

func (P *Polarion) AddProject(path string) {

	path, _ = filepath.Abs(path)

	base := filepath.Base(path)
	dir := filepath.Dir(path)

	p := getProject(dir, base)
	P.projects = append(P.projects, p)
	P.documents = append(P.documents, p.documents...)
	for _, it := range p.Items {
		if P.Items == nil {
			P.Items = make(map[string]*Item)
		}
		P.Items[it.ID] = it
	}

	P.findLinks()
}

func (P *Polarion) Item(id string) *Item {
	return P.Items[id]
}

func (P *Polarion) Document(prj, space, doc string) *Document {

	// log.Printf("Doc\n%s / %s / %s", prj, space, doc)

	for _, d := range P.documents {
		//log.Printf("Doc\n%s / %s / %s", d.Prj.ID, d.Space, d.ID)

		if d.Space == space && d.Prj.ID == prj && doc == d.ID {
			return d
		}
	}
	return nil
}

func (item *Item) Document() string {
	if len(item.Path) < 11 {
		return ""
	}

	return filepath.Base(item.Path[0 : len(item.Path)-10])
}

func (P *Polarion) Documents(prj string) *ogdl.Graph {

	g := ogdl.New(nil)

	for _, p := range P.projects {
		if p.ID == prj {
			for _, doc := range p.documents {
				g.Add(doc.ID)
			}
			return g
		}
	}

	return nil
}

func (P *Polarion) Projects() *ogdl.Graph {
	g := ogdl.New(nil)

	for _, p := range P.projects {
		g.Add(p.ID)
	}

	return g
}

// Discover all links between work items (fill P.Links)
func (P *Polarion) findLinks() {
	// Range over all work items
	for _, item := range P.Items {
		links := item.Tree.Get("linkedWorkItems")
		if links == nil {
			continue
		}
		links = links.Out[0]
		for _, link := range links.Out {
			role, _ := link.GetString("role")
			wi, _ := link.GetString("workItem")
			l := Link{}
			l.Source = item
			l.Destination = P.Items[wi]
			l.Type = role
			P.Links = append(P.Links, &l)
		}
	}
}

// Read project info, load documents and items
func getProject(path, name string) *Project {

	if path[len(path)-1] != '/' {
		path = path + "/"
	}

	p := &Project{Path: path + name, ID: name}

	// Range over spaces
	for _, space := range p.Spaces() {
		// log.Println("space", space)
		// Range over docs
		docs := p.Documents(space)
		for _, doc := range docs {
			d := p.getDocument(space, doc)
			p.documents = append(p.documents, d)

			// Scan workitems
			p.getItems(space, doc)
		}
	}

	p.getItemsNoDoc()

	return p
}

func (p *Project) getItems(space, doc string) {

	path := p.Path + "/modules/" + space + "/" + doc + "/workitems"
	files, err := ioutil.ReadDir(path)

	if err != nil {
		return
	}

	for _, file := range files {
		if file.IsDir() {
			wi := Item{}

			wi.ID = file.Name()
			wi.Path = "/modules/" + space + "/" + doc + "/workitems"
			wi.Tree = readXml(path + "/" + wi.ID + "/workitem.xml")

			// normalize
			wi.Tree = wi.Tree.GetAt(0)
			simplify(wi.Tree)

			wi.Tree.This = "workitem"
			wi.Tree.Add("id").Add(wi.ID)
			// log.Println("wi.ID", wi.ID)

			wi.Type, _ = wi.Tree.GetString("type")

			if p.Items == nil {
				p.Items = make(map[string]*Item)
			}

			p.Items[wi.ID] = &wi
		}
	}
}

func (p *Project) getItemsNoDoc() {

	path := p.Path + "/.polarion/tracker/workitems"
	files, err := ioutil.ReadDir(path)

	if err != nil {
		log.Println("no workitems outside docs")
	}

	for _, file := range files {
		p.getItemsNoDoc_(path + "/" + file.Name())
	}
}

func (p *Project) getItemsNoDoc_(path string) {

	files, err := ioutil.ReadDir(path)

	if err != nil {
		return
	}

	for _, file := range files {
		if file.IsDir() {
			p.getItemsNoDoc_(path + "/" + file.Name())
			continue
		}

		// if not workitem.xml ignore
		if file.Name() != "workitem.xml" {
			continue
		}

		wi := Item{}

		wi.ID = filepath.Base(path)
		wi.Path = ""
		wi.Tree = readXml(path + "/" + file.Name())

		// normalize
		wi.Tree = wi.Tree.GetAt(0)
		simplify(wi.Tree)

		wi.Tree.This = "workitem"
		wi.Tree.Add("id").Add(wi.ID)

		wi.Type, _ = wi.Tree.GetString("type")

		if p.Items == nil {
			p.Items = make(map[string]*Item)
		}

		p.Items[wi.ID] = &wi
	}
}

func (p *Project) getDocument(space, doc string) *Document {

	path := p.Path + "/modules/" + space + "/" + doc + "/module.xml"
	g := readXml(path)
	simplify(g)
	g.Out[0].Add("id").Add(doc)

	d := &Document{}
	d.ID = doc
	d.Tree = g
	d.Space = space
	d.Prj = p

	d.getItems()

	return d
}

// Get items referenced in the HTML portion of the XML document
func (doc *Document) getItems() {

	if doc == nil || doc.Tree == nil {
		return
	}

	doc.Toc = ogdl.New(nil)

	htm := doc.Tree.Get("module.homePageContent").String()

	z := html.NewTokenizer(strings.NewReader(htm))

	for {
		tt := z.Next()

		if tt == html.ErrorToken {
			break
		}

		if tt == html.StartTagToken {

			htag, attr := z.TagName()

			if attr {
				for {
					k, v, more := z.TagAttr()
					attr := string(k)
					val := string(v)
					if attr == "id" {
						i := strings.Index(val, "workitem;params=id=")
						if i != -1 {
							wi := val[i+19:]
							i = strings.Index(wi, "|") // External!
							if i != -1 {
								wi = wi[0:i]
							}
							tag := string(htag)
							if tag == "div" {
								// Store the item ID, because the item itself may not
								// be available in this project
								doc.Items = append(doc.Items, wi)
							} else if htag[0] == 'h' && len(htag) == 2 {
								// item is header
							}
						}
					}
					if !more {
						break
					}
				}
			}
		}
	}
}

func (doc *Document) Html() string {

	if doc == nil || doc.Tree == nil {
		return "No content"
	}

	doc.Toc = ogdl.New(nil)

	htm := doc.Tree.Get("module.homePageContent").String()

	z := html.NewTokenizer(strings.NewReader(htm))

	var buffer bytes.Buffer

	var h = []int{0, 0, 0, 0, 0, 0}

	attachmentDir := PolarionBasePath + doc.Prj.ID + "/modules/" + doc.Space + "/" + doc.ID + "/attachments/"
	wiDir := PolarionBasePath + doc.Prj.ID + "/modules/" + doc.Space + "/" + doc.ID + "/workitems/"

	removeStyle := regexp.MustCompile(`style=".*"`)

	for {
		tt := z.Next()

		if tt == html.ErrorToken {
			break
		}
		switch tt {

		case html.SelfClosingTagToken:

			htag, attr := z.TagName()
			buffer.WriteString("<")
			buffer.Write(htag)
			if attr {
				for {
					k, v, more := z.TagAttr()
					attr := string(k)
					val := string(v)

					if attr == "src" && strings.HasPrefix(val, "attachment:") {
						val = attachmentDir + val[11:]
					}
					buffer.WriteString(" " + attr)
					buffer.WriteString("=\"" + val + "\" ")

					if !more {
						break
					}
				}
			}
			buffer.WriteString(">")

		case html.StartTagToken:

			htag, attr := z.TagName()
			buffer.WriteString("<")
			buffer.Write(htag)
			closed := false

			if attr {
				for {
					k, v, more := z.TagAttr()
					attr := string(k)
					if attr == "style" {
						continue
					}
					val := string(v)
					if attr == "id" {
						i := strings.Index(val, "workitem;params=id=")
						if i != -1 {
							wi := val[i+19:]
							i = strings.Index(wi, "|") // External!
							cls := "item"
							if i != -1 {
								wi = wi[0:i]
								cls = "item-ext"
							}
							closed = true
							item := doc.Prj.Items[wi]
							if item != nil {
								tag := string(htag)
								if tag == "div" {
									buffer.WriteString(" class='" + cls + "'>")

									itemTitle := item.Tree.Get("title").String()
									buffer.WriteString("<a id='" + item.ID + "'>")

									// Add item to TOI
									toi := doc.Tree.Get("toi")
									if toi == nil {
										toi = doc.Tree.Add("toi")
									}
									toi.Add(item.ID).Add(itemTitle)

									buffer.WriteString("<div class='wiTitle'>")
									buffer.WriteString(itemTitle)
									buffer.WriteString(" (<span class='uid'>" + item.ID + ", " + item.Tree.Get("type").String() + "</span>)")
									buffer.WriteString("</div><br>")

									desc := item.Tree.Get("description").String()
									desc = strings.Replace(desc, "attachment:", attachmentDir, -1)
									desc = strings.Replace(desc, "workitemimg:", wiDir+item.ID+"/attachment", -1)

									// This possible HTML is ugly (full of spans and style)
									desc = removeStyle.ReplaceAllString(desc, "")

									buffer.WriteString(desc)

									// Add test steps if any
									ts := item.Tree.Get("testSteps.struct.steps.list")

									if ts != nil && ts.Len() > 0 {

										buffer.WriteString("<table class='table'>\n")
										buffer.WriteString("<thead><tr><td>Step</td><td>Description</td><td>Criteria</td></tr></thead>\n")

										for _, t := range ts.Out {

											buffer.WriteString("<tr>")
											step := t.Get("values.list")

											for _, col := range step.Out {
												cell := col.GetAt(1).ThisString()
												cell = strings.Replace(cell, "attachment:", attachmentDir, -1)
												buffer.WriteString("<td>" + cell + "</td>")
											}
											buffer.WriteString("</tr>\n")

										}
										buffer.WriteString("</table>\n")
									}

								} else if htag[0] == 'h' && len(htag) == 2 {

									title := item.Tree.Get("title").String()
									anchor := Sanitize(title)

									buffer.WriteString(" id='" + anchor + "'>")

									lev := int(htag[1] - '0')
									if lev >= 2 && lev < 7 {
										h[lev-1]++
										for i = lev; i < 6; i++ {
											h[i] = 0
										}

										var hd bytes.Buffer
										for i = 1; i < lev; i++ {
											hd.WriteString(strconv.Itoa(h[i]))
											hd.WriteByte('.')
										}

										hd.WriteByte(' ')
										hd.WriteString(title)
										buffer.WriteString(hd.String())
										doc.Toc.Add(hd.String()).Add(Sanitize(anchor))
									}
								}

							}
						}
					} else {
						buffer.WriteString(" " + attr)
						buffer.WriteString("=\"" + val + "\" ")
					}
					if !more {
						break
					}
				}
			}
			if !closed {
				buffer.WriteString(">")
			}

		case html.EndTagToken:

			b, _ := z.TagName()
			buffer.WriteString("</")
			buffer.Write(b)
			buffer.WriteString(">\n")
		case html.TextToken:
			buffer.Write(z.Text())
		}
	}
	return buffer.String()
}

// Spaces returns all names of directories in the modules/ directory of the project
func (p *Project) Spaces() []string {
	return ListDirectories(p.Path + "/modules")
}

// Documents returns all names of directories in the modules/<space>/ directory of
// the project
func (p *Project) Documents(space string) []string {
	return ListDirectories(p.Path + "/modules/" + space)
}

func ListDirectories(path string) []string {

	files, err := ioutil.ReadDir(path)

	if err != nil {
		log.Println("ListDirectories", err)
		return nil
	}

	var ss []string

	for _, file := range files {
		if file.IsDir() {
			ss = append(ss, file.Name())
		}
	}
	return ss

}

// TODO This is now available in ogdl.io.gxml

func simplify(g *ogdl.Graph) {
	if g.Out == nil {
		return
	}

	for _, n := range g.Out {

		if len(n.Out) == 0 {
			continue
		}
		if n.Out[0].ThisString() == "@id" && len(n.Out) >= 2 {
			if n.This != "p" {
				n.This = n.Out[0].Out[0].This
			}
			n.Out[0] = n.Out[len(n.Out)-1]
			n.Out = n.Out[:1]
		}
		simplify(n)
	}
}

func isMn(r rune) bool {
	return unicode.Is(unicode.Mn, r) // Mn: nonspacing marks
}

func readXml(file string) *ogdl.Graph {

	b, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}

	decoder := xml.NewDecoder(bytes.NewReader(b))

	g := ogdl.New(nil)
	var key string
	level := -1

	att := true

	var stack []*ogdl.Graph
	stack = append(stack, g)

	tr := transform.Chain(norm.NFD, transform.RemoveFunc(isMn), norm.NFC)

	for {
		// Read tokens from the XML document in a stream.
		t, _ := decoder.Token()
		if t == nil {
			break
		}
		// Inspect the type of the token just read.
		switch se := t.(type) {

		case xml.StartElement:
			level++

			key = se.Name.Local
			// No accents in key
			key, _, _ = transform.String(tr, key)

			n := stack[len(stack)-1].Add(key)
			// push
			stack = append(stack, n)
			if att {
				for _, at := range se.Attr {
					n.Add("@" + at.Name.Local).Add(at.Value)
				}
			}

		case xml.CharData:

			val := strings.TrimSpace(string(se))
			if len(val) > 0 {
				stack[len(stack)-1].Add(val)
			}

		case xml.EndElement:
			level--
			// pop

			stack = stack[:len(stack)-1]

		}
	}

	return g
}

// Sanitize a string for use as an anchor
// source:github.com/shurcooL/sanitized_anchor_name
func Sanitize(text string) string {
	var anchorName []rune
	var futureDash = false
	for _, r := range text {
		switch {
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			if futureDash && len(anchorName) > 0 {
				anchorName = append(anchorName, '-')
			}
			futureDash = false
			anchorName = append(anchorName, unicode.ToLower(r))
		default:
			futureDash = true
		}
	}
	return string(anchorName)
}
