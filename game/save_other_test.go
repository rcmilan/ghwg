//go:build !js

package game

import (
	"path/filepath"
	"testing"
)

func TestSaveFile(t *testing.T) {
	p := filepath.Join(t.TempDir(), "ghosthotel", "save.json")
	if b, err := readFile(p); b != nil || err != nil {
		t.Fatalf("no file: %q %v", b, err)
	}
	for _, want := range []string{`{"level":1}`, `{"level":2,"bonus":1.5}`} { // the second replaces the first
		if err := writeFile(p, []byte(want)); err != nil {
			t.Fatal(err)
		}
		if b, err := readFile(p); string(b) != want || err != nil {
			t.Fatalf("got %q %v, want %q", b, err, want)
		}
	}
	if m, _ := filepath.Glob(p + ".tmp"); len(m) != 0 {
		t.Error("temp file left behind")
	}
}
