package embed

import (
	"embed"
)

// StaticFiles contains all embedded static files
//
//go:embed static/*
var StaticFiles embed.FS
