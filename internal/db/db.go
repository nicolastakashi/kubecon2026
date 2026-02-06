package db

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

type DB struct {
	conn *sql.DB
}

func New(dbPath string) (*DB, error) {
	conn, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := conn.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{conn: conn}
	if err := db.init(); err != nil {
		return nil, err
	}

	return db, nil
}

func (d *DB) init() error {
	schema := `
	CREATE TABLE IF NOT EXISTS parcels (
		id TEXT PRIMARY KEY,
		status TEXT NOT NULL,
		outcome TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	`
	if _, err := d.conn.Exec(schema); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}
	return nil
}

func (d *DB) Close() error {
	return d.conn.Close()
}

func (d *DB) Conn() *sql.DB {
	return d.conn
}

func (d *DB) GetBacklogCounts() (ingested int, readyToShip int, err error) {
	err = d.conn.QueryRow("SELECT COUNT(*) FROM parcels WHERE status = 'ingested'").Scan(&ingested)
	if err != nil {
		return 0, 0, err
	}

	err = d.conn.QueryRow("SELECT COUNT(*) FROM parcels WHERE status = 'processed' AND outcome = 'ok'").Scan(&readyToShip)
	if err != nil {
		return 0, 0, err
	}

	return ingested, readyToShip, nil
}
