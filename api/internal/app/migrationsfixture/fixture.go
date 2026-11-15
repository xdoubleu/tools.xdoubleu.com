// Package migrationsfixture embeds sequential and timestamp-versioned goose
// migrations for testing how they apply together.
package migrationsfixture

import "embed"

//go:embed migrations/*.sql
var FS embed.FS
