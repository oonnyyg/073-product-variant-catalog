package catalog

import (
	"fmt"
	"sort"
)

// QueryResultItem is one sellable SKU returned by a query.
type QueryResultItem struct {
	ID    string            `json:"id"`
	Attrs map[string]string `json:"attrs"`
	Price int               `json:"price"`
	Stock int               `json:"stock"`
}

// QueryResult contains matching sellable SKUs and available options per
// dimension (only values actually supported by sellable SKUs).
type QueryResult struct {
	SKUs    []QueryResultItem   `json:"skus"`
	Options map[string][]string `json:"options"`
}

// Query returns listed, in-stock SKUs of a product matching every selected
// dimension value. Empty filter means all sellable combinations. Unknown
// dimensions or values are errors.
func (c *Catalog) Query(productID string, filter map[string]string) (*QueryResult, error) {
	product, ok := c.products[productID]
	if !ok {
		return nil, fmt.Errorf("unknown product %q", productID)
	}

	legal := map[string]map[string]bool{}
	for _, d := range product.Dimensions {
		vals := map[string]bool{}
		for _, v := range d.Values {
			vals[v] = true
		}
		legal[d.Name] = vals
	}
	for dim, val := range filter {
		vals, known := legal[dim]
		if !known {
			return nil, fmt.Errorf("product %q has no dimension %q", productID, dim)
		}
		if !vals[val] {
			return nil, fmt.Errorf("dimension %q has no value %q", dim, val)
		}
	}

	var all []*SKU
	for _, s := range c.skus {
		if s.Product != productID || !s.Listed || s.Stock <= 0 {
			continue
		}
		match := true
		for dim, val := range filter {
			if s.Attrs[dim] != val {
				match = false
				break
			}
		}
		if match {
			all = append(all, s)
		}
	}

	sort.Slice(all, func(i, j int) bool { return all[i].ID < all[j].ID })

	result := &QueryResult{
		SKUs:    []QueryResultItem{},
		Options: map[string][]string{},
	}
	for _, s := range all {
		result.SKUs = append(result.SKUs, QueryResultItem{
			ID:    s.ID,
			Attrs: cloneAttrs(s.Attrs),
			Price: s.Price,
			Stock: s.Stock,
		})
	}

	// Options for each dimension: keep filters on the OTHER dimensions,
	// ignore this dimension's own selection, and collect values that appear
	// on sellable SKUs only.
	for _, d := range product.Dimensions {
		available := map[string]bool{}
		for _, s := range c.skus {
			if s.Product != productID || !s.Listed || s.Stock <= 0 {
				continue
			}
			ok := true
			for dim, val := range filter {
				if dim == d.Name {
					continue
				}
				if s.Attrs[dim] != val {
					ok = false
					break
				}
			}
			if ok {
				available[s.Attrs[d.Name]] = true
			}
		}
		values := make([]string, 0, len(available))
		for v := range available {
			values = append(values, v)
		}
		sort.Strings(values)
		result.Options[d.Name] = values
	}

	return result, nil
}
