package catalog

import "fmt"

// SetActive lists or delists a single SKU. Inactive SKUs keep their id and
// dimension combination.
func (c *Catalog) SetActive(skuID string, active bool) error {
	pid, ok := c.SKUIndex[skuID]
	if !ok {
		return fmt.Errorf("sku %q not found", skuID)
	}
	c.Products[pid].SKUs[skuID].Active = active
	return nil
}

// SKUUpdate is one price/stock change.
type SKUUpdate struct {
	ID    string `json:"id"`
	Price *int   `json:"price,omitempty"`
	Stock *int   `json:"stock,omitempty"`
}

// BatchUpdate applies price/stock updates atomically: if any item is invalid,
// the whole batch is rejected and nothing changes.
func (c *Catalog) BatchUpdate(updates []SKUUpdate) ([]string, error) {
	if len(updates) == 0 {
		return nil, fmt.Errorf("update batch is empty")
	}

	type change struct {
		sku   *SKU
		price int
		stock int
	}
	changes := make([]change, 0, len(updates))
	seen := map[string]bool{}

	for i, u := range updates {
		loc := func(msg string) error {
			if u.ID == "" {
				return fmt.Errorf("update batch item %d: %s", i, msg)
			}
			return fmt.Errorf("sku %q: %s", u.ID, msg)
		}
		if u.ID == "" {
			return nil, loc("sku id must not be empty")
		}
		if seen[u.ID] {
			return nil, loc("duplicate sku id within batch")
		}
		pid, ok := c.SKUIndex[u.ID]
		if !ok {
			return nil, loc("sku not found")
		}
		sku := c.Products[pid].SKUs[u.ID]
		price, stock := sku.Price, sku.Stock
		if u.Price != nil {
			if *u.Price < 0 {
				return nil, loc("price must be a non-negative integer")
			}
			price = *u.Price
		}
		if u.Stock != nil {
			if *u.Stock < 0 {
				return nil, loc("stock must be a non-negative integer")
			}
			stock = *u.Stock
		}
		seen[u.ID] = true
		changes = append(changes, change{sku: sku, price: price, stock: stock})
	}

	ids := make([]string, 0, len(changes))
	for _, ch := range changes {
		ch.sku.Price = ch.price
		ch.sku.Stock = ch.stock
		ids = append(ids, ch.sku.ID)
	}
	return ids, nil
}
