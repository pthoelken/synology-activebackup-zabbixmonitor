package collector

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

func ReadCache(path string) (Snapshot, error) {
	var snapshot Snapshot
	data, err := os.ReadFile(path)
	if err != nil {
		return snapshot, err
	}
	if len(data) == 0 {
		return snapshot, errors.New("cache file is empty")
	}
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return snapshot, err
	}
	return snapshot, nil
}

func WriteCache(path string, snapshot Snapshot) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
