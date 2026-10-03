package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"example.com/product-variant-catalog/internal/catalog"
)

type request struct {
	Cmd     string               `json:"cmd"`
	Product string               `json:"product,omitempty"`
	ID      string               `json:"id,omitempty"`
	Dims    []catalog.Dimension  `json:"dimensions,omitempty"`
	SKUs    []catalog.SKUInput   `json:"skus,omitempty"`
	Listed  *bool                `json:"listed,omitempty"`
	Updates []catalog.AttrUpdate `json:"updates,omitempty"`
	Filter  map[string]string    `json:"filter,omitempty"`
}

type response struct {
	OK     bool   `json:"ok"`
	Error  string `json:"error,omitempty"`
	Result any    `json:"result,omitempty"`
}

func main() {
	code := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	os.Exit(code)
}

func run(args []string, in io.Reader, out, errOut io.Writer) int {
	stateDir := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--state":
			if i+1 >= len(args) {
				fmt.Fprintln(errOut, "--state requires a directory")
				return 2
			}
			i++
			stateDir = args[i]
		default:
			fmt.Fprintf(errOut, "unknown argument %q\n", args[i])
			return 2
		}
	}
	if stateDir == "" {
		fmt.Fprintln(os.Stderr, "missing required --state directory")
		return 2
	}

	cat, err := catalog.Load(stateDir)
	if err != nil {
		fmt.Fprintf(errOut, "load state: %v\n", err)
		return 1
	}

	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	writer := bufio.NewWriter(out)
	defer writer.Flush()

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(bytesTrimSpace(line)) == 0 {
			continue
		}
		var req request
		resp := response{}
		if err := json.Unmarshal(line, &req); err != nil {
			resp.Error = "invalid JSON: " + err.Error()
			writeResp(writer, resp)
			continue
		}

		mutated, err := dispatch(cat, &req, &resp)
		if err != nil {
			resp.Error = err.Error()
		} else {
			resp.OK = true
		}
		if mutated && err == nil {
			if saveErr := cat.Save(stateDir); saveErr != nil {
				resp = response{Error: "save state: " + saveErr.Error()}
			}
		}
		writeResp(writer, resp)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(errOut, "read stdin: %v\n", err)
		return 1
	}
	return 0
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

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n'
}

func writeResp(w *bufio.Writer, r response) {
	data, _ := json.Marshal(r)
	_, _ = w.Write(data)
	_ = w.WriteByte('\n')
	_ = w.Flush()
}

// dispatch executes one command. It reports whether the command mutated
// state so the caller can persist it.
func dispatch(cat *catalog.Catalog, req *request, resp *response) (bool, error) {
	switch req.Cmd {
	case "create_product":
		return true, cat.CreateProduct(req.ID, req.Dims)
	case "add_skus":
		return true, cat.AddSKUs(req.Product, req.SKUs)
	case "set_listed":
		if req.Listed == nil {
			return false, fmt.Errorf("set_listed requires boolean field \"listed\"")
		}
		return true, cat.SetListed(req.ID, *req.Listed)
	case "update_skus":
		return true, cat.UpdateSKUs(req.Updates)
	case "query":
		result, err := cat.Query(req.Product, req.Filter)
		if err != nil {
			return false, err
		}
		resp.Result = result
		return false, nil
	default:
		return false, fmt.Errorf("unknown command %q", req.Cmd)
	}
}
