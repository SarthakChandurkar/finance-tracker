package domain

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode"

	"golang.org/x/crypto/bcrypt"

	"financetracker/internal/models"
	"financetracker/internal/storage"
)

var (
	ErrUsernameTaken      = errors.New("username is already taken")
	ErrUsernameRequired   = errors.New("username is required")
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrUserNotFound       = errors.New("user not found")
)

const minPasswordLength = 8

// PasswordPolicyError explains exactly which rule a candidate password
// failed, so a register form can surface a specific, actionable message.
type PasswordPolicyError struct {
	Reason string
}

func (e *PasswordPolicyError) Error() string {
	return e.Reason
}

// AuthService owns registration, login and logout. Passwords are hashed
// with bcrypt (a one-way function) - the app never stores or has access
// to a reversible/decryptable copy of a password, which is intentional:
// that's the industry-standard, secure way to handle credentials.
type AuthService struct {
	store    *storage.Store
	sessions *storage.SessionStore
}

func NewAuthService(store *storage.Store, sessions *storage.SessionStore) *AuthService {
	return &AuthService{store: store, sessions: sessions}
}

// Register creates a new account. Username must be unique
// (case-insensitive); password must satisfy ValidatePassword.
func (a *AuthService) Register(ctx context.Context, username, password string) (models.User, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return models.User{}, ErrUsernameRequired
	}
	if err := ValidatePassword(password); err != nil {
		return models.User{}, err
	}

	var created models.User
	err := a.store.Update(func(d *storage.Data) error {
		lower := strings.ToLower(username)
		for _, u := range d.Users {
			if strings.ToLower(u.Username) == lower {
				return ErrUsernameTaken
			}
		}

		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}

		id, err := newUserID()
		if err != nil {
			return err
		}

		created = models.User{
			ID:           id,
			Username:     username,
			PasswordHash: string(hash),
			CreatedAt:    time.Now().UTC(),
		}
		d.Users = append(d.Users, created)

		// Every user gets their own starting Savings wallet and Settled
		// category, matching what used to be seeded once globally.
		// Both intentionally reuse the same literal ID across users
		// ("savings" / "settled") - storage.go namespaces the Redis key
		// by owner precisely so that's safe.
		defaultWallet := models.NewSavingsWallet()
		defaultWallet.UserID = created.ID
		d.Wallets = append(d.Wallets, defaultWallet)

		defaultCategory := models.NewSettledCategory()
		defaultCategory.UserID = created.ID
		d.Categories = append(d.Categories, defaultCategory)

		return nil
	})
	if err != nil {
		return models.User{}, err
	}
	return created, nil
}

// Login verifies credentials and, on success, starts a new session and
// returns its opaque token for the caller to set as a cookie.
func (a *AuthService) Login(ctx context.Context, username, password string) (token string, err error) {
	username = strings.TrimSpace(username)

	var match *models.User
	a.store.View(func(d storage.Data) {
		lower := strings.ToLower(username)
		for i := range d.Users {
			if strings.ToLower(d.Users[i].Username) == lower {
				match = &d.Users[i]
				return
			}
		}
	})
	if match == nil {
		return "", ErrInvalidCredentials
	}
	if err := bcrypt.CompareHashAndPassword([]byte(match.PasswordHash), []byte(password)); err != nil {
		return "", ErrInvalidCredentials
	}

	return a.sessions.Create(ctx, match.ID)
}

// Logout ends the session tied to token. Safe to call with an empty or
// already-expired token.
func (a *AuthService) Logout(ctx context.Context, token string) error {
	return a.sessions.Destroy(ctx, token)
}

// CurrentUser returns the ID of the user tied to a session token, if the
// session is still valid. This also slides the session's idle-expiry
// window forward, since a lookup means the user is active right now.
func (a *AuthService) CurrentUser(ctx context.Context, token string) (userID string, ok bool) {
	userID, ok, err := a.sessions.UserID(ctx, token)
	if err != nil {
		return "", false
	}
	return userID, ok
}

// GetUser returns the account tied to userID, if it still exists.
func (a *AuthService) GetUser(userID string) (models.User, bool) {
	var found models.User
	var ok bool
	a.store.View(func(d storage.Data) {
		for _, u := range d.Users {
			if u.ID == userID {
				found, ok = u, true
				return
			}
		}
	})
	return found, ok
}

// DeleteAccount permanently removes userID's account along with every
// wallet, category, and transaction they own, then ends the session tied
// to token. This is irreversible - there is no per-user data left behind
// for anyone else to see once this returns.
func (a *AuthService) DeleteAccount(ctx context.Context, userID, token string) error {
	err := a.store.Update(func(d *storage.Data) error {
		users := make([]models.User, 0, len(d.Users))
		found := false
		for _, u := range d.Users {
			if u.ID == userID {
				found = true
				continue
			}
			users = append(users, u)
		}
		if !found {
			return ErrUserNotFound
		}
		d.Users = users

		wallets := make([]models.Wallet, 0, len(d.Wallets))
		for _, w := range d.Wallets {
			if w.UserID != userID {
				wallets = append(wallets, w)
			}
		}
		d.Wallets = wallets

		categories := make([]models.Category, 0, len(d.Categories))
		for _, c := range d.Categories {
			if c.UserID != userID {
				categories = append(categories, c)
			}
		}
		d.Categories = categories

		transactions := make([]models.Transaction, 0, len(d.Transactions))
		for _, tx := range d.Transactions {
			if tx.UserID != userID {
				transactions = append(transactions, tx)
			}
		}
		d.Transactions = transactions

		return nil
	})
	if err != nil {
		return err
	}
	return a.sessions.Destroy(ctx, token)
}

func newUserID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// ValidatePassword enforces: at least 8 characters; at least one
// uppercase letter, one lowercase letter, one digit, and one punctuation
// character; and no run of 3+ sequential letters or digits (e.g. "abc",
// "bcd", "123", "cba", "987").
func ValidatePassword(pw string) error {
	if len(pw) < minPasswordLength {
		return &PasswordPolicyError{Reason: "password must be at least 8 characters long"}
	}

	var hasUpper, hasLower, hasDigit, hasPunct bool
	for _, r := range pw {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsDigit(r):
			hasDigit = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasPunct = true
		}
	}
	switch {
	case !hasUpper:
		return &PasswordPolicyError{Reason: "password must contain at least one uppercase letter"}
	case !hasLower:
		return &PasswordPolicyError{Reason: "password must contain at least one lowercase letter"}
	case !hasDigit:
		return &PasswordPolicyError{Reason: "password must contain at least one number"}
	case !hasPunct:
		return &PasswordPolicyError{Reason: "password must contain at least one punctuation character"}
	}

	if hasSequentialRun(pw, 3) {
		return &PasswordPolicyError{Reason: `password must not contain a run of 3+ sequential letters or numbers (e.g. "abc", "123")`}
	}
	return nil
}

// hasSequentialRun reports whether pw contains `run` or more consecutive
// characters that step by exactly +1 or -1 in code point order, checked
// within letters (case-insensitive) and digits separately so e.g. "9a" is
// never treated as a sequential pair.
func hasSequentialRun(pw string, run int) bool {
	norm := []rune(strings.ToLower(pw))

	asc, desc := 1, 1
	for i := 1; i < len(norm); i++ {
		prev, cur := norm[i-1], norm[i]
		sameClass := (unicode.IsDigit(prev) && unicode.IsDigit(cur)) ||
			(unicode.IsLetter(prev) && unicode.IsLetter(cur))

		if sameClass && cur-prev == 1 {
			asc++
		} else {
			asc = 1
		}
		if sameClass && prev-cur == 1 {
			desc++
		} else {
			desc = 1
		}

		if asc >= run || desc >= run {
			return true
		}
	}
	return false
}
