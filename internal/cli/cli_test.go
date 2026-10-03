package cli

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func runLines(t *testing.T, dir, input string) []map[string]interface{} {
	t.Helper()
	engine, err := NewEngine(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := engine.Run(strings.NewReader(input), &out); err != nil {
		t.Fatal(err)
	}
	var resps []map[string]interface{}
	for _, line := range strings.Split(strings.TrimSpace(out.String()), "\n") {
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("bad response %q: %v", line, err)
		}
		resps = append(resps, m)
	}
	return resps
}

func TestEndToEnd(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	input := strings.Join([]string{
		`{"cmd":"create_product","data":{"id":"shirt","dimensions":[{"name":"color","values":["red","blue"]},{"name":"size","values":["s","m"]}]}}`,
		`{"cmd":"add_skus","data":{"product":"shirt","skus":[{"id":"a1","values":{"color":"red","size":"s"},"price":1000,"stock":2},{"id":"a2","values":{"color":"red","size":"m"},"price":1200,"stock":0},{"id":"a3","values":{"color":"blue","size":"s"},"price":900,"stock":1}]}}`,
		`{"cmd":"query","data":{"product":"shirt","select":{}}}`,
		`{"cmd":"set_active","data":{"sku":"a1","active":false}}`,
		`{"cmd":"query","data":{"product":"shirt","select":{"color":"red"}}}`,
		`{"cmd":"add_skus","data":{"product":"shirt","skus":[{"id":"bad","values":{"color":"red","size":"xl"},"price":1,"stock":1}]}}`,
		`{"cmd":"query","data":{"product":"shirt","select":{"color":"green"}}}`,
		`not json at all`,
	}, "\n")

	resps := runLines(t, dir, input)
	if len(resps) != 8 {
		t.Fatalf("got %d responses", len(resps))
	}
	if resps[0]["ok"] != true || resps[1]["ok"] != true {
		t.Fatalf("setup failed: %+v %+v", resps[0], resps[1])
	}

	// Initial query: a1 and a3 sellable; a2 out of stock.
	skus := resps[2]["result"].(map[string]interface{})["skus"].([]interface{})
	if len(skus) != 2 {
		t.Fatalf("expected 2 sellable skus, got %d", len(skus))
	}

	// After delisting a1, color=red yields nothing; processing continues.
	if resps[3]["ok"] != true {
		t.Fatalf("set_active failed: %+v", resps[3])
	}
	q2 := resps[4]["result"].(map[string]interface{})
	if len(q2["skus"].([]interface{})) != 0 {
		t.Fatalf("delisted sku still returned: %+v", q2)
	}

	if resps[5]["ok"] != false {
		t.Fatalf("invalid sku batch should error: %+v", resps[5])
	}
	if resps[6]["ok"] != false {
		t.Fatalf("unknown value should error: %+v", resps[6])
	}
	if resps[7]["ok"] != false {
		t.Fatalf("malformed line should error but keep running: %+v", resps[7])
	}
}

func TestInvalidJSONAndPersistence(t *testing.T) {
	dir := t.TempDir()
	runLines(t, dir, `{"cmd":"create_product","data":{"id":"p","dimensions":[{"name":"d","values":["v1","v2"]}]}}`+"\nnotjson\n")

	// Reload state from disk and query.
	resps := runLines(t, dir, `{"cmd":"query","data":{"product":"p","select":{}}}`+"\n")
	if resps[0]["ok"] != true {
		t.Fatalf("product did not persist: %+v", resps[0])
	}
	avail := resps[0]["result"].(map[string]interface{})["available"].(map[string]interface{})
	if _, ok := avail["d"]; !ok {
		t.Fatalf("dimension missing after reload: %+v", avail)
	}
}
