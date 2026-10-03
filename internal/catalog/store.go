package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
)

const stateFileName = "catalog-state.json"

// stateVersion documents the on-disk format for forward compatibility.
type stateFile struct {
	Version int      `json:"version"`
	Catalog *Catalog `json:"catalog"`
}

// Load reads catalog state from dir, returning an empty catalog if no state
// file exists yet.
func Load(dir string) (*Catalog, error) {
	path := filepath.Join(dir, stateFileName)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return New(), nil
	}
	if err != nil {
		return nil, err
	}
	var sf stateFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return nil, err
	}
	if sf.Catalog == nil {
		return New(), nil
	}
	return sf.Catalog, nil
}

// Save writes catalog state to dir via a temp file + rename so the state
// file is never left half-written.
func (c *Catalog) Save(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	sf := stateFile{Version: 1, Catalog: c}
	data, err := json.MarshalIndent(&sf, "", "  ")
	if err != nil {
		return err
	}
	final := filepath.Join(dir, stateFileName)
	tmp, err := os.CreateTemp(dir, ".catalog-state-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, final)
}
