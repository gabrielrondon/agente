package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

// Open opens (or creates) the SQLite database at the given path.
func Open(dsn string) (*sql.DB, error) {
	if err := os.MkdirAll(filepath.Dir(dsn), 0o755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return db, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(schema)
	return err
}

const schema = `
PRAGMA journal_mode=WAL;
PRAGMA foreign_keys=ON;

-- comprador
CREATE TABLE IF NOT EXISTS suppliers (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  phone      TEXT NOT NULL,
  city       TEXT NOT NULL,
  categories TEXT NOT NULL DEFAULT '[]',  -- JSON array
  rating     REAL NOT NULL DEFAULT 0,
  active     BOOLEAN NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS quotes (
  id           TEXT PRIMARY KEY,
  request_id   TEXT NOT NULL,
  supplier_id  TEXT NOT NULL,
  items        TEXT NOT NULL DEFAULT '[]', -- JSON
  response     TEXT,
  price        REAL,
  status       TEXT NOT NULL DEFAULT 'pending', -- pending/received/accepted/rejected
  created_at   DATETIME NOT NULL,
  responded_at DATETIME
);

CREATE TABLE IF NOT EXISTS purchase_memory (
  id               TEXT PRIMARY KEY,
  description      TEXT NOT NULL,
  items            TEXT NOT NULL DEFAULT '[]',
  chosen_supplier  TEXT,
  total_price      REAL,
  created_at       DATETIME NOT NULL
);

-- patrimonial
CREATE TABLE IF NOT EXISTS assets (
  id          TEXT PRIMARY KEY,
  name        TEXT NOT NULL,
  type        TEXT NOT NULL,  -- house/car/appliance/etc
  brand       TEXT,
  model       TEXT,
  acquired_at DATE,
  location    TEXT,
  metadata    TEXT NOT NULL DEFAULT '{}', -- JSON livre
  notes       TEXT
);

CREATE TABLE IF NOT EXISTS maintenance_history (
  id          TEXT PRIMARY KEY,
  asset_id    TEXT NOT NULL,
  description TEXT NOT NULL,
  cost        REAL,
  done_at     DATE NOT NULL,
  next_due    DATE,
  supplier    TEXT,
  FOREIGN KEY(asset_id) REFERENCES assets(id)
);
`
