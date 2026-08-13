package models

import "errors"

const SettledCategoryID = "settled"

const SettledCategoryName = "Settled"

type Category struct {
	ID     string `json:"id"`
	UserID string `json:"user_id"`
	Name   string `json:"name"`
}

func NewSettledCategory() Category {
	return Category{
		ID:   SettledCategoryID,
		Name: SettledCategoryName,
	}
}

func (c Category) Validate() error {
	if c.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

func (c Category) GetID() string {
	return c.ID
}

// OwnerID identifies which user this category belongs to. Storage uses
// this so two users' default "Settled" categories (same literal ID) never
// collide on the same Redis key.
func (c Category) OwnerID() string {
	return c.UserID
}
