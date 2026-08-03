package models

import "errors"

const SettledCategoryID = "settled"

const SettledCategoryName = "Settled"

type Category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
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
