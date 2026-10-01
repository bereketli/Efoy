// Package db embeds the goose SQL migrations so the core-api binary can apply
// them (`core-api migrate up`), for example from the Kubernetes pre-sync job.
package db

import "embed"

// Migrations holds db/migrations/*.sql under the "migrations" directory.
//
//go:embed migrations/*.sql
var Migrations embed.FS
