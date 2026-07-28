package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/redis/go-redis/v9"

	"financetracker/internal/models"
)

// Data is still the shape the rest of the app (domain layer, handlers)
// works with in memory — nothing about View()/Update() changes from the
// caller's perspective. What changes is how Load()/saveLocked() get this
// shape into and out of Redis: instead of one JSON blob, each entity gets
// its own key, indexed by a Redis Set per collection.
type Data struct {
	Transactions []models.Transaction `json:"transactions"`
	Quotas       []models.Quota       `json:"quotas"`
	Categories   []models.Category    `json:"categories"`
}

const (
	keyTxPrefix       = "tx:"
	keyQuotaPrefix    = "quota:"
	keyCategoryPrefix = "category:"

	idxTransactions = "idx:transactions"
	idxQuotas       = "idx:quotas"
	idxCategories   = "idx:categories"
)

// Store wraps the in-memory Data with a mutex, same as before. Load() and
// Update() still hand back/take a single Data value — only the Redis
// layout underneath is schema-wise now.
type Store struct {
	mu   sync.RWMutex
	rdb  *redis.Client
	ctx  context.Context
	data Data
}

// NewStore creates a Store around the given Redis client. Unlike the
// single-blob version, there's no "key" argument any more — the keys are
// fixed per-entity-type prefixes/indexes, defined above.
func NewStore(rdb *redis.Client) *Store {
	return &Store{
		rdb: rdb,
		ctx: context.Background(),
	}
}

// Load reads every entity from Redis into memory. If neither the quotas
// nor categories index exists yet, this is treated as a first run: it
// seeds the mandatory Savings quota (A2.2) and Settled category (A4.2)
// and writes them out.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	exists, err := s.rdb.Exists(s.ctx, idxQuotas, idxCategories).Result()
	if err != nil {
		return fmt.Errorf("checking redis for existing data: %w", err)
	}
	if exists == 0 {
		s.data = Data{
			Quotas:     []models.Quota{models.NewSavingsQuota()},
			Categories: []models.Category{models.NewSettledCategory()},
		}
		return s.saveLocked()
	}

	txs, err := loadCollection[models.Transaction](s.ctx, s.rdb, idxTransactions, keyTxPrefix)
	if err != nil {
		return fmt.Errorf("loading transactions: %w", err)
	}
	quotas, err := loadCollection[models.Quota](s.ctx, s.rdb, idxQuotas, keyQuotaPrefix)
	if err != nil {
		return fmt.Errorf("loading quotas: %w", err)
	}
	categories, err := loadCollection[models.Category](s.ctx, s.rdb, idxCategories, keyCategoryPrefix)
	if err != nil {
		return fmt.Errorf("loading categories: %w", err)
	}

	s.data = Data{
		Transactions: txs,
		Quotas:       quotas,
		Categories:   categories,
	}
	return nil
}

// loadCollection fetches every ID in idxKey's Set, then MGETs the actual
// entity JSON for each one and unmarshals it into T. Generic so it works
// identically for Transaction, Quota, and Category.
func loadCollection[T any](ctx context.Context, rdb *redis.Client, idxKey, prefix string) ([]T, error) {
	ids, err := rdb.SMembers(ctx, idxKey).Result()
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = prefix + id
	}

	vals, err := rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	result := make([]T, 0, len(vals))
	for _, v := range vals {
		if v == nil {
			// Index pointed at a key that no longer exists (e.g. a crash
			// between deleting the entity and updating the index). Skip
			// it rather than fail the whole load.
			continue
		}
		str, ok := v.(string)
		if !ok {
			continue
		}
		var item T
		if err := json.Unmarshal([]byte(str), &item); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

// saveLocked writes the current in-memory Data back to Redis, one entity
// per key, and keeps each idx:* Set in sync — including removing keys for
// entities that were deleted from the in-memory slice since the last save.
// The whole thing runs inside a single Redis transaction (TxPipeline) so
// a concurrent reader never sees a half-updated state.
func (s *Store) saveLocked() error {
	pipe := s.rdb.TxPipeline()

	if err := syncCollection(s.ctx, s.rdb, pipe, idxTransactions, keyTxPrefix,
		s.data.Transactions, func(tx models.Transaction) string { return tx.ID }); err != nil {
		return fmt.Errorf("syncing transactions: %w", err)
	}
	if err := syncCollection(s.ctx, s.rdb, pipe, idxQuotas, keyQuotaPrefix,
		s.data.Quotas, func(q models.Quota) string { return q.ID }); err != nil {
		return fmt.Errorf("syncing quotas: %w", err)
	}
	if err := syncCollection(s.ctx, s.rdb, pipe, idxCategories, keyCategoryPrefix,
		s.data.Categories, func(c models.Category) string { return c.ID }); err != nil {
		return fmt.Errorf("syncing categories: %w", err)
	}

	if _, err := pipe.Exec(s.ctx); err != nil {
		return fmt.Errorf("writing data to redis: %w", err)
	}
	return nil
}

// syncCollection diffs the entity IDs currently in Redis's idx:* Set
// against the IDs present in `items`, then queues (on pipe): a SET for
// every current item, a DEL for every entity key that was removed, and a
// full replace of the idx:* Set so it matches exactly.
func syncCollection[T any](
	ctx context.Context,
	rdb *redis.Client,
	pipe redis.Pipeliner,
	idxKey, prefix string,
	items []T,
	idOf func(T) string,
) error {
	oldIDs, err := rdb.SMembers(ctx, idxKey).Result()
	if err != nil {
		return err
	}
	oldSet := make(map[string]bool, len(oldIDs))
	for _, id := range oldIDs {
		oldSet[id] = true
	}

	newIDs := make([]string, 0, len(items))
	for _, item := range items {
		id := idOf(item)
		newIDs = append(newIDs, id)
		delete(oldSet, id) // whatever's left in oldSet after this loop = removed entities

		b, err := json.Marshal(item)
		if err != nil {
			return err
		}
		pipe.Set(ctx, prefix+id, b, 0)
	}

	// Delete keys for entities no longer present.
	for removedID := range oldSet {
		pipe.Del(ctx, prefix+removedID)
	}

	// Replace the index wholesale so it exactly matches newIDs.
	pipe.Del(ctx, idxKey)
	if len(newIDs) > 0 {
		members := make([]interface{}, len(newIDs))
		for i, id := range newIDs {
			members[i] = id
		}
		pipe.SAdd(ctx, idxKey, members...)
	}
	return nil
}

// View gives read-only access to the current in-memory data.
func (s *Store) View(fn func(Data)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	fn(s.data)
}

// Update gives exclusive access, applies fn, then persists to Redis.
func (s *Store) Update(fn func(*Data) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := fn(&s.data); err != nil {
		return err
	}
	return s.saveLocked()
}
