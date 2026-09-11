package domain

import (
	"financetracker/internal/models"
	"financetracker/internal/storage"
	"fmt"
	"sort"
	"time"
)

type CategoryService struct {
	store storage.DataStore
}

func NewCategoryService(store storage.DataStore) *CategoryService {
	return &CategoryService{store: store}
}

func (s *CategoryService) GetCategoryByID(id string) (models.Category, error) {
	var found bool
	var result models.Category

	s.store.View(func(d storage.Data) {
		for _, cat := range d.Categories {
			if cat.ID == id {
				found = true
				result = cat
				break
			}
		}
	})

	if !found {

		return models.Category{}, fmt.Errorf("category '%s' not found", id)
	}

	return result, nil
}

func (s *CategoryService) CreateCategory(name string) (models.Category, error) {
	if name == "" {
		return models.Category{}, fmt.Errorf("name is required")
	}
	category := models.Category{ID: newID(), Name: name}
	if err := category.Validate(); err != nil {
		return models.Category{}, err
	}
	if err := s.store.Update(func(d *storage.Data) error {
		for _, existing := range d.Categories {
			if existing.Name == name {
				return fmt.Errorf("category already exists: %s", name)
			}
		}
		d.Categories = append(d.Categories, category)
		return nil
	}); err != nil {
		return models.Category{}, err
	}
	return category, nil
}

func (s *CategoryService) RenameCategory(categoryID, newName string) (models.Category, error) {
	if categoryID == models.SettledCategoryID {
		return models.Category{}, fmt.Errorf("cannot rename the Settled category")
	}
	if newName == "" {
		return models.Category{}, fmt.Errorf("name is required")
	}
	var renamed models.Category
	err := s.store.Update(func(d *storage.Data) error {
		idx := -1
		for i, c := range d.Categories {
			if c.ID == categoryID {
				idx = i
				renamed = c
				break
			}
		}
		if idx == -1 {
			return fmt.Errorf("category not found: %s", categoryID)
		}
		for _, c := range d.Categories {
			if c.ID != categoryID && c.Name == newName {
				return fmt.Errorf("category already exists: %s", newName)
			}
		}
		oldName := renamed.Name
		renamed.Name = newName
		if err := renamed.Validate(); err != nil {
			return err
		}
		d.Categories[idx] = renamed
		for ti, tx := range d.Transactions {
			if tx.Category == oldName {
				tx.Category = newName
				d.Transactions[ti] = tx
			}
		}
		return nil
	})
	return renamed, err
}

func (s *CategoryService) DeleteCategory(categoryID string) (models.Category, error) {
	if categoryID == models.SettledCategoryID {
		return models.Category{}, fmt.Errorf("cannot delete the Settled category")
	}
	var deleted models.Category
	err := s.store.Update(func(d *storage.Data) error {
		idx := -1
		for i, c := range d.Categories {
			if c.ID == categoryID {
				idx = i
				deleted = c
				break
			}
		}
		if idx == -1 {
			return fmt.Errorf("category not found: %s", categoryID)
		}
		d.Categories = append(d.Categories[:idx], d.Categories[idx+1:]...)
		for ti, tx := range d.Transactions {
			if tx.Category == deleted.Name {
				tx.Category = models.SettledCategoryName
				d.Transactions[ti] = tx
			}
		}
		return nil
	})
	return deleted, err
}

type CategoryTotal struct {
	CategoryID   string  `json:"category_id"`
	CategoryName string  `json:"category_name"`
	Total        float64 `json:"total"`
}

func (s *CategoryService) MonthlyTotals(now time.Time) ([]CategoryTotal, error) {
	totals := map[string]float64{}
	categories := []models.Category{}
	s.store.View(func(d storage.Data) {
		categories = append(categories, d.Categories...)
		for _, tx := range d.Transactions {
			if tx.Type != models.Debit && tx.Type != models.Credit {
				continue
			}
			if tx.Date.Year() != now.Year() || tx.Date.Month() != now.Month() {
				continue
			}
			for _, c := range d.Categories {
				if tx.Category == c.Name && tx.Type == models.Debit {
					totals[c.ID] += tx.Amount
					break
				}
				if tx.Category == c.Name && tx.Type == models.Credit {
					totals[c.ID] -= tx.Amount
					break
				}
			}
		}
	})

	results := make([]CategoryTotal, 0, len(categories))
	for _, c := range categories {
		results = append(results, CategoryTotal{
			CategoryID:   c.ID,
			CategoryName: c.Name,
			Total:        totals[c.ID],
		})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Total > results[j].Total
	})
	return results, nil
}

func (s *CategoryService) GlobalTotals() ([]CategoryTotal, error) {
	totals := map[string]float64{}
	categories := []models.Category{}
	s.store.View(func(d storage.Data) {
		categories = append(categories, d.Categories...)
		for _, tx := range d.Transactions {
			if tx.Type != models.Debit && tx.Type != models.Credit {
				continue
			}
			for _, c := range d.Categories {
				if tx.Category == c.Name && tx.Type == models.Debit {
					totals[c.ID] += tx.Amount
					break
				}
				if tx.Category == c.Name && tx.Type == models.Credit {
					totals[c.ID] -= tx.Amount
					break
				}
			}
		}
	})

	results := make([]CategoryTotal, 0, len(categories))
	for _, c := range categories {
		results = append(results, CategoryTotal{
			CategoryID:   c.ID,
			CategoryName: c.Name,
			Total:        totals[c.ID],
		})
	}
	return results, nil
}

func (s *CategoryService) Recategorize(sourceCategoryID, destCategoryID string) error {
	if sourceCategoryID == destCategoryID {
		return fmt.Errorf("source and destination category must differ")
	}
	var sourceName, destName string
	err := s.store.Update(func(d *storage.Data) error {
		for _, c := range d.Categories {
			switch c.ID {
			case sourceCategoryID:
				sourceName = c.Name
			case destCategoryID:
				destName = c.Name
			}
		}
		if sourceName == "" {
			return fmt.Errorf("source category not found: %s", sourceCategoryID)
		}
		if destName == "" {
			return fmt.Errorf("destination category not found: %s", destCategoryID)
		}
		for ti, tx := range d.Transactions {
			if tx.Category == sourceName {
				tx.Category = destName
				d.Transactions[ti] = tx
			}
		}
		return nil
	})
	return err
}
