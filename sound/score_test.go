package sound

import (
	"encoding/binary"
	"slices"
	"sort"
	"strings"
	"testing"
)

const testDiv = 480 // ticks a beat in the test files

// A tiny SMF writer: enough to build the files the tests read.

func vlq(v int) []byte {
	out := []byte{byte(v & 0x7f)}
	for v >>= 7; v > 0; v >>= 7 {
		out = append([]byte{byte(v&0x7f) | 0x80}, out...)
	}
	return out
}

func chunk(id string, body []byte) []byte {
	h := append([]byte(id), 0, 0, 0, 0)
	binary.BigEndian.PutUint32(h[4:], uint32(len(body)))
	return append(h, body...)
}

func smf(format, div uint16, tracks ...[]byte) []byte {
	h := make([]byte, 6)
	binary.BigEndian.PutUint16(h, format)
	binary.BigEndian.PutUint16(h[2:], uint16(len(tracks)))
	binary.BigEndian.PutUint16(h[4:], div)
	out := chunk("MThd", h)
	for _, t := range tracks {
		out = append(out, chunk("MTrk", t)...)
	}
	return out
}

func meta(delta int, typ byte, text string) []byte {
	b := append(vlq(delta), 0xFF, typ)
	b = append(b, vlq(len(text))...)
	return append(b, text...)
}

// tn is a note in ticks.
type tn struct{ pitch, start, length, vel int }

// track writes a named track of notes (on with the given velocity, off as note-on velocity 0 for odd pitches and
// as 0x80 for even ones, to cover both), plus a coda marker at codaTick when it is >= 0.
func track(name string, codaTick int, notes ...tn) []byte {
	type ev struct {
		tick int
		b    []byte
	}
	var evs []ev
	for _, n := range notes {
		evs = append(evs, ev{n.start, []byte{0x90, byte(n.pitch), byte(n.vel)}})
		if n.pitch%2 == 1 {
			evs = append(evs, ev{n.start + n.length, []byte{0x90, byte(n.pitch), 0}})
		} else {
			evs = append(evs, ev{n.start + n.length, []byte{0x80, byte(n.pitch), 64}})
		}
	}
	if codaTick >= 0 {
		evs = append(evs, ev{codaTick, append([]byte{0xFF, 0x06, 4}, "coda"...)})
	}
	sort.SliceStable(evs, func(i, j int) bool { return evs[i].tick < evs[j].tick })
	var out []byte
	if name != "" {
		out = meta(0, 0x03, name)
	}
	out = append(out, vlq(0)...)
	out = append(out, 0xFF, 0x51, 3, 0x07, 0xA1, 0x20) // tempo: ignored
	last := 0
	for _, e := range evs {
		out = append(out, vlq(e.tick-last)...)
		out = append(out, e.b...)
		last = e.tick
	}
	return append(out, 0, 0xFF, 0x2F, 0)
}

func TestParseNotes(t *testing.T) {
	b := smf(1, testDiv,
		track("Magenta", -1, tn{70, testDiv, testDiv / 2, 127}, tn{65, 0, testDiv, 64}),
		track("  tim ", -1, tn{62, 0, 2 * testDiv, 100}))
	sc, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	m := sc.Notes[Magenta]
	if len(m) != 2 || m[0].Pitch != 65 || m[0].Start != 0 || m[0].Len != 1 || m[1].Pitch != 70 ||
		m[1].Start != 1 || m[1].Len != .5 || m[1].Vel != 1 {
		t.Errorf("Magenta (sorted by start): %+v", m)
	}
	if len(sc.Notes[Tim]) != 1 || sc.Notes[Tim][0].Len != 2 {
		t.Errorf("Tim (name is trimmed, case-insensitive): %+v", sc.Notes[Tim])
	}
	if sc.End != 2 || sc.Coda != 2 {
		t.Errorf("no marker: Coda=End=2, got Coda %v End %v", sc.Coda, sc.End)
	}
}

func TestParseCoda(t *testing.T) {
	sc, err := Parse(smf(1, testDiv, track("Concierge", 4*testDiv, tn{40, 0, testDiv, 90})))
	if err != nil || sc.Coda != 4 || sc.End != 4 {
		t.Errorf("marker after the last note: Coda %v End %v err %v", sc.Coda, sc.End, err)
	}
	// a cue point on another track, upper case, with the notes running past it
	cue := append(meta(0, 0x03, "Magda"), meta(testDiv, 0x07, "CODA")...)
	cue = append(cue, 0, 0xFF, 0x2F, 0)
	sc, err = Parse(smf(1, testDiv, track("Concierge", -1, tn{40, 0, 8 * testDiv, 90}), cue))
	if err != nil || sc.Coda != 1 || sc.End != 8 {
		t.Errorf("cue on another track: Coda %v End %v err %v", sc.Coda, sc.End, err)
	}
}

func TestParseRunningStatusAndSkips(t *testing.T) {
	var tr []byte
	tr = append(tr, meta(0, 0x03, "Osvaldo")...)
	tr = append(tr, 0, 0xF0, 3, 1, 2, 0xF7)        // sysex
	tr = append(tr, 0, 0xB0, 7, 100)               // controller
	tr = append(tr, 0, 0xC0, 5)                    // program change
	tr = append(tr, 0, 0x90, 80, 100)              // on
	tr = append(tr, vlq(testDiv)...)               // running status: on, a second note
	tr = append(tr, 82, 100)                       //
	tr = append(tr, vlq(testDiv)...)               // running status: velocity 0 closes the first
	tr = append(tr, 80, 0)                         //
	tr = append(tr, vlq(testDiv)...)               //
	tr = append(tr, 82, 0)                         // and the second
	tr = append(tr, 0, 0xE0, 0, 64)                // pitch bend
	tr = append(tr, 0, 0xFF, 0x58, 4, 4, 2, 24, 8) // meter
	tr = append(tr, 0, 0xFF, 0x2F, 0)
	sc, err := Parse(smf(1, testDiv, tr))
	if err != nil {
		t.Fatal(err)
	}
	n := sc.Notes[Osvaldo]
	if len(n) != 2 || n[0] != (Note{80, 0, 2, 100. / 127}) || n[1] != (Note{82, 1, 2, 100. / 127}) {
		t.Errorf("running status notes: %+v", n)
	}
}

func TestParseOverlapFIFO(t *testing.T) {
	// two C4s: on 0, on 1, off 2, off 3 beats; the first off closes the first on
	sc, err := Parse(smf(1, testDiv, track("Magda", -1,
		tn{60, 0, 2 * testDiv, 80}, tn{60, testDiv, 2 * testDiv, 80})))
	if err != nil {
		t.Fatal(err)
	}
	n := sc.Notes[Magda]
	if len(n) != 2 || n[0].Len != 2 || n[1].Start != 1 || n[1].Len != 2 {
		t.Errorf("overlap: %+v", n)
	}
}

func TestParseFormat0AndTracks(t *testing.T) {
	_, names, err := ParseTracks(smf(0, testDiv, track("Tim", -1, tn{60, 0, testDiv, 80})))
	if err != nil || !slices.Equal(names, []string{"Tim"}) {
		t.Errorf("format 0: %v %v", names, err)
	}
	sc, names, err := ParseTracks(smf(1, testDiv, track("", -1, tn{60, 0, testDiv, 80}),
		track("Piano", -1, tn{60, 0, testDiv, 80}), track("Tim", -1, tn{61, 0, testDiv, 80})))
	if err != nil || !slices.Equal(names, []string{"Piano", "Tim"}) {
		t.Errorf("names (unnamed tracks skipped): %v %v", names, err)
	}
	if sc.End != 1 || len(sc.Notes[Tim]) != 1 {
		t.Errorf("unknown tracks ignored: %+v", sc)
	}
}

func TestParseErrors(t *testing.T) {
	for name, b := range map[string][]byte{
		"empty":   nil,
		"junk":    []byte("not a midi file at all"),
		"smpte":   smf(1, 0xE728, track("Tim", -1)),
		"format2": smf(2, testDiv, track("Tim", -1)),
		"cut":     smf(1, testDiv, track("Tim", -1, tn{60, 0, testDiv, 80}))[:30],
	} {
		if _, err := Parse(b); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	if _, err := Parse(smf(1, 0xE728)); err == nil || !strings.Contains(err.Error(), "SMPTE") {
		t.Errorf("smpte error: %v", err)
	}
}

// The shipped score carries the Concierge's real line, and lints clean.
func TestShippedScoreRealLine(t *testing.T) {
	data, err := Data.ReadFile("data/fugue.mid")
	if err != nil {
		t.Skip("no score")
	}
	sc, names, err := ParseTracks(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(sc.Real) == 0 {
		t.Error("no Concierge real track")
	}
	if l := Lint(sc, names); len(l) != 0 {
		t.Errorf("the shipped score lints: %q", l)
	}
}
