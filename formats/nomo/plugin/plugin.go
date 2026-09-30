// Package plugin shows Nomo worksheets (.nomo) as typeset pages in any host
// that runs golib/fn/httphook interceptors (e.g. gserver). Blank import it to
// enable:
//
//	import _ "github.com/rveen/golib/formats/nomo/plugin"
//
// A request for a .nomo file is answered with the worksheet evaluated and drawn
// as the Nomo editor draws it with Typeset on. ?m=raw still returns the source,
// which is gserver's convention for the file as stored.
package plugin

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"log"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/rveen/golib/fn"
	"github.com/rveen/golib/fn/httphook"
	"github.com/rveen/golib/formats/nomo"
)

func init() { httphook.Register(serveNomo) }

// The math font is served from here. A dot directory, so it cannot collide
// with a file a site would publish (fn hides dot entries), and absolute, so the
// same pages work at any depth and in multi-host mode.
const (
	assetDir = "/.nomo/"
	fontURL  = assetDir + "stix-two-math-subset.woff2"
)

// serveNomo returns true when it answered the request, and false to let normal
// handling proceed.
func serveNomo(root *fn.FNode, w http.ResponseWriter, rh *http.Request, reqPath string) bool {
	if rh.Method != http.MethodGet && rh.Method != http.MethodHead {
		return false
	}

	// The font and its licence. Matched on the URL rather than reqPath, which
	// carries the host name in multi-host mode.
	switch rh.URL.Path {
	case fontURL:
		serveAsset(w, rh, "font/woff2", nomo.Font, fontTag)
		return true
	case assetDir + "OFL.txt":
		serveAsset(w, rh, "text/plain; charset=utf-8", nomo.FontLicense, licenseTag)
		return true
	}

	if !strings.EqualFold(path.Ext(reqPath), ".nomo") || rh.FormValue("m") == "raw" {
		return false
	}

	// Same resolution as normal serving, for native, embedded and SVN-backed
	// roots alike. On failure normal handling runs and answers 404.
	fd := *root
	f := &fd
	if err := f.GetRaw(reqPath); err != nil || f.Type == "dir" {
		return false
	}

	// Rendering is milliseconds, so there is no server-side cache; an ETag on
	// the source and the engine lets a browser revalidate without even that.
	sum := sha256.New()
	sum.Write([]byte(nomo.ModuleHash))
	sum.Write(f.Content)
	etag := `"` + hex.EncodeToString(sum.Sum(nil))[:32] + `"`
	w.Header().Set("ETag", etag)
	if rh.Header.Get("If-None-Match") == etag {
		w.WriteHeader(http.StatusNotModified)
		return true
	}

	title := strings.TrimSuffix(path.Base(reqPath), path.Ext(reqPath))
	page, err := nomo.Render(string(f.Content), title, fontURL)
	if err != nil {
		log.Println("nomo render failed:", reqPath, err)
		http.Error(w, "Nomo render failed: "+err.Error(), http.StatusInternalServerError)
		return true
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, rh, path.Base(reqPath)+".html", time.Time{}, bytes.NewReader(page))
	return true
}

var fontTag, licenseTag = tag(nomo.Font), tag(nomo.FontLicense)

func tag(data []byte) string {
	sum := sha256.Sum256(data)
	return `"` + hex.EncodeToString(sum[:16]) + `"`
}

func serveAsset(w http.ResponseWriter, rh *http.Request, mime string, data []byte, etag string) {
	w.Header().Set("Content-Type", mime)
	// A day, not immutable: the URL does not change when a new build of the
	// plugin embeds a new font, so the ETag is what catches it.
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("ETag", etag)
	http.ServeContent(w, rh, path.Base(rh.URL.Path), time.Time{}, bytes.NewReader(data))
}
