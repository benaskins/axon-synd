package synd

import "embed"

// Migrations contains the SQL migration files for axon-synd.
// Composition roots pass this to migration.Run from axon-base.
//
//go:embed migrations/*.sql
var Migrations embed.FS
