package sound

import (
	"testing"
	"testing/fstest"
)

func TestLoadBankEmptyIsSine(t *testing.T) {
	b, err := LoadBank(fstest.MapFS{})
	if err != nil {
		t.Fatal(err)
	}
	for i, set := range b.Inst {
		if len(set) != len(SineBank().Inst[i]) {
			t.Fatalf("%s: want the sine set, got %d samples", InstrumentNames[i], len(set))
		}
	}
	if len(b.Steps) == 0 {
		t.Fatal("steps fall back to the sine bank's")
	}
}

// The shipped bank: the three steps; no instrument samples while the game has no music (10, 2026-11-27: the
// music, #16, is not in the release), so the instruments are the sine placeholders.
func TestLoadBankShipped(t *testing.T) {
	b, err := LoadBank(Data)
	if err != nil {
		t.Fatal(err)
	}
	for i, set := range b.Inst {
		if len(set) != len(SineBank().Inst[i]) {
			t.Errorf("%s: %d samples shipped, want none (the sine set)", InstrumentNames[i], len(set))
		}
	}
	if len(b.Steps) != 3 {
		t.Errorf("3 steps, got %d", len(b.Steps))
	}
}

func TestLoadBankBadFile(t *testing.T) {
	for _, name := range []string{"data/sax_62.ogg", "data/step_1.ogg"} {
		_, err := LoadBank(fstest.MapFS{name: {Data: []byte("not an ogg")}})
		if err == nil {
			t.Fatalf("%s: a malformed file is an error", name)
		}
		if want := name; len(err.Error()) < len(want) || err.Error()[7:7+len(want)] != want {
			t.Fatalf("the error names the file %s, got %v", name, err)
		}
	}
}

// The loader hands the bank over only with the last file in, one file a step.
func TestBankLoaderSteps(t *testing.T) {
	l, err := NewBankLoader(Data)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(l.files); n < 2 {
		t.Fatalf("files: %v", l.files)
	}
	steps, n := 0, len(l.files)
	for {
		b, err := l.Step()
		if err != nil {
			t.Fatal(err)
		}
		steps++
		if b != nil {
			break
		}
	}
	if steps != n {
		t.Fatalf("one file a step: %d steps for %d files", steps, n)
	}
}

// Bell: peak 1, dies away to nothing.
func TestBell(t *testing.T) {
	b := Bell(81, 2.5)
	peak, tail := float32(0), float32(0)
	for i, x := range b.PCM {
		peak = max(peak, abs32(x))
		if i > len(b.PCM)*9/10 {
			tail = max(tail, abs32(x))
		}
	}
	if peak < .99 || peak > 1.001 || tail > .05 {
		t.Errorf("peak %.3f, tail %.3f", peak, tail)
	}
}

func abs32(x float32) float32 { return max(x, -x) }
