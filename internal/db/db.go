package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open creates a new SQLite database connection with optimized pragmas.
func Open(dbPath string) (*sql.DB, error) {
	dsn := dbPath + "?_pragma=busy_timeout(5000)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("opening database %q: %w", dbPath, err)
	}

	// Connection pool tuned for low-memory device.
	db.SetMaxOpenConns(1) // SQLite is single-writer; WAL allows concurrent reads.

	// Set additional pragmas.
	pragmas := []struct {
		name, value string
	}{
		{"journal_mode", "WAL"},
		{"synchronous", "NORMAL"},
		{"foreign_keys", "ON"},
	}
	for _, p := range pragmas {
		if _, err := db.Exec("PRAGMA " + p.name + "=" + p.value); err != nil {
			db.Close()
			return nil, fmt.Errorf("setting PRAGMA %s=%s: %w", p.name, p.value, err)
		}
	}

	// Verify the connection works.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return db, nil
}

// Close closes the database connection.
func Close(db *sql.DB) error {
	return db.Close()
}
