// Package migrations holds the blog's goose migrations as embedded SQL
// files.
package migrations

import "embed"

// FS is the migration directory, applied in version order by goose.
//
//go:embed *.sql
var FS embed.FS
