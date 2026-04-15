// Package sqliteembed embeds the sqlite3 binary and an example db,
// exposing a minimal Query helper that shells out to the embedded binary
// extracted to a temp dir.
package sqliteembed

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

//go:embed bin/sqlite3
var sqliteBin []byte

//go:embed db/example.db
var exampleDB []byte

var (
	extractOnce sync.Once
	extractedBin string
	extractedDB  string
	extractErr   error
)

func extract() (string, string, error) {
	extractOnce.Do(func() {
		dir, err := os.MkdirTemp("", "proto-sqlite-*")
		if err != nil {
			extractErr = err
			return
		}
		bin := filepath.Join(dir, "sqlite3")
		if err := os.WriteFile(bin, sqliteBin, 0o755); err != nil {
			extractErr = err
			return
		}
		db := filepath.Join(dir, "example.db")
		if err := os.WriteFile(db, exampleDB, 0o644); err != nil {
			extractErr = err
			return
		}
		extractedBin, extractedDB = bin, db
	})
	return extractedBin, extractedDB, extractErr
}

// Query runs a SQL statement against the embedded example db and returns
// stdout from the sqlite3 CLI.
func Query(stmt string) (string, error) {
	bin, db, err := extract()
	if err != nil {
		return "", fmt.Errorf("extract: %w", err)
	}
	out, err := exec.Command(bin, db, stmt).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("sqlite3: %w", err)
	}
	return string(out), nil
}
