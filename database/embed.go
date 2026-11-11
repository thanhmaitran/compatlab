// Package database embeds the SQL migrations so the server binary can apply
// them on startup. The files follow the golang-migrate naming convention
// (NNNNNN_name.up.sql / .down.sql), so the migrate CLI can also be used.
package database

import "embed"

// Migrations contains the files under migrations/.
//
//go:embed migrations/*.sql
var Migrations embed.FS
