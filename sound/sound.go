// Package sound is 09's audio: one mixer behind one Ebitengine player, the score read from a MIDI file, a sample
// bank and a sequencer that walks the score at the music position the game gives it (context/09-sound/).
//
// Nothing here knows the game: the game (game/sound.go) decides each frame where the music is, which voices are
// heard and how loud, and which effects play. The package runs headless (tests, -render) without an audio
// context; only Open touches Ebitengine's audio.
package sound

import (
	"embed"
	"math"
)

// Rate is the mixer's sample rate (T398): 22050 Hz halves the mixing cost in wasm.
// ponytail: raise to 44100 if the live listen finds it dull.
const Rate = 22050

// Voice is one line of the fugue, in the MIDI file's track order (decisions.md "Voices and instruments").
type Voice int

const (
	Concierge Voice = iota
	Magenta
	Magda
	Celestine
	Tim
	Osvaldo
	Voices // the count
)

// VoiceNames are the MIDI track names, by Voice.
var VoiceNames = [Voices]string{"Concierge", "Magenta", "Magda", "Celestine", "Tim", "Osvaldo"}

// Ghost: the voice belongs to the ghost realm (the Concierge plays in both).
func (v Voice) Ghost() bool { return v == Celestine || v == Tim || v == Osvaldo }

// Channels of the mixer: one per Voice, then the effects channel.
const (
	SFX      = int(Voices)
	Channels = SFX + 1
)

// Bus is a volume slider: Music (the voices) or Effects (the SFX channel).
type Bus int

const (
	Music Bus = iota
	Effects
)

func busOf(ch int) Bus {
	if ch == SFX {
		return Effects
	}
	return Music
}

// Time: a 60 BPM ballad at ×1, a beat is 6 game minutes (decisions.md "Time"; live test 2, from 120).
const (
	BeatFrames = 60 // game frames a beat
	BarBeats   = 4  // 4/4
	LoopBars   = 20 // the loop in play: 8 game hours
	LoopBeats  = LoopBars * BarBeats
)

// Note is one note of a voice, in beats from the score's start.
type Note struct {
	Pitch      int     // MIDI note number
	Start, Len float64 // beats
	Vel        float32 // 0..1
}

// Score is the parsed MIDI file: each voice's notes, sorted by Start. The loop is beats [0, LoopBeats); the coda
// is [Coda, End), Coda at the `coda` marker (LoopBeats when the file is well formed).
type Score struct {
	Notes     [Voices][]Note
	Real      []Note // the Concierge's line in the real dimension (track RealConcierge); none: Notes[Concierge] in both
	Coda, End float64
}

// RealConcierge names the track of the Concierge's real-dimension line: the humans play in F major over his walk
// there, the ghosts in D minor over the Concierge track (live test 2).
const RealConcierge = "Concierge real"

// Sample is mono PCM at Rate, its pitch Root (MIDI note number).
type Sample struct {
	PCM  []float32
	Root int
}

// RateFor: the playback rate that sounds pitch from a sample recorded at root.
func RateFor(pitch, root int) float32 { return float32(math.Exp2(float64(pitch-root) / 12)) }

// Sine is a test sample: a plucked-ish sine at the given MIDI pitch, secs long (the seam's test tone, and the
// SAMPLER's placeholder until the real samples land).
func Sine(pitch int, secs float64) *Sample {
	n := int(secs * Rate)
	s := &Sample{PCM: make([]float32, n), Root: pitch}
	f := 440 * math.Exp2(float64(pitch-69)/12)
	for i := range s.PCM {
		t := float64(i) / Rate
		s.PCM[i] = float32(math.Sin(2*math.Pi*f*t) * math.Exp(-3*t) * min(1, t*200))
	}
	return s
}

// Bell: a struck bell at pitch (Risset's partials, the higher ones dying sooner), peak 1, fading to nothing.
func Bell(pitch int, secs float64) *Sample {
	parts := [][3]float64{{.56, 1, 1}, {.92, .67, .65}, {1.19, 1, .55}, {1.7, 1.8, .325}, {2, 2.67, .35}, {2.74, 1.67, .25}, {3, 1.46, .2}, {3.76, 1.33, .15}, {4.07, 1.33, .1}}
	n := int(secs * Rate)
	s := &Sample{PCM: make([]float32, n), Root: pitch}
	f := 440 * math.Exp2(float64(pitch-69)/12)
	peak := 0.0
	x := make([]float64, n)
	for i := range x {
		t := float64(i) / Rate
		for _, p := range parts {
			x[i] += p[1] * math.Sin(2*math.Pi*f*p[0]*t) * math.Exp(-4*t/(secs*p[2]))
		}
		x[i] *= min(1, t*500) * (1 - t/secs)
		peak = max(peak, math.Abs(x[i]))
	}
	for i := range x {
		s.PCM[i] = float32(x[i] / peak)
	}
	return s
}

// Data holds the shipped sound files (data/README.md: the names the bank and the score expect).
//
//go:embed data
var Data embed.FS
