package catalog

import "testing"

func newQueriedCatalog(t *testing.T) *Catalog {
	t.Helper()
	c := New()
	if err := c.CreateProduct("p1", dimsForProduct()); err != nil {
		t.Fatal(err)
	}
	addTestSKUs(t, c, "p1")
	// sku-r-m has stock 0; delist sku-b-s too so both paths are covered.
	if err := c.SetActive("sku-b-s", false); err != nil {
		t.Fatal(err)
	}
	return c
}

func skuIDs(views []SKUView) []string {
	ids := make([]string, len(views))
	for i, v := range views {
		ids[i] = v.ID
	}
	return ids
}

func TestQueryEmptyFilter(t *testing.T) {
	c := newQueriedCatalog(t)
	res, err := c.Query("p1", nil)
	if err != nil {
		t.Fatal(err)
	}
	// Sellable: sku-r-s (active,stock5), sku-b-l (active,stock2).
	// sku-r-m out of stock, sku-b-s delisted.
	got := skuIDs(res.SKUs)
	want := []string{"sku-b-l", "sku-r-s"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v want %v", got, want)
	}
	if eq := slicesEqual(res.Available["color"], []string{"red", "blue"}); !eq {
		t.Fatalf("color options = %v", res.Available["color"])
	}
	if eq := slicesEqual(res.Available["size"], []string{"s", "l"}); !eq {
		t.Fatalf("size options = %v", res.Available["size"])
	}
}

func TestQueryCrossDimensionFilter(t *testing.T) {
	c := newQueriedCatalog(t)
	// Select color=red: only sku-r-s is sellable. size options should keep
	// the color filter and ignore size's own (unset here).
	res, err := c.Query("p1", map[string]string{"color": "red"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.SKUs) != 1 || res.SKUs[0].ID != "sku-r-s" {
		t.Fatalf("unexpected skus: %v", skuIDs(res.SKUs))
	}
	if !slicesEqual(res.Available["size"], []string{"s"}) {
		t.Fatalf("size options = %v", res.Available["size"])
	}
	// color options ignore own selection (other filters none): every
	// sellable color is still an option.
	if !slicesEqual(res.Available["color"], []string{"red", "blue"}) {
		t.Fatalf("color options = %v", res.Available["color"])
	}
}

func TestQueryOwnSelectionIgnoredForOptions(t *testing.T) {
	c := newQueriedCatalog(t)
	// Select size=s. Sellable with size s: sku-r-s (sku-b-s delisted).
	// size options ignore own selection, keep color filter (none):
	// sellable sizes overall are {s,l}.
	res, err := c.Query("p1", map[string]string{"size": "s"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.SKUs) != 1 || res.SKUs[0].ID != "sku-r-s" {
		t.Fatalf("unexpected skus: %v", skuIDs(res.SKUs))
	}
	if !slicesEqual(res.Available["size"], []string{"s", "l"}) {
		t.Fatalf("size options should ignore own selection: %v", res.Available["size"])
	}
	if !slicesEqual(res.Available["color"], []string{"red"}) {
		t.Fatalf("color options = %v", res.Available["color"])
	}
}

func TestQueryNoMatchAndErrors(t *testing.T) {
	c := newQueriedCatalog(t)
	res, err := c.Query("p1", map[string]string{"color": "red", "size": "l"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.SKUs) != 0 {
		t.Fatalf("expected no skus, got %v", skuIDs(res.SKUs))
	}
	// Options ignore each dimension's own selection while keeping others:
	// keeping size=l allows switching color to blue; keeping color=red
	// allows switching size to s.
	if !slicesEqual(res.Available["color"], []string{"blue"}) {
		t.Fatalf("color options = %v", res.Available["color"])
	}
	if !slicesEqual(res.Available["size"], []string{"s"}) {
		t.Fatalf("size options = %v", res.Available["size"])
	}
	if _, err := c.Query("p1", map[string]string{"width": "w"}); err == nil {
		t.Fatal("unknown dimension accepted")
	}
	if _, err := c.Query("p1", map[string]string{"color": "green"}); err == nil {
		t.Fatal("unknown value accepted")
	}
	if _, err := c.Query("nope", nil); err == nil {
		t.Fatal("unknown product accepted")
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
