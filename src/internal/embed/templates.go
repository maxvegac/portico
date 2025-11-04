package embed

import (
	"embed"
)

// Templates contains all embedded template files
//
//go:embed templates/*.tmpl
var Templates embed.FS
