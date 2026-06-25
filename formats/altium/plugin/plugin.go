// Package plugin enables on-demand Altium->KiCad conversion in any host that
// runs golib/fn/httphook interceptors (e.g. gserver). Blank import it to enable:
//
//	import _ "github.com/rveen/golib/formats/altium/plugin"
//
// It registers a "virtual extension" interceptor and pulls in the altium
// converter as a dependency only of this package, so the host need not import
// the converter directly.
package plugin

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rveen/golib/fn"
	"github.com/rveen/golib/fn/httphook"
	"github.com/rveen/golib/formats/altium"
)

func init() { httphook.Register(serveAltiumKicad) }

// kicadCacheDir holds converted Altium->KiCad output. It starts with a dot so
// fn.dir() hides it from directory listings.
const kicadCacheDir = ".kicad-cache"

// serveAltiumKicad handles "virtual extension" requests for Altium files viewed
// in KiCanvas: a path like "Foo.SchDoc.kicad_sch" (resp. "Foo.PcbDoc.kicad_pcb")
// refers to the real Altium file "Foo.SchDoc" (resp. "Foo.PcbDoc") converted to
// KiCad format on the fly. The KiCad extension is kept on the URL so KiCanvas
// selects the right parser.
//
// It returns true when the request was handled (served or errored). It returns
// false when reqPath is not such a virtual path, so normal handling proceeds.
func serveAltiumKicad(root *fn.FNode, w http.ResponseWriter, rh *http.Request, reqPath string) bool {

	var base, kicadExt, altiumExt string
	var convert func([]byte) ([]byte, error)

	switch {
	case strings.HasSuffix(reqPath, ".kicad_sch"):
		base = strings.TrimSuffix(reqPath, ".kicad_sch")
		kicadExt, altiumExt, convert = ".kicad_sch", ".schdoc", altium.ConvertToKicadSch
	case strings.HasSuffix(reqPath, ".kicad_pcb"):
		base = strings.TrimSuffix(reqPath, ".kicad_pcb")
		kicadExt, altiumExt, convert = ".kicad_pcb", ".pcbdoc", altium.ConvertToKicadPcb
	default:
		return false
	}

	// Only intercept when the base is actually an Altium file.
	if !strings.HasSuffix(strings.ToLower(base), altiumExt) {
		return false
	}

	// Read the real Altium source. GetRaw uses the same resolution as normal
	// file serving and reads the bytes, working uniformly for native, embedded
	// and SVN-backed roots. (We must not GetMeta first: for SVN-backed files
	// that mutates the node's Root, breaking a subsequent lookup.)
	fd := *root
	f := &fd
	if err := f.GetRaw(base); err != nil || len(f.Content) == 0 {
		return false
	}

	// Cache the converted output keyed by the source content hash. Any change to
	// the source invalidates automatically, and it works regardless of backing
	// store (mtime is unavailable for SVN-backed paths).
	sum := sha1.Sum(f.Content)
	cachePath := filepath.Join(kicadCacheDir, hex.EncodeToString(sum[:])+kicadExt)

	if data, err := os.ReadFile(cachePath); err == nil {
		http.ServeContent(w, rh, filepath.Base(reqPath), time.Time{}, bytes.NewReader(data))
		return true
	}

	out, err := convert(f.Content)
	if err != nil {
		log.Println("altium conversion failed:", base, err)
		http.Error(w, "Altium conversion failed: "+err.Error(), 500)
		return true
	}

	// Write to cache atomically (temp name + rename). Best-effort.
	if err := os.MkdirAll(kicadCacheDir, 0755); err == nil {
		tmp := cachePath + ".tmp"
		if err := os.WriteFile(tmp, out, 0644); err == nil {
			os.Rename(tmp, cachePath)
		}
	}

	http.ServeContent(w, rh, filepath.Base(reqPath), time.Time{}, bytes.NewReader(out))
	return true
}
