package catalog

import "fmt"

// SKUView is a SKU in query results.
type SKUView struct {
	ID     string            `json:"id"`
	Values map[string]string `json:"values"`
	Price  int               `json:"price"`
	Stock  int               `json:"stock"`
}

// QueryResult contains sellable SKUs and, for every dimension, the values
// still selectable while honoring the other dimensions' filters.
type QueryResult struct {
	SKUs      []SKUView           `json:"skus"`
	Available map[string][]string `json:"available"`
}

// Query returns active, in-stock SKUs of a product matching all selected
// dimension values. Unknown dimensions or values are errors.
func (c *Catalog) Query(productID string, selected map[string]string) (*QueryResult, error) {
	p, err := c.product(productID)
	if err != nil {
		return nil, err
	}

	dimByName := map[string]*Dimension{}
	for _, d := range p.Dimensions {
		dimByName[d.Name] = d
	}
	for name, val := range selected {
		d, ok := dimByName[name]
		if !ok {
			return nil, fmt.Errorf("product %q: unknown dimension %q", productID, name)
		}
		if !contains(d.Values, val) {
			return nil, fmt.Errorf("product %q dimension %q: unknown value %q", productID, name, val)
		}
	}

	// sellable: active, stock > 0, matches every selected dimension.
	sellable := []*SKU{}
	for _, id := range sortedSKUIDs(p) {
		sku := p.SKUs[id]
		if !sku.Active || sku.Stock <= 0 {
			continue
		}
		matches := true
		for name, val := range selected {
			if sku.Values[name] != val {
				matches = false
				break
			}
		}
		if matches {
			sellable = append(sellable, sku)
		}
	}

	views := make([]SKUView, 0, len(sellable))
	for _, sku := range sellable {
		views = append(views, SKUView{
			ID:     sku.ID,
			Values: sku.Values,
			Price:  sku.Price,
			Stock:  sku.Stock,
		})
	}

	available := map[string][]string{}
	for _, d := range p.Dimensions {
		// For this dimension, keep the other dimensions' selections and
		// ignore this dimension's own selection.
		others := map[string]string{}
		for name, val := range selected {
			if name != d.Name {
				others[name] = val
			}
		}
		optionSet := map[string]bool{}
		for _, sku := range sellableAll(p) {
			ok := true
			for name, val := range others {
				if sku.Values[name] != val {
					ok = false
					break
				}
			}
			if ok {
				optionSet[sku.Values[d.Name]] = true
			}
		}
		options := make([]string, 0, len(optionSet))
		for _, v := range d.Values {
			if optionSet[v] {
				options = append(options, v)
			}
		}
		available[d.Name] = options
	}

	return &QueryResult{SKUs: views, Available: available}, nil
}

// sellableAll lists active, in-stock SKUs regardless of selection.
func sellableAll(p *Product) []*SKU {
	out := []*SKU{}
	for _, id := range sortedSKUIDs(p) {
		sku := p.SKUs[id]
		if sku.Active && sku.Stock > 0 {
			out = append(out, sku)
		}
	}
	return out
}
