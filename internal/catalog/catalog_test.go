package catalog

import "testing"

func dimsForProduct() []*Dimension {
	return []*Dimension{
		{Name: "color", Values: []string{"red", "blue"}},
		{Name: "size", Values: []string{"s", "m", "l"}},
	}
}

func TestCreateProductValidation(t *testing.T) {
	c := New()
	if err := c.CreateProduct("", dimsForProduct()); err == nil {
		t.Fatal("empty id accepted")
	}
	if err := c.CreateProduct("p1", nil); err == nil {
		t.Fatal("no dimensions accepted")
	}
	bad := []*Dimension{
		{Name: "color", Values: []string{"red", "red"}},
		{Name: "color", Values: []string{"x"}},
	}
	if err := c.CreateProduct("p1", bad); err == nil {
		t.Fatal("duplicate dim/value accepted")
	}
	if err := c.CreateProduct("p1", dimsForProduct()); err != nil {
		t.Fatalf("valid create failed: %v", err)
	}
	if err := c.CreateProduct("p1", dimsForProduct()); err == nil {
		t.Fatal("duplicate product accepted")
	}
}

func addTestSKUs(t *testing.T, c *Catalog, pid string) {
	t.Helper()
	specs := []SKUSpec{
		{ID: "sku-r-s", Values: map[string]string{"color": "red", "size": "s"}, Price: 100, Stock: 5},
		{ID: "sku-r-m", Values: map[string]string{"size": "m", "color": "red"}, Price: 200, Stock: 0},
		{ID: "sku-b-s", Values: map[string]string{"color": "blue", "size": "s"}, Price: 300, Stock: 3},
		{ID: "sku-b-l", Values: map[string]string{"color": "blue", "size": "l"}, Price: 400, Stock: 2},
	}
	if _, err := c.AddSKUs(pid, specs); err != nil {
		t.Fatalf("add skus: %v", err)
	}
}

func TestAddSKUsValidationAndAtomicity(t *testing.T) {
	c := New()
	if err := c.CreateProduct("p1", dimsForProduct()); err != nil {
		t.Fatal(err)
	}

	// Unknown value.
	bad := []SKUSpec{{ID: "x", Values: map[string]string{"color": "red", "size": "xl"}, Price: 1, Stock: 1}}
	if _, err := c.AddSKUs("p1", bad); err == nil {
		t.Fatal("invalid value accepted")
	}
	// Missing a dimension.
	bad = []SKUSpec{{ID: "x", Values: map[string]string{"color": "red"}, Price: 1, Stock: 1}}
	if _, err := c.AddSKUs("p1", bad); err == nil {
		t.Fatal("missing dimension accepted")
	}
	// Unknown extra dimension cannot happen with count mismatch test too.
	bad = []SKUSpec{{ID: "x", Values: map[string]string{"color": "red", "size": "s", "fit": "x"}, Price: 1, Stock: 1}}
	if _, err := c.AddSKUs("p1", bad); err == nil {
		t.Fatal("extra dimension accepted")
	}
	// Negative price.
	bad = []SKUSpec{{ID: "x", Values: map[string]string{"color": "red", "size": "s"}, Price: -1, Stock: 1}}
	if _, err := c.AddSKUs("p1", bad); err == nil {
		t.Fatal("negative price accepted")
	}

	// Duplicate id inside batch -> atomic reject, nothing written.
	bad = []SKUSpec{
		{ID: "dup", Values: map[string]string{"color": "red", "size": "s"}, Price: 1, Stock: 1},
		{ID: "dup", Values: map[string]string{"color": "red", "size": "m"}, Price: 1, Stock: 1},
	}
	if _, err := c.AddSKUs("p1", bad); err == nil {
		t.Fatal("duplicate id in batch accepted")
	}
	// Conflicting combination (order independent) inside batch.
	bad = []SKUSpec{
		{ID: "a", Values: map[string]string{"color": "red", "size": "s"}, Price: 1, Stock: 1},
		{ID: "b", Values: map[string]string{"size": "s", "color": "red"}, Price: 1, Stock: 1},
	}
	if _, err := c.AddSKUs("p1", bad); err == nil {
		t.Fatal("conflicting combo in batch accepted")
	}
	if len(c.Products["p1"].SKUs) != 0 {
		t.Fatalf("atomicity broken, %d skus written", len(c.Products["p1"].SKUs))
	}

	addTestSKUs(t, c, "p1")

	// Global uniqueness: same id for another product fails.
	c.CreateProduct("p2", dimsForProduct())
	bad = []SKUSpec{{ID: "sku-r-s", Values: map[string]string{"color": "red", "size": "s"}, Price: 1, Stock: 1}}
	if _, err := c.AddSKUs("p2", bad); err == nil {
		t.Fatal("globally duplicate sku id accepted")
	}

	// Combination already taken (including by an inactive sku).
	if err := c.SetActive("sku-r-s", false); err != nil {
		t.Fatal(err)
	}
	bad = []SKUSpec{{ID: "sku-new", Values: map[string]string{"size": "s", "color": "red"}, Price: 1, Stock: 1}}
	if _, err := c.AddSKUs("p1", bad); err == nil {
		t.Fatal("inactive sku combo should remain occupied")
	}
}

func TestBatchUpdateAtomicity(t *testing.T) {
	c := New()
	c.CreateProduct("p1", dimsForProduct())
	addTestSKUs(t, c, "p1")

	neg := -5
	updates := []SKUUpdate{
		{ID: "sku-r-s", Price: intPtr(150)},
		{ID: "sku-b-s", Stock: &neg},
	}
	if _, err := c.BatchUpdate(updates); err == nil {
		t.Fatal("negative stock accepted")
	}
	if c.Products["p1"].SKUs["sku-r-s"].Price != 100 {
		t.Fatal("price changed despite batch failure")
	}

	goodPrice, goodStock := 999, 7
	if _, err := c.BatchUpdate([]SKUUpdate{
		{ID: "sku-r-s", Price: &goodPrice, Stock: &goodStock},
		{ID: "sku-b-l", Stock: &goodStock},
	}); err != nil {
		t.Fatal(err)
	}
	sku := c.Products["p1"].SKUs["sku-r-s"]
	if sku.Price != 999 || sku.Stock != 7 {
		t.Fatalf("update not applied: %+v", sku)
	}

	// Missing sku.
	if _, err := c.BatchUpdate([]SKUUpdate{{ID: "nope", Price: intPtr(1)}}); err == nil {
		t.Fatal("update of missing sku accepted")
	}
	// Duplicate in batch.
	if _, err := c.BatchUpdate([]SKUUpdate{
		{ID: "sku-r-s", Price: intPtr(1)},
		{ID: "sku-r-s", Stock: intPtr(1)},
	}); err == nil {
		t.Fatal("duplicate sku in update batch accepted")
	}
}

func intPtr(v int) *int { return &v }
