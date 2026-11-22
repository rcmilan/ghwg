package sound

import (
	"fmt"
	"strings"
)

// Ranges are each instrument's nominal sounding MIDI range, low to high: past it the repitched samples stretch
// too far (data/README.md). Lint warns about notes outside the range of their voice's instrument.
var Ranges = [Instruments][2]int{
	Bass:    {28, 60},
	BassWet: {28, 60},
	Sax:     {49, 81},
	Vibes:   {53, 89},
	Celesta: {60, 108},
	Trumpet: {54, 86},
	Whistle: {67, 98},
}

var noteNames = [12]string{"C", "C#", "D", "Eb", "E", "F", "F#", "G", "Ab", "A", "Bb", "B"}

// NoteName: MIDI 60 is C4.
func NoteName(p int) string { return fmt.Sprintf("%s%d", noteNames[(p%12+12)%12], p/12-1) }

func at(beat float64) string {
	return fmt.Sprintf("bar %d beat %d", int(beat)/BarBeats+1, int(beat)%BarBeats+1)
}

// Lint lists the score's problems, one per line, prefixed `error:` (the game cannot play it right) or `warn:`
// (a judgment call, e.g. a jazz chromatic). names are every track name in the file (ParseTracks).
func Lint(sc *Score, names []string) []string {
	var out []string
	add := func(f string, a ...any) { out = append(out, fmt.Sprintf(f, a...)) }
	for _, n := range names {
		known := strings.EqualFold(n, RealConcierge)
		for _, vn := range VoiceNames {
			known = known || strings.EqualFold(n, vn)
		}
		if !known {
			add("error: unknown track name %q", n)
		}
	}
	for v, vn := range VoiceNames {
		if len(sc.Notes[v]) == 0 {
			add("error: no notes for voice %s (missing or empty track)", vn)
		}
	}
	if sc.Coda != LoopBeats {
		add("error: coda marker missing or at beat %g, want exactly %d (%d bars)", sc.Coda, LoopBeats, LoopBars)
	}
	for v, vn := range VoiceNames {
		r := Ranges[Cast[v]]
		for _, n := range sc.Notes[v] {
			if n.Start < sc.Coda && sc.Coda < n.Start+n.Len-1e-9 {
				add("error: %s %s %s crosses the coda line", vn, at(n.Start), NoteName(n.Pitch))
			}
			if n.Pitch < r[0] || n.Pitch > r[1] {
				add("warn: %s %s %s is outside %s's range %s-%s", vn, at(n.Start), NoteName(n.Pitch),
					InstrumentNames[Cast[v]], NoteName(r[0]), NoteName(r[1]))
			}
		}
	}
	parts, names := append(sc.Notes[:], sc.Real), append(VoiceNames[:], RealConcierge)
	for v := range parts { // the fugue's rule, in the loop (the coda's chords are block chords)
		for w := v + 1; w < len(parts); w++ {
			if ghostSide(v, len(parts)) != ghostSide(w, len(parts)) {
				continue // the realms never sound together in the loop (his two lines included)
			}
			for k := 1; float64(k)/2 < sc.Coda; k++ {
				t0, t1 := float64(k-1)/2+lintLag, float64(k)/2+lintLag
				if Parallel(Sounding(parts[v], t0), Sounding(parts[v], t1), Sounding(parts[w], t0), Sounding(parts[w], t1)) {
					add("warn: %s and %s move in parallel fifths or octaves into %s", names[v], names[w], at(float64(k)/2))
				}
			}
		}
	}
	return out
}

// ghostSide: part i of n (the voices, then the Concierge's real line) plays in the ghost dimension.
func ghostSide(i, n int) bool { return i < n-1 && (Voice(i) == Concierge || Voice(i).Ghost()) }

// lintLag: the parallels are read a little after each eighth, so swung and straight eighths line up.
const lintLag = .2

// Sounding: the pitch notes sound at beat t, the top one if several, -1 in a rest.
func Sounding(notes []Note, t float64) int {
	p := -1
	for _, n := range notes {
		if n.Start > t {
			break
		}
		if t < n.Start+n.Len {
			p = max(p, n.Pitch)
		}
	}
	return p
}

// Parallel: two voices moving a0->a1 and b0->b1 go from a perfect fifth (or octave, unison) to another, both
// moving. A rest on either side is no motion.
func Parallel(a0, a1, b0, b1 int) bool {
	if a0 < 0 || a1 < 0 || b0 < 0 || b1 < 0 || a0 == a1 || b0 == b1 {
		return false
	}
	i0, i1 := abs(a0-b0)%12, abs(a1-b1)%12
	return i0 == i1 && (i0 == 7 || i0 == 0)
}
