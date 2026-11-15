// Package showsmigrations holds the favourite shows' migrations, which keep
// the "podcasts" schema the shows were created in.
package showsmigrations

import "embed"

//go:embed migrations/*.sql
var FS embed.FS
