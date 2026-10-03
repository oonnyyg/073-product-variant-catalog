package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func runLines(t *testing.T, dir, input string) []map[string]any {
	t.Helper()
	var out, errOut bytes.Buffer
	code := run([]string{"--state", dir}, strings.NewReader(input), &out, &errOut)
	if code != 0 {
		t.Fatalf("exit code %d, stderr: %s", code, errOut.String())
	}
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	resps := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("invalid response %q: %v", line, err)
		}
		resps = append(resps, m)
	}
	return resps
}

func j(v string) string { return v }

func TestLineProtocolAndPersistence(t *testing.T) {
	dir := t.TempDir()

	session1 := strings.Join([]string{
		j(`{"cmd":"create_product","id":"shirt","dimensions":[{"name":"color","values":["red","blue"]},{"name":"size","values":["S","L"]}]}`),
		j(`{"cmd":"add_skus","product":"shirt","skus":[{"id":"rs","attrs":{"color":"red","size":"S"},"price":100,"stock":5},{"id":"rl","attrs":{"color":"red","size":"L"},"price":100,"stock":0},{"id":"bs","attrs":{"color":"blue","size":"S"},"price":100,"stock":5},{"id":"bl","attrs":{"color":"blue","size":"L"},"price":100,"stock":5}]}`),
		// Invalid batch: id already exists globally -> whole batch rejected.
		j(`{"cmd":"add_skus","product":"shirt","skus":[{"id":"rs","attrs":{"color":"blue","size":"S"},"price":1,"stock":1}]}`),
		j(`{"cmd":"set_listed","id":"bs","listed":false}`),
		j(`{"cmd":"update_skus","updates":[{"sku":"rs","price":150},{"sku":"nope","stock":2}]}`),
		j(`{"cmd":"query","product":"shirt","filter":{"color":"red"}}`),
		j(`{"cmd":"query","product":"shirt","filter":{"color":"red","size":"L"}}`),
		j(`{"cmd":"query","product":"shirt","filter":{"color":"green"}}`),
		`not json`,
	}, "\n")

	resps := runLines(t, dir, session1)
	if resps[0]["ok"] != true {
		t.Fatalf("create product failed: %v", resps[0])
	}
	if resps[1]["ok"] != true {
		t.Fatalf("add skus failed: %v", resps[1])
	}
	if e, _ := resps[2]["error"].(string); !strings.Contains(e, "rs") {
		t.Fatalf("expected error naming sku rs, got %v", resps[2])
	}
	if resps[3]["ok"] != true {
		t.Fatalf("delist failed: %v", resps[3])
	}
	if e, _ := resps[4]["error"].(string); !strings.Contains(e, "nope") {
		t.Fatalf("expected error naming sku nope, got %v", resps[4])
	}

	// Query red: rs sellable (price still 100 because update batch failed atomically).
	q := resps[5]["result"].(map[string]any)
	skus := q["skus"].([]any)
	if len(skus) != 1 {
		t.Fatalf("expected one sellable red sku, got %v", skus)
	}
	rs := skus[0].(map[string]any)
	if rs["id"] != "rs" || rs["price"].(float64) != 100 {
		t.Fatalf("unexpected rs: %v", rs)
	}
	options := q["options"].(map[string]any)
	sizeOpts := options["size"].([]any)
	if len(sizeOpts) != 1 || sizeOpts[0] != "S" {
		t.Fatalf("red size options = %v", sizeOpts)
	}

	// red/L out of stock: empty list but recovery options present.
	q2 := resps[6]["result"].(map[string]any)
	if len(q2["skus"].([]any)) != 0 {
		t.Fatal("red/L should have no sellable sku")
	}
	colorOpts := q2["options"].(map[string]any)["color"].([]any)
	if len(colorOpts) != 1 || colorOpts[0] != "blue" {
		t.Fatalf("recovery color options = %v", colorOpts)
	}

	if _, ok := resps[7]["error"].(string); !ok {
		t.Fatal("unknown value should error")
	}
	if _, ok := resps[8]["error"].(string); !ok {
		t.Fatal("malformed JSON should error")
	}

	// Reopen: state persists including delisting.
	resps2 := runLines(t, dir, `{"cmd":"query","product":"shirt"}`)
	q3 := resps2[0]["result"].(map[string]any)
	ids := []string{}
	for _, item := range q3["skus"].([]any) {
		ids = append(ids, item.(map[string]any)["id"].(string))
	}
	if len(ids) != 2 || ids[0] != "bl" || ids[1] != "rs" {
		t.Fatalf("after restart sellable ids = %v, want [bl rs]", ids)
	}
}
