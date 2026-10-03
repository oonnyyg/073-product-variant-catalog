package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"

	"example.com/product-variant-catalog/internal/catalog"
)

// Request is one line of input. Data carries command-specific payload.
type Request struct {
	Cmd  string          `json:"cmd"`
	Data json.RawMessage `json:"data"`
}

type response struct {
	OK     bool        `json:"ok"`
	Result interface{} `json:"result,omitempty"`
	Error  string      `json:"error,omitempty"`
}

type productData struct {
	ID         string               `json:"id"`
	Dimensions []*catalog.Dimension `json:"dimensions"`
}

type addSKUData struct {
	Product string            `json:"product"`
	SKUs    []catalog.SKUSpec `json:"skus"`
}

type activeData struct {
	SKUID  string `json:"sku"`
	Active bool   `json:"active"`
}

type updateData struct {
	Updates []catalog.SKUUpdate `json:"updates"`
}

type queryData struct {
	Product string            `json:"product"`
	Select  map[string]string `json:"select"`
}

// Engine processes commands against a catalog and persists after each
// successful mutation.
type Engine struct {
	Catalog  *catalog.Catalog
	StateDir string
}

// NewEngine loads state from dir.
func NewEngine(dir string) (*Engine, error) {
	c, err := catalog.Load(dir)
	if err != nil {
		return nil, err
	}
	return &Engine{Catalog: c, StateDir: dir}, nil
}

// Run reads JSON lines from r and writes one JSON response per line to w.
func (e *Engine) Run(r io.Reader, w io.Writer) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	bw := bufio.NewWriter(w)
	defer bw.Flush()

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytesTrimSpace(line)) == 0 {
			continue
		}
		var req Request
		if err := json.Unmarshal(line, &req); err != nil {
			writeErr(bw, fmt.Sprintf("invalid json: %v", err))
			continue
		}
		result, save, err := e.dispatch(&req)
		if err != nil {
			writeErr(bw, err.Error())
			continue
		}
		if save {
			if err := e.Catalog.Save(e.StateDir); err != nil {
				writeErr(bw, fmt.Sprintf("failed to persist state: %v", err))
				continue
			}
		}
		writeOK(bw, result)
	}
	return scanner.Err()
}

// dispatch executes one command. The bool reports whether state changed.
func (e *Engine) dispatch(req *Request) (interface{}, bool, error) {
	switch req.Cmd {
	case "create_product":
		var d productData
		if err := json.Unmarshal(req.Data, &d); err != nil {
			return nil, false, fmt.Errorf("invalid data: %v", err)
		}
		if err := e.Catalog.CreateProduct(d.ID, d.Dimensions); err != nil {
			return nil, false, err
		}
		return map[string]string{"id": d.ID}, true, nil

	case "add_skus":
		var d addSKUData
		if err := json.Unmarshal(req.Data, &d); err != nil {
			return nil, false, fmt.Errorf("invalid data: %v", err)
		}
		ids, err := e.Catalog.AddSKUs(d.Product, d.SKUs)
		if err != nil {
			return nil, false, err
		}
		return map[string]interface{}{"added": ids}, true, nil

	case "set_active":
		var d activeData
		if err := json.Unmarshal(req.Data, &d); err != nil {
			return nil, false, fmt.Errorf("invalid data: %v", err)
		}
		if err := e.Catalog.SetActive(d.SKUID, d.Active); err != nil {
			return nil, false, err
		}
		return map[string]interface{}{"sku": d.SKUID, "active": d.Active}, true, nil

	case "update_skus":
		var d updateData
		if err := json.Unmarshal(req.Data, &d); err != nil {
			return nil, false, fmt.Errorf("invalid data: %v", err)
		}
		ids, err := e.Catalog.BatchUpdate(d.Updates)
		if err != nil {
			return nil, false, err
		}
		return map[string]interface{}{"updated": ids}, true, nil

	case "query":
		var d queryData
		if err := json.Unmarshal(req.Data, &d); err != nil {
			return nil, false, fmt.Errorf("invalid data: %v", err)
		}
		res, err := e.Catalog.Query(d.Product, d.Select)
		if err != nil {
			return nil, false, err
		}
		return res, false, nil

	default:
		return nil, false, fmt.Errorf("unknown command %q", req.Cmd)
	}
}

func writeOK(w io.Writer, result interface{}) {
	writeJSON(w, response{OK: true, Result: result})
}

func writeErr(w io.Writer, msg string) {
	writeJSON(w, response{OK: false, Error: msg})
}

func writeJSON(w io.Writer, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		data = []byte(`{"ok":false,"error":"internal json error"}`)
	}
	w.Write(data)
	w.Write([]byte("\n"))
}

func bytesTrimSpace(b []byte) []byte {
	start, end := 0, len(b)
	for start < end && isSpace(b[start]) {
		start++
	}
	for end > start && isSpace(b[end-1]) {
		end--
	}
	return b[start:end]
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}
