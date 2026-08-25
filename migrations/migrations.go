// Package migrations exposes Forge's versioned SQL migrations.
package migrations

import "embed"

// Files contains the immutable, numbered SQL migrations.
//
//go:embed *.sql
var Files embed.FS
