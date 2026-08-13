package storage

import "financetracker/internal/models"

// DataStore is the read/update contract every domain service depends on.
// *Store satisfies it directly (unfiltered, whole-dataset access - used
// only by AuthService, which legitimately needs to see all users).
// UserScopedStore also satisfies it, presenting one user's own records as
// if they were the whole dataset. Domain services (WalletService,
// TransactionService, CategoryService, LoanService, ScheduleService) are
// written against this interface, so which implementation they get
// decides whose data they can see - their own logic never changes.
type DataStore interface {
	View(fn func(Data))
	Update(fn func(*Data) error) error
}

// UserScopedStore wraps a *Store and restricts it to a single user's
// Transactions, Wallets, and Categories. Reads are filtered to that user;
// writes are merged back without ever touching another user's records.
// Users is intentionally never exposed here - nothing that goes through
// a UserScopedStore can see or modify the account/credential list.
type UserScopedStore struct {
	inner  *Store
	userID string
}

func NewUserScopedStore(inner *Store, userID string) *UserScopedStore {
	return &UserScopedStore{inner: inner, userID: userID}
}

func (u *UserScopedStore) View(fn func(Data)) {
	u.inner.View(func(full Data) {
		fn(u.filter(full))
	})
}

func (u *UserScopedStore) Update(mutate func(*Data) error) error {
	return u.inner.Update(func(full *Data) error {
		scoped := u.filter(*full)
		if err := mutate(&scoped); err != nil {
			return err
		}
		full.Transactions = mergeOwnedTransactions(full.Transactions, scoped.Transactions, u.userID)
		full.Wallets = mergeOwnedWallets(full.Wallets, scoped.Wallets, u.userID)
		full.Categories = mergeOwnedCategories(full.Categories, scoped.Categories, u.userID)
		return nil
	})
}

// filter returns a copy of full containing only this user's own
// Transactions/Wallets/Categories.
func (u *UserScopedStore) filter(full Data) Data {
	var scoped Data
	for _, tx := range full.Transactions {
		if tx.UserID == u.userID {
			scoped.Transactions = append(scoped.Transactions, tx)
		}
	}
	for _, w := range full.Wallets {
		if w.UserID == u.userID {
			scoped.Wallets = append(scoped.Wallets, w)
		}
	}
	for _, c := range full.Categories {
		if c.UserID == u.userID {
			scoped.Categories = append(scoped.Categories, c)
		}
	}
	return scoped
}

// mergeOwnedTransactions replaces this user's slice of transactions
// within the full (all-users) list with the post-mutation result,
// leaving every other user's transactions untouched. Anything in
// updated is (re-)stamped with userID, so a freshly created record is
// always correctly owned even if the mutating code never set UserID
// itself.
func mergeOwnedTransactions(all, updated []models.Transaction, userID string) []models.Transaction {
	kept := make([]models.Transaction, 0, len(all))
	for _, tx := range all {
		if tx.UserID != userID {
			kept = append(kept, tx)
		}
	}
	for _, tx := range updated {
		tx.UserID = userID
		kept = append(kept, tx)
	}
	return kept
}

func mergeOwnedWallets(all, updated []models.Wallet, userID string) []models.Wallet {
	kept := make([]models.Wallet, 0, len(all))
	for _, w := range all {
		if w.UserID != userID {
			kept = append(kept, w)
		}
	}
	for _, w := range updated {
		w.UserID = userID
		kept = append(kept, w)
	}
	return kept
}

func mergeOwnedCategories(all, updated []models.Category, userID string) []models.Category {
	kept := make([]models.Category, 0, len(all))
	for _, c := range all {
		if c.UserID != userID {
			kept = append(kept, c)
		}
	}
	for _, c := range updated {
		c.UserID = userID
		kept = append(kept, c)
	}
	return kept
}
