package catalog

import "testing"

func seedCatalog(t *testing.T) *Catalog {
	t.Helper()
	c := New()
	mkProduct(t, c)
	// red/S listed in stock; red/L listed zero stock; blue/S delisted; blue/L listed stock.
	inputs := []SKUInput{
		{ID: "rs", Attrs: map[string]string{"color": "red", "size": "S"}, Price: 100, Stock: 5},
		{ID: "rl", Attrs: map[string]string{"color": "red", "size": "L"}, Price: 100, Stock: 0},
		{ID: "bs", Attrs: map[string]string{"color": "blue", "size": "S"}, Price: 100, Stock: 5},
		{ID: "bl", Attrs: map[string]string{"color": "blue", "size": "L"}, Price: 100, Stock: 5},
	}
	if err := c.AddSKUs("shirt", inputs); err != nil {
		t.Fatal(err)
	}
	if err := c.SetListed("bs", false); err != nil {
		t.Fatal(err)
	}
	return c
}

func ids(items []QueryResultItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.ID
	}
	return out
}

func TestQuerySellable(t *testing.T) {
	c := seedCatalog(t)

	res, err := c.Query("shirt", nil)
	if err != nil {
		t.Fatal(err)
	}
	got := ids(res.SKUs)
	want := []string{"bl", "rs"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("empty filter: got %v want %v", got, want)
	}
}

func TestQueryCrossDimensionOptions(t *testing.T) {
	c := seedCatalog(t)

	// Filter color=red: only rs sellable (rl out of stock).
	res, err := c.Query("shirt", map[string]string{"color": "red"})
	if err != nil {
		t.Fatal(err)
	}
	if got := ids(res.SKUs); len(got) != 1 || got[0] != "rs" {
		t.Fatalf("color=red skus = %v", got)
	}
	// color options ignore its own red selection but keep no other filters:
	// red stays available (rs), blue via bl only (bs delisted).
	if got := res.Options["color"]; len(got) != 2 || got[0] != "blue" || got[1] != "red" {
		t.Fatalf("color options = %v", got)
	}
	// size options keep color=red: only S is sellable for red.
	if got := res.Options["size"]; len(got) != 1 || got[0] != "S" {
		t.Fatalf("size options = %v", got)
	}
}

func TestQueryNoMatchAndErrors(t *testing.T) {
	c := seedCatalog(t)

	res, err := c.Query("shirt", map[string]string{"color": "red", "size": "L"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.SKUs) != 0 {
		t.Fatalf("expected no sellable red/L, got %v", ids(res.SKUs))
	}
	if res.SKUs == nil {
		t.Fatal("expected empty non-nil slice")
	}
	// No SKU matches, but options show how to recover: switching color to
	// blue keeps size=L sellable (bl); switching size to S keeps red (rs).
	if got := res.Options["color"]; len(got) != 1 || got[0] != "blue" {
		t.Fatalf("color recovery options = %v", got)
	}
	if got := res.Options["size"]; len(got) != 1 || got[0] != "S" {
		t.Fatalf("size recovery options = %v", got)
	}

	if _, err := c.Query("shirt", map[string]string{"width": "W"}); err == nil {
		t.Fatal("unknown dimension accepted")
	}
	if _, err := c.Query("shirt", map[string]string{"color": "green"}); err == nil {
		t.Fatal("unknown value accepted")
	}
	if _, err := c.Query("nope", nil); err == nil {
		t.Fatal("unknown product accepted")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	c := seedCatalog(t)
	dir := t.TempDir()
	if err := c.Save(dir); err != nil {
		t.Fatal(err)
	}
	c2, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c2.SKU("bs") == nil || c2.SKU("bs").Listed {
		t.Fatal("delisted sku state not restored")
	}
	if c2.Product("shirt") == nil {
		t.Fatal("product not restored")
	}
	// Reloaded catalog must still enforce combination uniqueness.
	err = c2.AddSKUs("shirt", []SKUInput{
		{ID: "new", Attrs: map[string]string{"color": "red", "size": "S"}, Price: 1, Stock: 1},
	})
	if err == nil {
		t.Fatal("conflicting combo accepted after reload")
	}

	empty, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if empty.Product("shirt") != nil {
		t.Fatal("fresh state dir should be empty")
	}
}
