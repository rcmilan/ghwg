//go:build !js

package game

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

func savePath() (string, error) {
	dir, err := os.UserConfigDir()
	return filepath.Join(dir, "ghosthotel", "save.json"), err
}

func platformRead() ([]byte, error) {
	p, err := savePath()
	if err != nil {
		return nil, err
	}
	return readFile(p)
}

func platformWrite(b []byte) error {
	p, err := savePath()
	if err != nil {
		return err
	}
	return writeFile(p, b)
}

func readFile(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return b, err
}

// writeFile: a temp file beside it, then a rename, so a crash never leaves half a save.
func writeFile(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
