// score writes the first draft of the fugue (T401, rewritten after live test 2) to sound/data/fugue.mid:
// go run ./tools/score [-o file].
//
// Once the user starts revising (T405) the .mid is the source of truth and this tool is history: running it
// again overwrites their edits, so it refuses to touch an existing file without -force.
//
// How the draft is built (context/09-sound/decisions.md):
//   - a D minor ballad at 60 BPM in Monk's vocabulary (our own subject and changes): 20 bars, ten 2-bar cells,
//     each cell i | iiø V7b9 in its own key: D A D A D G C Eb D D, so a cell's lines fit any cell transposed;
//   - one fugue for six voices split by realm: per cell the humans (Magenta, Magda) play two of the lines and
//     the ghosts (Tim, Celestine, Osvaldo) the other two, one ghost resting in turn; no line is ever doubled.
//     The subject goes to the humans in cells 1 3 7 10, the ghosts in 2 4 6 8, the Concierge in 5 and 9;
//   - the lines: the subject (S), its countersubject (CS), and three slower ones (CS2, a guide-tone line CS3
//     sliding down by half steps, CS4 for the cells where the bass has the subject);
//   - each line is placed in the octave nearest its voice's register; the humans swung, the ghosts straight;
//   - the real dimension is bright cool jazz (live test 3; the game reads the score 2.5 times as fast there,
//     150 BPM): the humans have their own lines in F, a pianoless two-horn counterpoint over the walk in the
//     Mulligan / Modern Jazz Quartet manner, each cell Fmaj7 | Bbmaj7 C7 (I | IV V7, the F scale making the IV
//     Lydian), staying near F (realKeys: F, C and Bb only). Its subject (RS) passes between Magenta and Magda, the
//     other playing a countersubject (RC, or RC2 in quarters), the bass taking RS in cells 5 and 9 and walking
//     quarters from bar 1. The Concierge has a second track (sound.RealConcierge) walking it, played in real, his
//     own track in ghost;
//   - the Concierge walks the changes (halves in the first four cells, then quarters), each note picked to move
//     in no parallel fifth or octave with any voice above (sound.Parallel, which midilint also checks);
//   - after bar 20 a `coda` marker and the finale's music: a treasure chest opening (coda below).
package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math"
	"os"

	"ghosthotel/sound"
)

const (
	tpq      = 480 // ticks a quarter: a swung eighth's 2/3 split is whole (320/160)
	loopBars = 20
	cells    = loopBars / 2
	rest     = 99
)

// A line: semitones from the cell's tonic and lengths in eighths, over a cell's two bars (16 eighths), written
// over Dm | Eø A7b9.
type step struct{ rel, len int }

var lines = map[string][]step{
	"S":   {{rest, 1}, {-5, 1}, {0, 2}, {3, 1}, {2, 1}, {-1, 2}, {8, 2}, {5, 1}, {2, 1}, {-1, 1}, {-4, 1}, {-5, 2}},
	"CS":  {{3, 2}, {5, 1}, {3, 1}, {0, 2}, {2, 1}, {3, 1}, {5, 4}, {2, 1}, {5, 1}, {-1, 2}},
	"CS2": {{7, 8}, {12, 4}, {5, 4}},
	"CS3": {{10, 4}, {9, 4}, {8, 4}, {7, 4}},
	"CS4": {{5, 8}, {8, 4}, {2, 4}},
}

// The real dimension's lines: semitones from the cell's tonic (realKeys), over Fmaj7 | Bbmaj7 C7, swung. RS is
// the subject, running eighths up through a Lydian B and the IV's raised fourth (E); RC answers it in longer notes,
// against its motion; RC2 is an inner line in quarters, contrary to RS where it can be. No two of them, nor RS in
// the bass, move in parallel fifths or octaves.
var realLines = map[string][]step{
	"RS":  {{4, 1}, {6, 1}, {7, 1}, {11, 1}, {9, 2}, {7, 1}, {4, 1}, {9, 2}, {11, 1}, {12, 1}, {11, 1}, {7, 1}, {5, 1}, {2, 1}},
	"RC":  {{rest, 1}, {12, 1}, {11, 4}, {14, 2}, {12, 2}, {9, 2}, {14, 2}, {11, 2}},
	"RC2": {{7, 2}, {4, 2}, {0, 2}, {-1, 2}, {0, 2}, {4, 2}, {5, 2}, {2, 2}},
}

var vel = map[string]int{"S": 100, "CS": 86, "CS2": 72, "CS3": 72, "CS4": 72, "RS": 100, "RC": 86, "RC2": 76}

// keys: each cell's tonic, semitones above D. realKeys: the real dimension's, near F (F C Bb), not the ghosts' tour.
var (
	keys     = [cells]int{0, 7, 0, 7, 0, 5, -2, 1, 0, 0}
	realKeys = [cells]int{3, 10, 3, 10, 3, 8, 3, 10, 3, 3}
)

type voice struct {
	name    string
	centre  int  // the MIDI pitch its lines sit nearest (their mean)
	swung   bool // the humans
	program int  // General MIDI, for listening in MuseScore; the game ignores it
	parts   [cells]string
}

// realParts: the real dimension's schedule, the humans' and the Concierge's real track ("" walks).
var realParts = map[string][cells]string{
	"Concierge": {"", "", "", "", "RS", "", "", "", "RS", ""},
	"Magenta":   {"RS", "RC", "RS", "RC2", "RC", "RS", "RC", "RS", "RC2", "RC"},
	"Magda":     {"RC2", "RS", "RC", "RS", "RC2", "RC", "RS", "RC2", "RC", "RS"},
}

// The schedule: the line each voice plays in each cell ("" rests; the Concierge's "" walks). The humans' parts
// here are unused: they play realParts.
var voices = []voice{
	{"Concierge", 40, false, 32, [cells]string{"", "", "", "", "S", "", "", "", "S", ""}},
	{"Magenta", 62, true, 66, [cells]string{"S", "CS", "CS2", "CS3", "CS", "CS2", "S", "CS", "CS2", "CS3"}},
	{"Magda", 66, true, 11, [cells]string{"CS3", "CS2", "S", "CS", "CS2", "CS3", "CS2", "CS3", "CS", "S"}},
	{"Celestine", 76, false, 8, [cells]string{"CS2", "", "CS", "", "CS4", "S", "", "CS2", "CS3", ""}},
	{"Tim", 64, false, 59, [cells]string{"CS", "S", "", "CS2", "CS3", "", "CS", "S", "", "CS"}},
	{"Osvaldo", 79, false, 78, [cells]string{"", "CS3", "CS3", "S", "", "CS", "CS3", "", "CS4", "CS2"}},
}

type note struct{ pitch, on, off, vel int } // ticks

// at: the tick of eighth e, swung or straight.
func at(e int, swung bool) int {
	t := e / 2 * tpq
	if e%2 == 1 {
		if swung {
			return t + tpq*2/3
		}
		return t + tpq/2
	}
	return t
}

// place writes a line into cell c, its tonic ks[c] over D, at the octave whose mean pitch is nearest the voice's
// centre.
func place(v voice, c int, l []step, velocity int, ks [cells]int, out []note) []note {
	sum, n := 0, 0
	for _, s := range l {
		if s.rel != rest {
			sum, n = sum+s.rel*s.len, n+s.len
		}
	}
	tonic := 62 + ks[c]
	tonic += 12 * int(math.Round(float64(v.centre-tonic-sum/n)/12))
	e := c * 16
	for _, s := range l {
		if s.rel != rest {
			out = append(out, note{tonic + s.rel, at(e, v.swung), at(e+s.len, v.swung), velocity})
		}
		e += s.len
	}
	return out
}

func beats(ns []note) []sound.Note {
	out := make([]sound.Note, len(ns))
	for i, n := range ns {
		out[i] = sound.Note{Pitch: n.pitch, Start: float64(n.on) / tpq, Len: float64(n.off-n.on) / tpq}
	}
	return out
}

// compose: every voice's part, then the Concierge's real line as one more (sound.RealConcierge), both his walks
// last: they steer clear of the lines above.
func compose() [][]note {
	parts := make([][]note, len(voices)+1)
	var ghosts, humans [][]sound.Note // each walk steers clear of its own realm's lines
	for i, v := range voices[1:] {
		sched, ls, ks := v.parts, lines, keys
		if v.swung { // the humans: the real dimension
			sched, ls, ks = realParts[v.name], realLines, realKeys
		}
		for c, l := range sched {
			if l != "" {
				parts[i+1] = place(v, c, ls[l], vel[l], ks, parts[i+1])
			}
		}
		if v.swung {
			humans = append(humans, beats(parts[i+1]))
		} else {
			ghosts = append(ghosts, beats(parts[i+1]))
		}
	}
	parts[0] = walk(voices[0], voices[0].parts, lines, keys, ghosts, minorCell, 4)
	parts[len(voices)] = walk(voices[0], realParts["Concierge"], realLines, realKeys, humans, majorCell, 0)
	for i := range parts {
		parts[i] = append(parts[i], coda(voices[i%len(voices)])...) // the real line ends on the Concierge's fanfare
	}
	return parts
}

// harmony: a cell's chords beat by beat, by root (semitones over the cell's tonic) and tones (from the root).
type harmony struct {
	root  [8]int
	tones [8][]int
}

var (
	minorCell = harmony{[8]int{0, 0, 0, 0, 2, 2, 7, 7}, // Dm | Eø A7
		[8][]int{{0, 3, 7, 10}, {0, 3, 7, 10}, {0, 3, 7, 10}, {0, 3, 7, 10}, {0, 3, 6, 10}, {0, 3, 6, 10}, {0, 4, 7, 10}, {0, 4, 7, 10}}}
	majorCell = harmony{[8]int{0, 0, 0, 0, 5, 5, 7, 7}, // Fmaj7 | Bbmaj7 C7
		[8][]int{{0, 4, 7, 11}, {0, 4, 7, 11}, {0, 4, 7, 11}, {0, 4, 7, 11}, {0, 4, 7, 11}, {0, 4, 7, 11}, {0, 4, 7, 10}, {0, 4, 7, 10}}}
)

// walk: the Concierge's part. Halves in the first twoFeel cells, quarters after, the subject in cells 5 and 9.
// Each note is the first candidate inside 33..50 that makes no parallel with a voice above: on a chord's first
// beat its root, then its fifth and third; elsewhere a half step into the next chord's root when one follows,
// then the chord's tones.
func walk(v voice, sched [cells]string, ls map[string][]step, ks [cells]int, above [][]sound.Note, h harmony, twoFeel int) []note {
	var out []note
	prev := -1
	for c, l := range sched {
		if l != "" {
			out = place(v, c, ls[l], 96, ks, out)
			prev = out[len(out)-1].pitch
			continue
		}
		step := 1
		if c < twoFeel {
			step = 2
		}
		tonic := 38 + ks[c]
		for b := 0; b < 8; b += step {
			root := tonic + h.root[b]
			var cands []int
			if b == 0 || h.root[b] != h.root[b-step] {
				cands = []int{root, root + 7, root + h.tones[b][1], root + h.tones[b][3]}
			} else {
				if b+step == 8 || h.root[b+step] != h.root[b] {
					next := 38 + ks[(c+1)%cells] + h.root[0]
					if b+step < 8 {
						next = tonic + h.root[b+step]
					}
					cands = []int{next - 1, next + 1}
				}
				for _, t := range h.tones[b] {
					cands = append(cands, root+t)
				}
			}
			t1 := float64(c*8 + b)
			pick := -1
			for _, p := range cands {
				for p > 50 {
					p -= 12
				}
				for p < 33 {
					p += 12
				}
				if p == prev {
					continue
				}
				if pick < 0 {
					pick = p // the fallback: the first candidate, parallel or not
				}
				clear := true
				for _, u := range above {
					clear = clear && !sound.Parallel(prev, p, sound.Sounding(u, t1-.5+.2), sound.Sounding(u, t1+.2))
				}
				if clear {
					pick = p
					break
				}
			}
			out = append(out, note{pick, (c*8 + b) * tpq, (c*8 + b + step) * tpq, 92 - 4*(b%2)})
			prev = pick
		}
	}
	return out
}

// coda: the finale's music, a treasure chest opening (live test 3), timed to the scene's frames at 60 a beat
// (game/questscene.go: the lid lifts at 430, the dark at 880, the eyes open at 950, the scene ends at 1250):
//   - the build, from the walk-in to the lid: over a C pedal pulsing faster and louder, a major triad creeps up a
//     half step at a time, each step shorter, the sax and vibes holding it, the trumpet and whistle pulsing it,
//     the celesta trembling over it, from G to E: the E major chord lifts into F as the lid opens;
//   - the reveal: an F major arpeggio flung up through the voices, a big Fmaj9 on its top, then a soft shimmer
//     (celesta arpeggios, vibes and bass now and then) under the starry figure and the MacGuffin's line;
//   - the dark: C7b9 (V7), the bass rolling louder; as the eyes open, F7 (I7) with a sforzando on the high
//     instruments, the rest under them.
func coda(v voice) []note {
	const lid, reveal, dark, eyes, end = 430, 460, 880, 950, 1250
	var out []note
	add := func(p int, f0, f1 float64, vel int) { // frames from the coda's start
		on := (loopBars*4)*tpq + int(math.Round(f0/60*tpq))
		out = append(out, note{p, on, (loopBars*4)*tpq + int(math.Round(f1/60*tpq)), min(vel, 127)})
	}
	steps := []float64{70, 60, 55, 50, 45, 40, 35, 30, 25, 20} // the creep's steps, frames: they sum to lid
	f := 0.0
	for k, l := range steps {
		vel := 50 + 5*k
		switch v.name {
		case "Magenta":
			add(55+k, f, f+l, vel) // the triad's root, G3 up to E4
		case "Magda":
			add(59+k, f, f+l, vel-6) // its third
		case "Tim", "Osvaldo":
			p := 62 + k // its fifth, pulsing in eighths (15 frames)
			if v.name == "Osvaldo" {
				p += 12
			}
			for t := f; t < f+l; t += 15 {
				add(p, t, min(t+8, f+l), vel-10)
			}
		case "Celestine": // a tremolo, the root and fifth two octaves up
			for i, t := 0, f; t < f+l; i, t = i+1, t+7.5 {
				add(79+k+7*(i%2)-12*((79+k+7*(i%2))/96), t, t+7.5, vel-15)
			}
		}
		f += l
	}
	if v.name == "Concierge" { // the pedal: a beat apart at first, a 16th at the lid
		for t := 0.0; t < lid; {
			l := 60 - 45*t/lid
			add(36, t, t+l, 60+int(40*t/lid))
			t += l
		}
	}
	flourish := map[string][]int{"Magda": {60, 72}, "Magenta": {65}, "Tim": {69}, "Osvaldo": {77}, "Celestine": {81}}
	order := []int{60, 65, 69, 72, 77, 81} // C F A C F A, 5 frames apart
	for _, p := range flourish[v.name] {
		i := 0
		for order[i] != p {
			i++
		}
		add(p, lid+5*float64(i), lid+5*float64(i)+6, 90)
	}
	big := map[string][]int{ // Fmaj9 on the flourish's top
		"Concierge": {29, 41}, "Magenta": {57, 64}, "Magda": {65, 69}, "Tim": {72, 76}, "Osvaldo": {79, 84}, "Celestine": {81, 84, 89},
	}
	for _, p := range big[v.name] {
		add(p, reveal, reveal+150, 88)
	}
	switch v.name { // the shimmer
	case "Celestine":
		arp := []int{77, 81, 84, 88, 91, 88, 84, 81} // F A C E G
		for i, t := 0, 540.0; t < dark-15; i, t = i+1, t+15 {
			add(arp[i%len(arp)], t, t+15, 42)
		}
	case "Magda":
		for t := 540.0; t < dark; t += 120 {
			add(64, t, t+110, 48)
			add(69, t, t+110, 48)
		}
	case "Concierge":
		for t := 540.0; t < dark; t += 120 {
			add(41, t, t+110, 52)
		}
	}
	v7 := map[string][]int{"Magenta": {58, 64}, "Magda": {61, 67}, "Tim": {70, 76}, "Osvaldo": {79}, "Celestine": {85, 88}}
	for _, p := range v7[v.name] {
		add(p, dark, eyes, 64)
	}
	if v.name == "Concierge" { // the roll, louder to the eyes
		for t := float64(dark); t < eyes; t += 10 {
			add(36, t, t+10, 60+int(40*(t-dark)/(eyes-dark)))
		}
	}
	i7 := map[string][]int{ // F7: the sforzando on the high ones, the rest under them
		"Concierge": {29, 41}, "Magenta": {63, 69}, "Magda": {65, 72}, "Tim": {81}, "Osvaldo": {87}, "Celestine": {89, 99},
	}
	high := v.name == "Tim" || v.name == "Osvaldo" || v.name == "Celestine"
	for _, p := range i7[v.name] {
		vel := 72
		if high {
			vel = 124
		}
		add(p, eyes, end, vel)
	}
	return out
}

// SMF writing.
func vlq(b *bytes.Buffer, x int) {
	var tmp [5]byte
	i := len(tmp) - 1
	tmp[i] = byte(x & 0x7f)
	for x >>= 7; x > 0; x >>= 7 {
		i--
		tmp[i] = byte(x&0x7f | 0x80)
	}
	b.Write(tmp[i:])
}

type event struct {
	t    int
	data []byte
}

func track(evs []event) []byte {
	var b bytes.Buffer
	last := 0
	// stable insertion sort by time; note-offs (0x8n) before note-ons at the same tick
	for i := 1; i < len(evs); i++ {
		for j := i; j > 0 && (evs[j].t < evs[j-1].t || evs[j].t == evs[j-1].t && evs[j].data[0]&0xf0 == 0x80 && evs[j-1].data[0]&0xf0 == 0x90); j-- {
			evs[j], evs[j-1] = evs[j-1], evs[j]
		}
	}
	for _, e := range evs {
		vlq(&b, e.t-last)
		last = e.t
		b.Write(e.data)
	}
	vlq(&b, 0)
	b.Write([]byte{0xff, 0x2f, 0x00})
	var out bytes.Buffer
	out.WriteString("MTrk")
	binary.Write(&out, binary.BigEndian, uint32(b.Len()))
	out.Write(b.Bytes())
	return out.Bytes()
}

func meta(kind byte, data []byte) []byte {
	b := []byte{0xff, kind}
	var l bytes.Buffer
	vlq(&l, len(data))
	return append(append(b, l.Bytes()...), data...)
}

func main() {
	path := flag.String("o", "sound/data/fugue.mid", "the MIDI file to write")
	force := flag.Bool("force", false, "overwrite an existing file (the user's edits are lost)")
	flag.Parse()
	if _, err := os.Stat(*path); err == nil && !*force {
		log.Fatalf("%s exists: it may hold the user's edits; -force to overwrite", *path)
	}
	var b bytes.Buffer
	b.WriteString("MThd")
	binary.Write(&b, binary.BigEndian, []uint32{6})
	binary.Write(&b, binary.BigEndian, []uint16{1, uint16(len(voices) + 2), tpq})
	coda := loopBars * 4 * tpq
	b.Write(track([]event{
		{0, meta(0x03, []byte("Weird Guests fugue"))},
		{0, meta(0x51, []byte{0x0f, 0x42, 0x40})}, // 60 BPM
		{0, meta(0x58, []byte{4, 2, 24, 8})},      // 4/4
		{0, meta(0x59, []byte{0xff, 0x01})},       // one flat, minor: D minor
		{coda, meta(0x06, []byte("coda"))},        // the marker the game jumps to
	}))
	parts := compose()
	for ch := range parts {
		v := voices[ch%len(voices)]
		name := v.name
		if ch == len(voices) {
			name = sound.RealConcierge
		}
		evs := []event{{0, meta(0x03, []byte(name))}, {0, []byte{byte(0xc0 | ch), byte(v.program)}}}
		for _, n := range parts[ch] {
			evs = append(evs, event{n.on, []byte{byte(0x90 | ch), byte(n.pitch), byte(n.vel)}}, event{n.off, []byte{byte(0x80 | ch), byte(n.pitch), 0}})
		}
		b.Write(track(evs))
	}
	if err := os.WriteFile(*path, b.Bytes(), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote %s: %d voices, %d bars + coda\n", *path, len(voices), loopBars)
}
