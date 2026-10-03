package catalog

import (
	"errors"
	"fmt"
	"sort"
)

// Product is a product with fixed variant dimensions.
type Product struct {
	ID         string          `json:"id"`
	Dimensions []*Dimension    `json:"dimensions"`
	SKUs       map[string]*SKU `json:"skus"`
}

// Dimension is one variant axis, e.g. "color" with values ["red","blue"].
type Dimension struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// SKU is a concrete sellable combination of dimension values.
type SKU struct {
	ID      string            `json:"id"`
	Product string            `json:"product"`
	Values  map[string]string `json:"values"`
	Price   int               `json:"price"`
	Stock   int               `json:"stock"`
	Active  bool              `json:"active"`
}

// Catalog holds all products keyed by product id, plus a global SKU index.
type Catalog struct {
	Products map[string]*Product `json:"products"`
	SKUIndex map[string]string   `json:"sku_index"` // sku id -> product id
}

// New returns an empty catalog.
func New() *Catalog {
	return &Catalog{
		Products: map[string]*Product{},
		SKUIndex: map[string]string{},
	}
}

// CreateProduct validates and registers a product with fixed dimensions.
func (c *Catalog) CreateProduct(id string, dims []*Dimension) error {
	if id == "" {
		return errors.New("product id must not be empty")
	}
	if _, exists := c.Products[id]; exists {
		return fmt.Errorf("product %q already exists", id)
	}
	if len(dims) == 0 {
		return fmt.Errorf("product %q must have at least one dimension", id)
	}
	seenDims := map[string]bool{}
	cleanDims := make([]*Dimension, 0, len(dims))
	for _, d := range dims {
		if d == nil || d.Name == "" {
			return fmt.Errorf("product %q has an empty dimension name", id)
		}
		if seenDims[d.Name] {
			return fmt.Errorf("product %q has duplicate dimension %q", id, d.Name)
		}
		seenDims[d.Name] = true
		if len(d.Values) == 0 {
			return fmt.Errorf("product %q dimension %q has no values", id, d.Name)
		}
		seenVals := map[string]bool{}
		cleanVals := make([]string, 0, len(d.Values))
		for _, v := range d.Values {
			if v == "" {
				return fmt.Errorf("product %q dimension %q has an empty value", id, d.Name)
			}
			if seenVals[v] {
				return fmt.Errorf("product %q dimension %q has duplicate value %q", id, d.Name, v)
			}
			seenVals[v] = true
			cleanVals = append(cleanVals, v)
		}
		cleanDims = append(cleanDims, &Dimension{Name: d.Name, Values: cleanVals})
	}
	c.Products[id] = &Product{
		ID:         id,
		Dimensions: cleanDims,
		SKUs:       map[string]*SKU{},
	}
	return nil
}

// product returns the product or an error locating it.
func (c *Catalog) product(pid string) (*Product, error) {
	p, ok := c.Products[pid]
	if !ok {
		return nil, fmt.Errorf("product %q not found", pid)
	}
	return p, nil
}

// sortedSKUIDs returns SKU ids of a product in ascending order.
func sortedSKUIDs(p *Product) []string {
	ids := make([]string, 0, len(p.SKUs))
	for id := range p.SKUs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
