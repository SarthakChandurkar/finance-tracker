package storage

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"sync"

	"github.com/redis/go-redis/v9"

	"financetracker/internal/models"
)

func NewRedisOptions() *redis.Options {
	db, _ := strconv.Atoi(os.Getenv("REDIS_DB"))

	opts := &redis.Options{
		Addr:     os.Getenv("REDIS_ADDR"),
		Username: os.Getenv("REDIS_USER"),
		Password: os.Getenv("REDIS_PASSWORD"),
		DB:       db,
	}

	return opts
}

type Data struct {
	Transactions []models.Transaction `json:"transactions"`
	Wallets      []models.Wallet      `json:"wallets"`
	Categories   []models.Category    `json:"categories"`
	Users        []models.User        `json:"users"`
}

const (
	keyTxPrefix       = "tx:"
	keyWalletPrefix   = "wallet:"
	keyCategoryPrefix = "category:"
	keyUserPrefix     = "user:"

	idxTransactions = "idx:transactions"
	idxWallets      = "idx:wallets"
	idxCategories   = "idx:categories"
	idxUsers        = "idx:users"
)

type Identifiable interface {
	GetID() string
}

type entityCollection[T Identifiable] struct {
	idxKey string
	prefix string
}

var (
	txCollection       = entityCollection[models.Transaction]{idxKey: idxTransactions, prefix: keyTxPrefix}
	walletCollection   = entityCollection[models.Wallet]{idxKey: idxWallets, prefix: keyWalletPrefix}
	categoryCollection = entityCollection[models.Category]{idxKey: idxCategories, prefix: keyCategoryPrefix}
	userCollection     = entityCollection[models.User]{idxKey: idxUsers, prefix: keyUserPrefix}
)

func (c entityCollection[T]) load(ctx context.Context, rdb *redis.Client) ([]T, error) {
	ids, err := rdb.SMembers(ctx, c.idxKey).Result()
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, nil
	}

	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = c.prefix + id
	}

	vals, err := rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}

	result := make([]T, 0, len(vals))
	for _, v := range vals {
		if v == nil {
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

func (c entityCollection[T]) queueSync(ctx context.Context, rdb *redis.Client, pipe redis.Pipeliner, items []T) error {
	oldIDs, err := rdb.SMembers(ctx, c.idxKey).Result()
	if err != nil {
		return err
	}
	oldSet := make(map[string]bool, len(oldIDs))
	for _, id := range oldIDs {
		oldSet[id] = true
	}

	newIDs := make([]string, 0, len(items))
	for _, item := range items {
		id := item.GetID()
		newIDs = append(newIDs, id)
		delete(oldSet, id)

		b, err := json.Marshal(item)
		if err != nil {
			return err
		}
		pipe.Set(ctx, c.prefix+id, b, 0)
	}

	for removedID := range oldSet {
		pipe.Del(ctx, c.prefix+removedID)
	}

	pipe.Del(ctx, c.idxKey)
	if len(newIDs) > 0 {
		members := make([]interface{}, len(newIDs))
		for i, id := range newIDs {
			members[i] = id
		}
		pipe.SAdd(ctx, c.idxKey, members...)
	}
	return nil
}

type Store struct {
	mu   sync.RWMutex
	rdb  *redis.Client
	ctx  context.Context
	data Data
}

func NewStore(rdb *redis.Client) *Store {
	return &Store{
		rdb: rdb,
		ctx: context.Background(),
	}
}

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	exists, err := s.rdb.Exists(s.ctx, idxWallets, idxCategories).Result()
	if err != nil {
		return fmt.Errorf("checking redis for existing data: %w", err)
	}
	if exists == 0 {
		before, err := snapshotJSON(Data{})
		if err != nil {
			return err
		}
		s.data = Data{
			Wallets:    []models.Wallet{models.NewSavingsWallet()},
			Categories: []models.Category{models.NewSettledCategory()},
		}
		return s.saveChanged(before)
	}

	txs, err := txCollection.load(s.ctx, s.rdb)
	if err != nil {
		return fmt.Errorf("loading transactions: %w", err)
	}
	wallets, err := walletCollection.load(s.ctx, s.rdb)
	if err != nil {
		return fmt.Errorf("loading wallets: %w", err)
	}
	categories, err := categoryCollection.load(s.ctx, s.rdb)
	if err != nil {
		return fmt.Errorf("loading categories: %w", err)
	}
	users, err := userCollection.load(s.ctx, s.rdb)
	if err != nil {
		return fmt.Errorf("loading users: %w", err)
	}

	s.data = Data{
		Transactions: txs,
		Wallets:      wallets,
		Categories:   categories,
		Users:        users,
	}
	return nil
}

type dataSnapshot struct {
	transactions []byte
	wallets      []byte
	categories   []byte
	users        []byte
}

func snapshotJSON(d Data) (dataSnapshot, error) {
	var snap dataSnapshot
	var err error
	if snap.transactions, err = json.Marshal(d.Transactions); err != nil {
		return snap, fmt.Errorf("snapshotting transactions: %w", err)
	}
	if snap.wallets, err = json.Marshal(d.Wallets); err != nil {
		return snap, fmt.Errorf("snapshotting wallets: %w", err)
	}
	if snap.categories, err = json.Marshal(d.Categories); err != nil {
		return snap, fmt.Errorf("snapshotting categories: %w", err)
	}
	if snap.users, err = json.Marshal(d.Users); err != nil {
		return snap, fmt.Errorf("snapshotting users: %w", err)
	}
	return snap, nil
}

func queueIfChanged[T Identifiable](
	ctx context.Context,
	rdb *redis.Client,
	pipe redis.Pipeliner,
	c entityCollection[T],
	beforeJSON []byte,
	items []T,
) (bool, error) {
	afterJSON, err := json.Marshal(items)
	if err != nil {
		return false, err
	}
	if bytes.Equal(beforeJSON, afterJSON) {
		return false, nil
	}
	if err := c.queueSync(ctx, rdb, pipe, items); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) saveChanged(before dataSnapshot) error {
	pipe := s.rdb.TxPipeline()
	touched := false

	if changed, err := queueIfChanged(s.ctx, s.rdb, pipe, txCollection, before.transactions, s.data.Transactions); err != nil {
		return fmt.Errorf("syncing transactions: %w", err)
	} else if changed {
		touched = true
	}
	if changed, err := queueIfChanged(s.ctx, s.rdb, pipe, walletCollection, before.wallets, s.data.Wallets); err != nil {
		return fmt.Errorf("syncing wallets: %w", err)
	} else if changed {
		touched = true
	}
	if changed, err := queueIfChanged(s.ctx, s.rdb, pipe, categoryCollection, before.categories, s.data.Categories); err != nil {
		return fmt.Errorf("syncing categories: %w", err)
	} else if changed {
		touched = true
	}
	if changed, err := queueIfChanged(s.ctx, s.rdb, pipe, userCollection, before.users, s.data.Users); err != nil {
		return fmt.Errorf("syncing users: %w", err)
	} else if changed {
		touched = true
	}

	if !touched {
		return nil
	}
	if _, err := pipe.Exec(s.ctx); err != nil {
		return fmt.Errorf("writing data to redis: %w", err)
	}
	return nil
}

func (s *Store) View(fn func(Data)) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	fn(s.data)
}

func (s *Store) Update(fn func(*Data) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	before, err := snapshotJSON(s.data)
	if err != nil {
		return fmt.Errorf("snapshotting before update: %w", err)
	}

	if err := fn(&s.data); err != nil {
		return err
	}

	return s.saveChanged(before)
}
