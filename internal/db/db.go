package db

import (
	"database/sql"
	"fmt"
	"runtime"
	"time"

	_ "modernc.org/sqlite"
)

// buildDSN constructs an optimized SQLite DSN with WAL-mode pragmas tuned for
// a low-memory device. Both the write and read pools point at the same file.
func buildDSN(dbPath string) string {
	return "file:" + dbPath +
		"?_pragma=busy_timeout(5000)" +
		"&_pragma=cache_size(-25600)" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=journal_size_limit(1048576)" +
		"&_pragma=mmap_size(67108864)" +
		"&_pragma=synchronous(NORMAL)" +
		"&_pragma=temp_store(MEMORY)" +
		"&_pragma=wal_autocheckpoint(500)" +
		"&_txlock=immediate"
}

// Open creates a new SQLite write connection pool with optimized pragmas.
// SQLite is single-writer, so the write pool is serialized to a single
// connection; concurrent reads are served by the separate read pool.
func Open(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", buildDSN(dbPath))
	if err != nil {
		return nil, fmt.Errorf("opening database %q: %w", dbPath, err)
	}

	// Serialize writes: SQLite allows only one writer at a time.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	// Verify the connection works.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging database: %w", err)
	}

	return db, nil
}

// OpenReadDB opens a second connection pool to the same database file tuned
// for concurrent reads under WAL. It must be closed separately from the write
// pool returned by Open.
func OpenReadDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", buildDSN(dbPath))
	if err != nil {
		return nil, fmt.Errorf("opening read database %q: %w", dbPath, err)
	}

	// WAL allows concurrent readers; size the pool for the device's cores.
	maxOpen := 2 * runtime.NumCPU()
	if maxOpen > 16 {
		maxOpen = 16
	}
	db.SetMaxOpenConns(maxOpen)
	db.SetConnMaxIdleTime(5 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)

	// Verify the connection works.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("pinging read database: %w", err)
	}

	return db, nil
}

// Close closes the database connection.
func Close(db *sql.DB) error {
	return db.Close()
}