package assets

import "embed"

// FS is the embedded filesystem for the assets directory.
//
//go:embed all:*
var FS embed.FS
