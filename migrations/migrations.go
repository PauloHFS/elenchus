package migrations

import "embed"

// FS exposes the migrations directory
//
//go:embed *.sql
var FS embed.FS
