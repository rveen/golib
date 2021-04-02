package types

import (
	"mime"
	"path/filepath"
	"strings"
)

var noExtensionNeeded = []string{
	".htm", ".html", ".ogdl", ".md",
}

var typeByExtension = map[string]string{
	".css":  "text/css",
	".htm":  "text/html",
	".html": "text/html",
	".pdf":  "application/pdf",
	".md":   "text/markdown",

	".gif":  "image/gif",
	".png":  "image/png",
	".jpeg": "image/jpeg",
	".jpg":  "image/jpeg",
	".svg":  "image/svg+xml",
	".webp": "image/webp",

	".xml":  "data/xml",
	".ogdl": "data/ogdl",
	".yml":  "data/yaml",
	".yaml": "data/yaml",
	".json": "data/json",
	".nb":   "data/notebook",
}

// TypeByExtension returns the type of a file according to its extension.
// A complete path can be provided, or just the extension  with or without dot.
func TypeByExtension(path string) string {

	// Allows ext to be an extension or a path
	ext := filepath.Ext(path)

	if len(ext) == 0 && len(path) != 0 {
		ext = "." + path
	}

	s := typeByExtension[ext]
	if s == "" {
		s = mime.TypeByExtension(ext)

		// Filter out character sets
		i := strings.IndexByte(s, ';')
		if i != -1 {
			return s[0:i]
		}
	}
	return s
}
