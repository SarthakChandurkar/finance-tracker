package storage

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/redis/go-redis/v9"
)

const sessionKeyPrefix = "session:"

// SessionStore manages short-lived login sessions in Redis. Sessions are
// kept separate from the Store's Data model on purpose: they are
// ephemeral, keyed by an opaque token, and expire on their own via a
// Redis TTL instead of participating in the load/save snapshot cycle used
// for transactions/wallets/categories/users.
type SessionStore struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewSessionStore(rdb *redis.Client, ttl time.Duration) *SessionStore {
	return &SessionStore{rdb: rdb, ttl: ttl}
}

// Create starts a new session for userID and returns the opaque token
// that should be stored in the client's session cookie.
func (s *SessionStore) Create(ctx context.Context, userID string) (string, error) {
	token, err := newSessionToken()
	if err != nil {
		return "", err
	}
	if err := s.rdb.Set(ctx, sessionKeyPrefix+token, userID, s.ttl).Err(); err != nil {
		return "", err
	}
	return token, nil
}

// UserID returns the user ID tied to token, if the session is still
// valid. Any successful lookup counts as activity and slides the idle
// expiry window forward by ttl - this is what gives the "log in again
// only after being idle" behavior. ok is false if the token is empty,
// unknown, or has already expired.
func (s *SessionStore) UserID(ctx context.Context, token string) (userID string, ok bool, err error) {
	if token == "" {
		return "", false, nil
	}
	val, err := s.rdb.Get(ctx, sessionKeyPrefix+token).Result()
	if err == redis.Nil {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	if err := s.rdb.Expire(ctx, sessionKeyPrefix+token, s.ttl).Err(); err != nil {
		return "", false, err
	}
	return val, true, nil
}

// Destroy ends a session immediately (logout).
func (s *SessionStore) Destroy(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	return s.rdb.Del(ctx, sessionKeyPrefix+token).Err()
}

func newSessionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
