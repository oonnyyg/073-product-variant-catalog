package catalog

import "fmt"

// SetListed marks a SKU listed (true) or delisted (false).
// A delisted SKU keeps its ID and occupied combination.
func (c *Catalog) SetListed(skuID string, listed bool) error {
	s, ok := c.skus[skuID]
	if !ok {
		return fmt.Errorf("unknown sku %q", skuID)
	}
	s.Listed = listed
	return nil
}

// AttrUpdate changes price and/or stock of one SKU.
type AttrUpdate struct {
	SKU   string `json:"sku"`
	Price *int   `json:"price,omitempty"`
	Stock *int   `json:"stock,omitempty"`
}

// UpdateSKUs applies price/stock updates atomically: if any item is invalid
// (unknown SKU or negative value) nothing changes.
func (c *Catalog) UpdateSKUs(updates []AttrUpdate) error {
	type planned struct {
		sku   *SKU
		price int
		stock int
	}
	plans := make([]planned, 0, len(updates))
	for _, u := range updates {
		s, ok := c.skus[u.SKU]
		if !ok {
			return fmt.Errorf("unknown sku %q", u.SKU)
		}
		price, stock := s.Price, s.Stock
		if u.Price != nil {
			if *u.Price < 0 {
				return fmt.Errorf("sku %q price must be non-negative", u.SKU)
			}
			price = *u.Price
		}
		if u.Stock != nil {
			if *u.Stock < 0 {
				return fmt.Errorf("sku %q stock must be non-negative", u.SKU)
			}
			stock = *u.Stock
		}
		plans = append(plans, planned{sku: s, price: price, stock: stock})
	}
	for _, p := range plans {
		p.sku.Price = p.price
		p.sku.Stock = p.stock
	}
	return nil
}
