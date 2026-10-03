package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Save writes the whole catalog to dir/products.json and dir/skus.json
// atomically (temp file + rename), so a crash never leaves a torn file.
func (c *Catalog) Save(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	products := make([]Product, 0, len(c.products))
	for _, id := range c.sortedProductIDs() {
		products = append(products, *c.products[id])
	}
	skus := make([]SKU, 0, len(c.skus))
	for _, s := range c.skus {
		skus = append(skus, *s)
	}

	if err := writeJSONAtomic(filepath.Join(dir, "products.json"), products); err != nil {
		return err
	}
	return writeJSONAtomic(filepath.Join(dir, "skus.json"), skus)
}

func writeJSONAtomic(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Load reads a catalog previously written by Save. A missing state dir means
// a fresh, empty catalog.
func Load(dir string) (*Catalog, error) {
	c := New()

	productData, err := os.ReadFile(filepath.Join(dir, "products.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	var products []Product
	if err := json.Unmarshal(productData, &products); err != nil {
		return nil, err
	}
	for i := range products {
		p := products[i]
		c.products[p.ID] = &p
	}

	skuData, err := os.ReadFile(filepath.Join(dir, "skus.json"))
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	var skus []SKU
	if err := json.Unmarshal(skuData, &skus); err != nil {
		return nil, err
	}
	for i := range skus {
		s := skus[i]
		c.skus[s.ID] = &s
	}
	return c, nil
}
