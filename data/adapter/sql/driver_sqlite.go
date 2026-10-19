//go:build !nitro_nosqlite

package sqladapter

// SQLite (pure Go, modernc). Exclude with -tags nitro_nosqlite to shrink
// Postgres-only binaries by several MB.
import _ "modernc.org/sqlite"
