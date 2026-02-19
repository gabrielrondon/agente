package suppliers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Supplier represents a local vendor.
type Supplier struct {
	ID         string
	Name       string
	Phone      string
	City       string
	Categories []string
	Rating     float64
	Active     bool
}

// Store handles supplier persistence.
type Store struct {
	db *sql.DB
}

// NewStore creates a supplier store backed by the given DB.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// Add inserts a new supplier and returns its generated ID.
func (s *Store) Add(sup Supplier) (string, error) {
	if sup.ID == "" {
		sup.ID = uuid.New().String()
	}
	cats, err := json.Marshal(sup.Categories)
	if err != nil {
		return "", fmt.Errorf("marshal categories: %w", err)
	}

	_, err = s.db.Exec(
		`INSERT INTO suppliers (id, name, phone, city, categories, rating, active)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		sup.ID, sup.Name, sup.Phone, sup.City, string(cats), sup.Rating, sup.Active,
	)
	if err != nil {
		return "", fmt.Errorf("insert supplier: %w", err)
	}
	return sup.ID, nil
}

// List returns all active suppliers.
func (s *Store) List() ([]Supplier, error) {
	rows, err := s.db.Query(
		`SELECT id, name, phone, city, categories, rating, active
		 FROM suppliers WHERE active = 1 ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSuppliers(rows)
}

// ByCategory returns active suppliers that match any of the given categories.
func (s *Store) ByCategory(categories []string) ([]Supplier, error) {
	// SQLite doesn't support array params; load all and filter in Go
	all, err := s.List()
	if err != nil {
		return nil, err
	}

	catSet := make(map[string]bool, len(categories))
	for _, c := range categories {
		catSet[c] = true
	}

	var result []Supplier
	for _, sup := range all {
		for _, c := range sup.Categories {
			if catSet[c] {
				result = append(result, sup)
				break
			}
		}
	}
	return result, nil
}

// Get returns a supplier by ID.
func (s *Store) Get(id string) (*Supplier, error) {
	row := s.db.QueryRow(
		`SELECT id, name, phone, city, categories, rating, active
		 FROM suppliers WHERE id = ?`, id,
	)
	sup, err := scanSupplier(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return sup, err
}

// UpdateRating updates a supplier's rating.
func (s *Store) UpdateRating(id string, rating float64) error {
	_, err := s.db.Exec(`UPDATE suppliers SET rating = ? WHERE id = ?`, rating, id)
	return err
}

func scanSuppliers(rows *sql.Rows) ([]Supplier, error) {
	var result []Supplier
	for rows.Next() {
		sup, err := scanSupplierRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, *sup)
	}
	return result, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanSupplier(row *sql.Row) (*Supplier, error) {
	return scanSupplierRow(row)
}

func scanSupplierRow(row rowScanner) (*Supplier, error) {
	var sup Supplier
	var catsJSON string
	err := row.Scan(&sup.ID, &sup.Name, &sup.Phone, &sup.City, &catsJSON, &sup.Rating, &sup.Active)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(catsJSON), &sup.Categories); err != nil {
		sup.Categories = nil
	}
	return &sup, nil
}

// QuoteStore manages quotes in the database.
type QuoteStore struct {
	db *sql.DB
}

// NewQuoteStore creates a QuoteStore.
func NewQuoteStore(db *sql.DB) *QuoteStore {
	return &QuoteStore{db: db}
}

// Quote represents a price quote from a supplier.
type Quote struct {
	ID          string
	RequestID   string
	SupplierID  string
	Items       []QuoteItem
	Response    string
	Price       float64
	Status      string // pending/received/accepted/rejected
	CreatedAt   time.Time
	RespondedAt *time.Time
}

// QuoteItem is a single item in a quote.
type QuoteItem struct {
	Name     string  `json:"name"`
	Qty      float64 `json:"qty"`
	Unit     string  `json:"unit"`
	Price    float64 `json:"price,omitempty"`
}

// CreateQuote inserts a pending quote.
func (qs *QuoteStore) CreateQuote(q Quote) error {
	if q.ID == "" {
		q.ID = uuid.New().String()
	}
	items, _ := json.Marshal(q.Items)
	_, err := qs.db.Exec(
		`INSERT INTO quotes (id, request_id, supplier_id, items, status, created_at)
		 VALUES (?, ?, ?, ?, 'pending', ?)`,
		q.ID, q.RequestID, q.SupplierID, string(items), q.CreatedAt,
	)
	return err
}

// UpdateQuoteResponse records a supplier's response.
func (qs *QuoteStore) UpdateQuoteResponse(id, response string, price float64) error {
	_, err := qs.db.Exec(
		`UPDATE quotes SET response = ?, price = ?, status = 'received', responded_at = ?
		 WHERE id = ?`,
		response, price, time.Now(), id,
	)
	return err
}

// PendingByRequest returns all pending quotes for a request.
func (qs *QuoteStore) PendingByRequest(requestID string) ([]Quote, error) {
	rows, err := qs.db.Query(
		`SELECT id, request_id, supplier_id, items, COALESCE(response,''), COALESCE(price,0), status, created_at
		 FROM quotes WHERE request_id = ?`, requestID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var quotes []Quote
	for rows.Next() {
		var q Quote
		var itemsJSON string
		err := rows.Scan(&q.ID, &q.RequestID, &q.SupplierID, &itemsJSON, &q.Response, &q.Price, &q.Status, &q.CreatedAt)
		if err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(itemsJSON), &q.Items)
		quotes = append(quotes, q)
	}
	return quotes, rows.Err()
}
