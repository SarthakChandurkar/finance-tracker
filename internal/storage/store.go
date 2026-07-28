package storage

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"

	"financetracker/internal/models"
)

// Data is the on-disk (now: on-Redis) shape (A5): the entire database is
// just three lists. This mirrors the "Single Schema Principle" — nothing
// here duplicates or pre-aggregates the transactions; every derived number
// (balances, totals, etc.) gets computed on the fly by the domain layer,
// not stored here.
type Data struct {
	Transactions  []models.Transaction `json:"transactions"`
	Quotas        []models.Quota       `json:"quotas"`
	Categories    []models.Category    `json:"categories"`
	LastMonthYear string               `json:"last_month_year,omitempty"` // tracks the last month-end sweep we did, so we don't double-sweep
}

// Store wraps the in-memory Data with a mutex (so concurrent HTTP requests
// don't corrupt it) and knows how to load/save that Data as a single JSON
// blob under one Redis key. This keeps the exact same public interface
// (Load / View / Update) the file-based version had, so nothing in the
// domain layer or main.go's handlers needs to change — only how Data gets
// persisted underneath.
type Store struct {
	mu   sync.RWMutex
	rdb  *redis.Client
	key  string
	ctx  context.Context
	data Data
}

// NewStore creates a Store pointed at the given Redis client and key. It
// does NOT load any data yet — call Load() explicitly right after, so
// startup errors are visible to whoever calls it (main.go), rather than
// hidden inside a constructor.
func NewStore(rdb *redis.Client, key string) *Store {
	return &Store{
		rdb: rdb,
		key: key,
		ctx: context.Background(),
	}
}

// Load reads the JSON blob from Redis into memory. If the key doesn't
// exist yet (first run), it seeds the store with the two mandatory
// records the requirements doc calls for — the "Savings" quota (A2.2)
// and the "Settled" category (A4.2) — and writes that out as the
// starting value.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	val, err := s.rdb.Get(s.ctx, s.key).Result()
	if errors.Is(err, redis.Nil) {
		s.data = Data{
			Quotas:     []models.Quota{models.NewSavingsQuota()},
			Categories: []models.Category{models.NewSettledCategory()},
		}
		return s.saveLocked()
	}
	if err != nil {
		return fmt.Errorf("reading data from redis: %w", err)
	}

	var d Data
	if err := json.Unmarshal([]byte(val), &d); err != nil {
		return fmt.Errorf("parsing data from redis: %w", err)
	}
	s.data = d
	return nil
}

// saveLocked performs the actual write. It assumes the caller already
// holds the lock (the "Locked" suffix is a common Go naming convention for
// an internal helper that skips its own locking because the caller did it).
//
// A single SET is inherently atomic in Redis — there's no "half-written"
// state another reader could observe, so we don't need the temp-file +
// rename dance the file-based version used.
func (s *Store) saveLocked() error {
	bytes, err := json.Marshal(s.data)
	if err != nil {
		return fmt.Errorf("encoding data: %w", err)
	}

	if err := s.rdb.Set(s.ctx, s.key, bytes, 0).Err(); err != nil {
		return fmt.Errorf("writing data to redis: %w", err)
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
// Redis before returning. This should be the ONLY way the rest of the app
// changes stored data — it guarantees every change is immediately saved,
// so nothing lives in memory-only for long.
//
// fn takes a *Data (pointer) so it can actually modify the store's fields.
// If fn returns an error, we skip saving — this lets callers abort a
// change (e.g. a failed validation) without writing anything to Redis.
func (s *Store) Update(fn func(*Data) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := fn(&s.data); err != nil {
		return err
	}
	return s.saveLocked()
}
