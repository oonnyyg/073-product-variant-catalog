package catalog

import (
	"fmt"
	"sort"
	"strings"
)

// SKUInput is one item of a batch-add request.
type SKUInput struct {
	ID    string            `json:"id"`
	Attrs map[string]string `json:"attrs"`
	Price int               `json:"price"`
	Stock int               `json:"stock"`
}

// comboKey is order-independent: attrs maps make ordering irrelevant,
// but we render deterministically for map keys and error messages.
func comboKey(attrs map[string]string, dimNames []string) string {
	parts := make([]string, 0, len(dimNames))
	for _, name := range dimNames {
		parts = append(parts, name+"="+attrs[name])
	}
	sort.Strings(parts)
	return strings.Join(parts, "&")
}

// AddSKUs adds SKUs for one product as an all-or-nothing batch.
// Duplicate IDs (globally or inside the batch) and conflicting combinations
// reject the whole batch.
func (c *Catalog) AddSKUs(productID string, inputs []SKUInput) error {
	product, ok := c.products[productID]
	if !ok {
		return fmt.Errorf("unknown product %q", productID)
	}

	dimNames := make([]string, 0, len(product.Dimensions))
	legal := make(map[string]map[string]bool, len(product.Dimensions))
	for _, d := range product.Dimensions {
		dimNames = append(dimNames, d.Name)
		vals := make(map[string]bool, len(d.Values))
		for _, v := range d.Values {
			vals[v] = true
		}
		legal[d.Name] = vals
	}

	// Existing combinations include delisted SKUs: they still occupy the combo.
	used := map[string]string{}
	for _, s := range c.skus {
		if s.Product == productID {
			used[comboKey(s.Attrs, dimNames)] = s.ID
		}
	}

	batchIDs := map[string]bool{}
	batchCombos := map[string]bool{}
	prepared := make([]*SKU, 0, len(inputs))

	for _, in := range inputs {
		if in.ID == "" {
			return fmt.Errorf("sku id must not be empty")
		}
		if _, exists := c.skus[in.ID]; exists {
			return fmt.Errorf("sku %q already exists", in.ID)
		}
		if batchIDs[in.ID] {
			return fmt.Errorf("sku %q duplicated in batch", in.ID)
		}
		batchIDs[in.ID] = true

		if len(in.Attrs) != len(dimNames) {
			return fmt.Errorf("sku %q must pick exactly one value for every dimension of product %q", in.ID, productID)
		}
		for _, name := range dimNames {
			v, present := in.Attrs[name]
			if !present {
				return fmt.Errorf("sku %q missing value for dimension %q", in.ID, name)
			}
			if v == "" {
				return fmt.Errorf("sku %q has empty value for dimension %q", in.ID, name)
			}
			if !legal[name][v] {
				return fmt.Errorf("sku %q uses invalid value %q for dimension %q", in.ID, v, name)
			}
		}
		for name := range in.Attrs {
			if !contains(dimNames, name) {
				return fmt.Errorf("sku %q references unknown dimension %q", in.ID, name)
			}
		}

		key := comboKey(in.Attrs, dimNames)
		if owner, taken := used[key]; taken {
			return fmt.Errorf("sku %q conflicts with sku %q on the same combination", in.ID, owner)
		}
		if batchCombos[key] {
			return fmt.Errorf("sku %q repeats a combination already present in the batch", in.ID)
		}
		batchCombos[key] = true

		if in.Price < 0 {
			return fmt.Errorf("sku %q price must be non-negative", in.ID)
		}
		if in.Stock < 0 {
			return fmt.Errorf("sku %q stock must be non-negative", in.ID)
		}

		prepared = append(prepared, &SKU{
			ID:      in.ID,
			Product: productID,
			Attrs:   cloneAttrs(in.Attrs),
			Price:   in.Price,
			Stock:   in.Stock,
			Listed:  true,
		})
	}

	for _, s := range prepared {
		c.skus[s.ID] = s
	}
	return nil
}

func contains(list []string, v string) bool {
	for _, item := range list {
		if item == v {
			return true
		}
	}
	return false
}
