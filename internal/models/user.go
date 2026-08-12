package models

import "time"

// User represents a registered account used purely to gate access.
// There is no per-user data isolation in this app: every authenticated
// user sees the same shared transactions/wallets/categories.
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"password_hash"`
	CreatedAt    time.Time `json:"created_at"`
}

func (u User) GetID() string {
	return u.ID
}
