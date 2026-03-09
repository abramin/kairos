package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// OpenDB opens a SQLite database at the given path.
// If path is ":memory:", uses an in-memory database.
// Sets WAL mode and enables foreign keys.
// Runs migrations automatically.
func OpenDB(path string) (*sql.DB, error) {
	if path != ":memory:" {
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("creating db directory: %w", err)
		}
	}

	// Embed foreign_keys and WAL mode in the DSN so they apply to every
	// connection in the pool, not just the first one opened by sql.Open.
	dsn := path
	if path != ":memory:" {
		dsn = path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)"
	} else {
		dsn = path + "?_pragma=foreign_keys(1)"
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	if err := Migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return db, nil
}
