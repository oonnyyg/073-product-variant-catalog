package catalog

import (
	"fmt"
	"sort"
)

// SKUSpec is the input for adding one SKU.
type SKUSpec struct {
	ID     string            `json:"id"`
	Values map[string]string `json:"values"`
	Price  int               `json:"price"`
	Stock  int               `json:"stock"`
}

// AddSKUs adds a batch of SKUs to a product atomically: if any item is
// invalid, none of them are added.
func (c *Catalog) AddSKUs(productID string, specs []SKUSpec) ([]string, error) {
	p, err := c.product(productID)
	if err != nil {
		return nil, err
	}
	if len(specs) == 0 {
		return nil, fmt.Errorf("product %q: sku batch is empty", productID)
	}

	dimByName := map[string]*Dimension{}
	for _, d := range p.Dimensions {
		dimByName[d.Name] = d
	}

	// Validate everything first so failures leave state untouched.
	built := make([]*SKU, 0, len(specs))
	batchIDs := map[string]bool{}
	batchCombos := map[string]bool{}

	for i, spec := range specs {
		loc := func(msg string) error {
			id := spec.ID
			if id == "" {
				return fmt.Errorf("product %q batch item %d: %s", productID, i, msg)
			}
			return fmt.Errorf("sku %q: %s", id, msg)
		}
		if spec.ID == "" {
			return nil, loc("sku id must not be empty")
		}
		if batchIDs[spec.ID] {
			return nil, loc("duplicate sku id within batch")
		}
		if _, exists := c.SKUIndex[spec.ID]; exists {
			return nil, loc("sku id already exists")
		}
		if spec.Price < 0 {
			return nil, loc("price must be a non-negative integer")
		}
		if spec.Stock < 0 {
			return nil, loc("stock must be a non-negative integer")
		}
		if len(spec.Values) != len(p.Dimensions) {
			return nil, loc(fmt.Sprintf("must select exactly one value for each of %d dimensions", len(p.Dimensions)))
		}

		// Assign values in canonical dimension order.
		values := make(map[string]string, len(p.Dimensions))
		for _, d := range p.Dimensions {
			v, ok := spec.Values[d.Name]
			if !ok {
				return nil, loc(fmt.Sprintf("missing value for dimension %q", d.Name))
			}
			if !contains(d.Values, v) {
				return nil, loc(fmt.Sprintf("value %q is not valid for dimension %q", v, d.Name))
			}
			values[d.Name] = v
		}
		// Reject unknown dimensions (len check above guarantees none when
		// counts match, but guard explicitly against extra keys).
		for name := range spec.Values {
			if _, ok := dimByName[name]; !ok {
				return nil, loc(fmt.Sprintf("unknown dimension %q", name))
			}
		}

		key := comboKey(values, p.Dimensions)
		if batchCombos[key] {
			return nil, loc("duplicate dimension combination within batch")
		}
		if existing := c.skuByCombo(p, values); existing != nil {
			return nil, loc(fmt.Sprintf("dimension combination already taken by sku %q", existing.ID))
		}

		batchIDs[spec.ID] = true
		batchCombos[key] = true
		built = append(built, &SKU{
			ID:      spec.ID,
			Product: productID,
			Values:  values,
			Price:   spec.Price,
			Stock:   spec.Stock,
			Active:  true,
		})
	}

	// Commit only after the whole batch validates.
	ids := make([]string, 0, len(built))
	for _, sku := range built {
		p.SKUs[sku.ID] = sku
		c.SKUIndex[sku.ID] = productID
		ids = append(ids, sku.ID)
	}
	sort.Strings(ids)
	return ids, nil
}

// skuByCombo returns an existing SKU of p with the same dimension values,
// regardless of key order. Inactive SKUs still occupy their combination.
func (c *Catalog) skuByCombo(p *Product, values map[string]string) *SKU {
	target := comboKey(values, p.Dimensions)
	for _, sku := range p.SKUs {
		if comboKey(sku.Values, p.Dimensions) == target {
			return sku
		}
	}
	return nil
}

// comboKey builds an order-independent key for a dimension-value assignment.
func comboKey(values map[string]string, dims []*Dimension) string {
	parts := make([]string, 0, len(dims))
	for _, d := range dims {
		parts = append(parts, d.Name+"="+values[d.Name])
	}
	sort.Strings(parts)
	key := ""
	for _, pt := range parts {
		key += pt + "\x00"
	}
	return key
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
