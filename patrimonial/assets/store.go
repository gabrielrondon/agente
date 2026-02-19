package assets

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Asset represents a tracked asset (house, car, appliance, etc).
type Asset struct {
	ID         string
	Name       string
	Type       string // house/car/appliance/etc
	Brand      string
	Model      string
	AcquiredAt *time.Time
	Location   string
	Metadata   map[string]any
	Notes      string
}

// MaintenanceRecord records a maintenance event for an asset.
type MaintenanceRecord struct {
	ID          string
	AssetID     string
	Description string
	Cost        float64
	DoneAt      time.Time
	NextDue     *time.Time
	Supplier    string
}

// Store handles asset persistence.
type Store struct {
	db *sql.DB
}

// NewStore creates an asset store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Add inserts a new asset.
func (s *Store) Add(a Asset) (string, error) {
	if a.ID == "" {
		a.ID = uuid.New().String()
	}
	meta, _ := json.Marshal(a.Metadata)
	var acquiredAt any
	if a.AcquiredAt != nil {
		acquiredAt = a.AcquiredAt.Format("2006-01-02")
	}

	_, err := s.db.Exec(
		`INSERT INTO assets (id, name, type, brand, model, acquired_at, location, metadata, notes)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		a.ID, a.Name, a.Type, a.Brand, a.Model, acquiredAt, a.Location, string(meta), a.Notes,
	)
	if err != nil {
		return "", fmt.Errorf("insert asset: %w", err)
	}
	return a.ID, nil
}

// List returns all assets.
func (s *Store) List() ([]Asset, error) {
	rows, err := s.db.Query(
		`SELECT id, name, type, COALESCE(brand,''), COALESCE(model,''),
		 COALESCE(acquired_at,''), COALESCE(location,''), metadata, COALESCE(notes,'')
		 FROM assets ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []Asset
	for rows.Next() {
		a, err := scanAsset(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *a)
	}
	return result, rows.Err()
}

// Get returns a single asset by ID.
func (s *Store) Get(id string) (*Asset, error) {
	row := s.db.QueryRow(
		`SELECT id, name, type, COALESCE(brand,''), COALESCE(model,''),
		 COALESCE(acquired_at,''), COALESCE(location,''), metadata, COALESCE(notes,'')
		 FROM assets WHERE id = ?`, id,
	)
	return scanAssetRow(row)
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanAsset(r rowScanner) (*Asset, error) {
	return scanAssetRow(r)
}

func scanAssetRow(r rowScanner) (*Asset, error) {
	var a Asset
	var metaJSON, acquiredStr string
	err := r.Scan(&a.ID, &a.Name, &a.Type, &a.Brand, &a.Model,
		&acquiredStr, &a.Location, &metaJSON, &a.Notes)
	if err != nil {
		return nil, err
	}
	if acquiredStr != "" {
		t, _ := time.Parse("2006-01-02", acquiredStr)
		a.AcquiredAt = &t
	}
	if err := json.Unmarshal([]byte(metaJSON), &a.Metadata); err != nil {
		a.Metadata = make(map[string]any)
	}
	return &a, nil
}

// AddMaintenance records a maintenance event.
func (s *Store) AddMaintenance(rec MaintenanceRecord) error {
	if rec.ID == "" {
		rec.ID = uuid.New().String()
	}
	var nextDue any
	if rec.NextDue != nil {
		nextDue = rec.NextDue.Format("2006-01-02")
	}
	_, err := s.db.Exec(
		`INSERT INTO maintenance_history (id, asset_id, description, cost, done_at, next_due, supplier)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		rec.ID, rec.AssetID, rec.Description, rec.Cost,
		rec.DoneAt.Format("2006-01-02"), nextDue, rec.Supplier,
	)
	return err
}

// MaintenanceHistory returns all maintenance records for an asset.
func (s *Store) MaintenanceHistory(assetID string) ([]MaintenanceRecord, error) {
	rows, err := s.db.Query(
		`SELECT id, asset_id, description, COALESCE(cost,0), done_at,
		 COALESCE(next_due,''), COALESCE(supplier,'')
		 FROM maintenance_history WHERE asset_id = ? ORDER BY done_at DESC`, assetID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []MaintenanceRecord
	for rows.Next() {
		var r MaintenanceRecord
		var nextDueStr string
		err := rows.Scan(&r.ID, &r.AssetID, &r.Description, &r.Cost,
			&r.DoneAt, &nextDueStr, &r.Supplier)
		if err != nil {
			return nil, err
		}
		if nextDueStr != "" {
			t, _ := time.Parse("2006-01-02", nextDueStr)
			r.NextDue = &t
		}
		result = append(result, r)
	}
	return result, rows.Err()
}
