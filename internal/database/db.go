package database

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

func Open(dbFile string) (*sql.DB, error) {
	dsn := dbFile + "?_journal=WAL&_synchronous=NORMAL&_cache_size=-64000&_temp_store=MEMORY&_mmap_size=268435456&_foreign_keys=ON"

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping db: %w", err)
	}

	return db, nil
}
