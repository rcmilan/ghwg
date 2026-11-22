package sound

import (
	"slices"
	"strings"
	"testing"
)

const loopTicks = LoopBeats * testDiv

// goodFile: every voice with one in-range, in-key note in the loop, coda at beat 160.
func goodFile(extra map[Voice][]tn, skip Voice, coda int) []byte {
	pitch := [Voices]int{43, 65, 72, 74, 69, 81} // G2 F4 C5 D5 A4 A5: in range and in the collection
	var tracks [][]byte
	for v := Voice(0); v < Voices; v++ {
		if v == skip {
			continue
		}
		ns := append([]tn{{pitch[v], 0, testDiv, 90}}, extra[v]...)
		c := -1
		if v == Concierge {
			c = coda
		}
		tracks = append(tracks, track(VoiceNames[v], c, ns...))
	}
	return smf(1, testDiv, tracks...)
}

func lint(t *testing.T, b []byte) []string {
	t.Helper()
	sc, names, err := ParseTracks(b)
	if err != nil {
		t.Fatal(err)
	}
	return Lint(sc, names)
}

func has(lines []string, sub string) bool {
	for _, l := range lines {
		if strings.Contains(l, sub) {
			return true
		}
	}
	return false
}

func TestLintClean(t *testing.T) {
	if l := lint(t, goodFile(nil, -1, loopTicks)); len(l) != 0 {
		t.Errorf("clean file: %q", l)
	}
}

func TestLintErrors(t *testing.T) {
	l := lint(t, goodFile(nil, Tim, loopTicks))
	if len(l) != 1 || l[0] != "error: no notes for voice Tim (missing or empty track)" {
		t.Errorf("missing track: %q", l)
	}

	b := goodFile(nil, -1, loopTicks)
	b = append(b, chunk("MTrk", track("Banjo", -1, tn{60, 0, testDiv, 80}))...)
	b[10]++ // one more track in the header
	if l := lint(t, b); len(l) != 1 || !strings.HasPrefix(l[0], "error: unknown track name \"Banjo\"") {
		t.Errorf("unknown track: %q", l)
	}

	if l := lint(t, goodFile(nil, -1, -1)); !has(l, "error: coda marker missing") {
		t.Errorf("no coda: %q", l)
	}
	if l := lint(t, goodFile(nil, -1, loopTicks-testDiv)); !has(l, "error: coda marker missing or at beat 79") {
		t.Errorf("early coda: %q", l)
	}

	cross := map[Voice][]tn{Magda: {{72, loopTicks - testDiv, 2 * testDiv, 80}}}
	l = lint(t, goodFile(cross, -1, loopTicks))
	if len(l) != 1 || l[0] != "error: Magda bar 20 beat 4 C5 crosses the coda line" {
		t.Errorf("crossing note: %q", l)
	}
	// ending exactly on the line, or starting on it, is fine
	ok := map[Voice][]tn{Magda: {{72, loopTicks - testDiv, testDiv, 80}, {72, loopTicks, testDiv, 80}}}
	if l := lint(t, goodFile(ok, -1, loopTicks)); len(l) != 0 {
		t.Errorf("notes at the line: %q", l)
	}
}

func TestLintWarnings(t *testing.T) {
	// bar 12 beat 3 is beat 46 (11*4+2): C7 is out of the sax's range; F#4 is fine (the harmony is chromatic)
	w := map[Voice][]tn{Magenta: {{96, 46 * testDiv, testDiv, 80}, {66, 47 * testDiv, testDiv, 80}}}
	l := lint(t, goodFile(w, -1, loopTicks))
	want := []string{
		"warn: Magenta bar 12 beat 3 C7 is outside sax's range C#3-A5",
	}
	if !slices.Equal(l, want) {
		t.Errorf("warnings: %q", l)
	}
}

func TestNoteName(t *testing.T) {
	for p, want := range map[int]string{60: "C4", 70: "Bb4", 28: "E1", 108: "C8"} {
		if got := NoteName(p); got != want {
			t.Errorf("NoteName(%d) = %s, want %s", p, got, want)
		}
	}
}

// The score is drafted separately (T401): once data/fugue.mid exists it must lint without errors.
func TestEmbeddedScore(t *testing.T) {
	b, err := Data.ReadFile("data/fugue.mid")
	if err != nil {
		t.Skip("data/fugue.mid not drafted yet")
	}
	sc, names, err := ParseTracks(b)
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range Lint(sc, names) {
		if strings.HasPrefix(l, "error:") {
			t.Error(l)
		}
	}
}

func TestParallel(t *testing.T) {
	for _, c := range []struct {
		a0, a1, b0, b1 int
		want           bool
	}{
		{50, 52, 57, 59, true},  // D-A to E-B
		{50, 52, 62, 64, true},  // octaves
		{50, 52, 57, 57, false}, // oblique
		{50, 52, 57, 60, false}, // fifth to sixth
		{57, 59, 50, 52, true},  // either order
		{-1, 52, 57, 59, false}, // from a rest
	} {
		if got := Parallel(c.a0, c.a1, c.b0, c.b1); got != c.want {
			t.Errorf("Parallel(%d,%d,%d,%d) = %v", c.a0, c.a1, c.b0, c.b1, got)
		}
	}
}
