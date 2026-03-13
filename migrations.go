package synd

import "embed"

// Migrations contains the SQL migration files for axon-synd.
// Composition roots pass this to axon.RunMigrations or axon.MustRunMigrations.
//
//go:embed migrations/*.sql
var Migrations embed.FS
