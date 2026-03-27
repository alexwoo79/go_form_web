// Package ui provides embedded template and static assets for runtime serving.
package ui

import "embed"

// Templates embeds all HTML templates used by the web handlers.
//
//go:embed templates/*.html
var Templates embed.FS

// Static embeds all static assets served by /static.
//
//go:embed all:static
var Static embed.FS
