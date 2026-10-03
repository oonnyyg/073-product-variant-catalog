package catalog

import (
	"strings"
	"testing"
)

func mkProduct(t *testing.T, c *Catalog) {
	t.Helper()
	if err := c.CreateProduct("shirt", []Dimension{
		{Name: "color", Values: []string{"red", "blue"}},
		{Name: "size", Values: []string{"S", "L"}},
	}); err != nil {
		t.Fatalf("create product: %v", err)
	}
}

func TestCreateProductValidation(t *testing.T) {
	c := New()
	if err := c.CreateProduct("", nil); err == nil {
		t.Fatal("empty product id accepted")
	}
	if err := c.CreateProduct("p", nil); err == nil {
		t.Fatal("product without dimensions accepted")
	}
	if err := c.CreateProduct("p", []Dimension{{Name: "color", Values: []string{"red", "red"}}}); err == nil {
		t.Fatal("duplicate dimension value accepted")
	}
	if err := c.CreateProduct("p", []Dimension{
		{Name: "color", Values: []string{"red"}},
		{Name: "color", Values: []string{"S"}},
	}); err == nil {
		t.Fatal("duplicate dimension name accepted")
	}
	mkProduct(t, c)
	if err := c.CreateProduct("shirt", []Dimension{{Name: "x", Values: []string{"y"}}}); err == nil {
		t.Fatal("duplicate product id accepted")
	}
}

func TestAddSKUsValidationAndAtomicity(t *testing.T) {
	c := New()
	mkProduct(t, c)

	good := []SKUInput{
		{ID: "a", Attrs: map[string]string{"color": "red", "size": "S"}, Price: 100, Stock: 5},
		{ID: "b", Attrs: map[string]string{"size": "L", "color": "red"}, Price: 200, Stock: 0},
	}
	if err := c.AddSKUs("shirt", good); err != nil {
		t.Fatalf("add skus: %v", err)
	}
	if s := c.SKU("a"); s == nil || !s.Listed {
		t.Fatalf("new sku missing or not listed: %+v", s)
	}

	// Second batch: duplicate combo (order-independent), duplicate id,
	// invalid value, negative price, wrong dimension count must all reject.
	bad := [][]SKUInput{
		{{ID: "c", Attrs: map[string]string{"size": "S", "color": "red"}, Price: 1, Stock: 1}},
		{{ID: "a", Attrs: map[string]string{"color": "blue", "size": "S"}, Price: 1, Stock: 1}},
		{{ID: "d", Attrs: map[string]string{"color": "green", "size": "S"}, Price: 1, Stock: 1}},
		{{ID: "e", Attrs: map[string]string{"color": "blue", "size": "S"}, Price: -1, Stock: 1}},
		{{ID: "f", Attrs: map[string]string{"color": "blue"}, Price: 1, Stock: 1}},
	}
	for i, batch := range bad {
		err := c.AddSKUs("shirt", batch)
		if err == nil {
			t.Fatalf("bad batch %d accepted", i)
		}
	}

	// Batch with one valid then one conflicting item must not partially write.
	err := c.AddSKUs("shirt", []SKUInput{
		{ID: "g", Attrs: map[string]string{"color": "blue", "size": "S"}, Price: 1, Stock: 1},
		{ID: "h", Attrs: map[string]string{"color": "red", "size": "S"}, Price: 1, Stock: 1},
	})
	if err == nil {
		t.Fatal("conflicting batch accepted")
	}
	if c.SKU("g") != nil {
		t.Fatal("partial batch write happened")
	}

	// Intra-batch duplicate combination.
	err = c.AddSKUs("shirt", []SKUInput{
		{ID: "i", Attrs: map[string]string{"color": "blue", "size": "S"}, Price: 1, Stock: 1},
		{ID: "j", Attrs: map[string]string{"size": "S", "color": "blue"}, Price: 1, Stock: 1},
	})
	if err == nil || !strings.Contains(err.Error(), "j") {
		t.Fatalf("expected sku j conflict, got %v", err)
	}
}

func TestDelistedStillOccupies(t *testing.T) {
	c := New()
	mkProduct(t, c)
	if err := c.AddSKUs("shirt", []SKUInput{
		{ID: "a", Attrs: map[string]string{"color": "red", "size": "S"}, Price: 100, Stock: 5},
	}); err != nil {
		t.Fatal(err)
	}
	if err := c.SetListed("a", false); err != nil {
		t.Fatal(err)
	}
	err := c.AddSKUs("shirt", []SKUInput{
		{ID: "b", Attrs: map[string]string{"color": "red", "size": "S"}, Price: 100, Stock: 5},
	})
	if err == nil {
		t.Fatal("delisted sku combination was reused")
	}
}

func TestUpdateSKUsAtomic(t *testing.T) {
	c := New()
	mkProduct(t, c)
	if err := c.AddSKUs("shirt", []SKUInput{
		{ID: "a", Attrs: map[string]string{"color": "red", "size": "S"}, Price: 100, Stock: 5},
	}); err != nil {
		t.Fatal(err)
	}
	neg := -3
	err := c.UpdateSKUs([]AttrUpdate{
		{SKU: "a", Price: intPtr(9)},
		{SKU: "missing", Stock: intPtr(2)},
		{SKU: "a", Stock: &neg},
	})
	if err == nil {
		t.Fatal("invalid update batch accepted")
	}
	if s := c.SKU("a"); s.Price != 100 || s.Stock != 5 {
		t.Fatalf("state changed on failed batch: %+v", s)
	}
	if err := c.UpdateSKUs([]AttrUpdate{{SKU: "a", Price: intPtr(150), Stock: intPtr(0)}}); err != nil {
		t.Fatal(err)
	}
	if s := c.SKU("a"); s.Price != 150 || s.Stock != 0 {
		t.Fatalf("update not applied: %+v", s)
	}
}

func intPtr(v int) *int { return &v }
