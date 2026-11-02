// Package migrations embeds the SQL migrations so the API binary can apply them on start.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
