package catalog

import (
	"fmt"
	"sort"
)

// Dimension is one variant axis of a product, e.g. color -> [red, blue].
type Dimension struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// SKU is one concrete, sellable combination of dimension values.
type SKU struct {
	ID      string            `json:"id"`
	Product string            `json:"product"`
	Attrs   map[string]string `json:"attrs"`
	Price   int               `json:"price"`
	Stock   int               `json:"stock"`
	Listed  bool              `json:"listed"`
}

// Product is an immutable-spec product with its SKUs.
type Product struct {
	ID         string      `json:"id"`
	Dimensions []Dimension `json:"dimensions"`
}

// Catalog holds all products and SKUs in memory.
type Catalog struct {
	products map[string]*Product
	skus     map[string]*SKU
}

// New creates an empty catalog.
func New() *Catalog {
	return &Catalog{
		products: map[string]*Product{},
		skus:     map[string]*SKU{},
	}
}

// Product returns a copy of the product, or nil if it does not exist.
func (c *Catalog) Product(id string) *Product {
	p, ok := c.products[id]
	if !ok {
		return nil
	}
	cp := *p
	cp.Dimensions = make([]Dimension, len(p.Dimensions))
	for i, d := range p.Dimensions {
		cp.Dimensions[i] = Dimension{Name: d.Name, Values: append([]string(nil), d.Values...)}
	}
	return &cp
}

// SKU returns a copy of the SKU, or nil if it does not exist.
func (c *Catalog) SKU(id string) *SKU {
	s, ok := c.skus[id]
	if !ok {
		return nil
	}
	cp := *s
	cp.Attrs = cloneAttrs(s.Attrs)
	return &cp
}

// CreateProduct validates and registers a product. Its specs never change.
func (c *Catalog) CreateProduct(id string, dims []Dimension) error {
	if id == "" {
		return fmt.Errorf("product id must not be empty")
	}
	if _, exists := c.products[id]; exists {
		return fmt.Errorf("product %q already exists", id)
	}
	if len(dims) == 0 {
		return fmt.Errorf("product %q must define at least one dimension", id)
	}
	seenNames := map[string]bool{}
	cleaned := make([]Dimension, 0, len(dims))
	for _, d := range dims {
		if d.Name == "" {
			return fmt.Errorf("product %q has an empty dimension name", id)
		}
		if seenNames[d.Name] {
			return fmt.Errorf("product %q has duplicate dimension %q", id, d.Name)
		}
		seenNames[d.Name] = true
		if len(d.Values) == 0 {
			return fmt.Errorf("product %q dimension %q has no values", id, d.Name)
		}
		seenValues := map[string]bool{}
		values := make([]string, 0, len(d.Values))
		for _, v := range d.Values {
			if v == "" {
				return fmt.Errorf("product %q dimension %q has an empty value", id, d.Name)
			}
			if seenValues[v] {
				return fmt.Errorf("product %q dimension %q has duplicate value %q", id, d.Name, v)
			}
			seenValues[v] = true
			values = append(values, v)
		}
		cleaned = append(cleaned, Dimension{Name: d.Name, Values: values})
	}
	c.products[id] = &Product{ID: id, Dimensions: cleaned}
	return nil
}

func cloneAttrs(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// sortedProductIDs lists product IDs in ascending order.
func (c *Catalog) sortedProductIDs() []string {
	ids := make([]string, 0, len(c.products))
	for id := range c.products {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
