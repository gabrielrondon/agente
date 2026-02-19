package memory

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// PurchaseRecord stores a completed purchase in memory.
type PurchaseRecord struct {
	ID              string
	Description     string
	Items           []string
	ChosenSupplier  string
	TotalPrice      float64
	CreatedAt       time.Time
}

// Store handles purchase memory persistence.
type Store struct {
	db *sql.DB
}

// NewStore creates a memory store.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Save records a completed purchase.
func (s *Store) Save(rec PurchaseRecord) error {
	if rec.ID == "" {
		rec.ID = uuid.New().String()
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now()
	}
	items, _ := json.Marshal(rec.Items)
	_, err := s.db.Exec(
		`INSERT INTO purchase_memory (id, description, items, chosen_supplier, total_price, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		rec.ID, rec.Description, string(items), rec.ChosenSupplier, rec.TotalPrice, rec.CreatedAt,
	)
	return err
}

// Recent returns the most recent n purchases.
func (s *Store) Recent(n int) ([]PurchaseRecord, error) {
	rows, err := s.db.Query(
		`SELECT id, description, items, COALESCE(chosen_supplier,''), COALESCE(total_price,0), created_at
		 FROM purchase_memory ORDER BY created_at DESC LIMIT ?`, n,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []PurchaseRecord
	for rows.Next() {
		var r PurchaseRecord
		var itemsJSON string
		err := rows.Scan(&r.ID, &r.Description, &itemsJSON, &r.ChosenSupplier, &r.TotalPrice, &r.CreatedAt)
		if err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(itemsJSON), &r.Items)
		records = append(records, r)
	}
	return records, rows.Err()
}

// Last returns the most recent purchase, or nil if none.
func (s *Store) Last() (*PurchaseRecord, error) {
	records, err := s.Recent(1)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	return &records[0], nil
}

// Format returns a human-readable summary of recent purchases.
func (s *Store) Format(n int) (string, error) {
	records, err := s.Recent(n)
	if err != nil {
		return "", err
	}
	if len(records) == 0 {
		return "Nenhuma compra registrada ainda.", nil
	}

	out := fmt.Sprintf("Últimas %d compras:\n\n", len(records))
	for i, r := range records {
		out += fmt.Sprintf("%d. %s\n", i+1, r.Description)
		out += fmt.Sprintf("   Fornecedor: %s | Total: R$ %.2f | Data: %s\n",
			r.ChosenSupplier, r.TotalPrice, r.CreatedAt.Format("02/01/2006"))
	}
	return out, nil
}
