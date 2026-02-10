package web

import "embed"

// DistFS holds the built React SPA files from the dist/ directory.
// When the UI has not been built, it contains only a placeholder index.html.
//
//go:embed dist/*
var DistFS embed.FS
