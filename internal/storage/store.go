package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"financetracker/internal/models"
)

// Data is the on-disk shape (A5): the entire database is just three lists.
// This mirrors the "Single Schema Principle" — nothing here duplicates or
// pre-aggregates the transactions; every derived number (balances, totals,
// etc.) gets computed on the fly by the domain layer, not stored here.
type Data struct {
	Transactions []models.Transaction `json:"transactions"`
	Quotas       []models.Quota       `json:"quotas"`
	Categories   []models.Category    `json:"categories"`
}

// Store wraps the in-memory Data with a mutex (so concurrent HTTP requests
// later on don't corrupt it) and knows how to load/save that Data to a
// JSON file on disk.
type Store struct {
	mu       sync.RWMutex
	filePath string
	data     Data
}

// NewStore creates a Store pointed at the given JSON file path. It does
// NOT load the file yet — call Load() explicitly right after, so startup
// errors are visible to whoever calls it (main.go), rather than hidden
// inside a constructor.
func NewStore(filePath string) *Store {
	return &Store{filePath: filePath}
}

// Load reads the JSON file into memory. If the file doesn't exist yet
// (first run), it seeds the store with the two mandatory records the
// requirements doc calls for — the "Savings" quota (A2.2) and the
// "Settled" category (A4.2) — and writes that out as the starting file.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	bytes, err := os.ReadFile(s.filePath)
	if os.IsNotExist(err) {
		s.data = Data{
			Quotas:     []models.Quota{models.NewSavingsQuota()},
			Categories: []models.Category{models.NewSettledCategory()},
		}
		return s.saveLocked()
	}
	if err != nil {
		return fmt.Errorf("reading data file: %w", err)
	}

	var d Data
	if err := json.Unmarshal(bytes, &d); err != nil {
		return fmt.Errorf("parsing data file: %w", err)
	}
	s.data = d
	return nil
}

// saveLocked performs the actual write. It assumes the caller already
// holds the lock (the "Locked" suffix is a common Go naming convention for
// an internal helper that skips its own locking because the caller did it).
//
// A5 requires "atomic writes (temp file → rename) to avoid corruption on
// crash." We do exactly that: write the full JSON to a temp file first,
// and only if that fully succeeds, rename it over the real file. os.Rename
// is atomic on Linux — the real file is never left half-written, even if
// the program crashes mid-write.
func (s *Store) saveLocked() error {
	bytes, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding data: %w", err)
	}

	dir := filepath.Dir(s.filePath)
	tmp, err := os.CreateTemp(dir, "data-*.tmp")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(bytes); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, s.filePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file into place: %w", err)
	}
	return nil
}

// View gives read-only access to the current data. Pass in a function that
// reads whatever it needs — View holds a read lock for the duration, so
// multiple Views can run at once, but they'll wait for any in-progress
// Update to finish first.
func (s *Store) View(fn func(Data)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	fn(s.data)
}

// Update gives exclusive read+write access, then persists the result to
// disk before returning. This should be the ONLY way the rest of the app
// changes stored data — it guarantees every change is immediately saved,
// so nothing lives in memory-only for long.
//
// fn takes a *Data (pointer) so it can actually modify the store's fields.
// If fn returns an error, we skip saving — this lets callers abort a
// change (e.g. a failed validation) without writing anything to disk.
func (s *Store) Update(fn func(*Data) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := fn(&s.data); err != nil {
		return err
	}
	return s.saveLocked()
}