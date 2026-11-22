package sound

import (
	"math"
	"sort"
)

// Sequencer walks a Score at the music position the game gives it and plays its notes through the Mixer, each
// voice on its own channel (T402 SAMPLER). The loop is [0, LoopBeats); after PlayCoda it plays the coda once
// at the next bar line, then stops. The game goroutine is its only caller.
type Sequencer struct {
	score *Score
	mix   *Mixer
	bank  *Bank
	beat  float64
	ghost bool

	on      [Voices][]sounding // the notes held right now, per voice
	pending bool               // PlayCoda asked, the bar line not reached
	inCoda  bool
	done    bool
}

type sounding struct {
	id  ID
	end float64
}

// NewSequencer: at beat 0, real treatment.
func NewSequencer(s *Score, m *Mixer, b *Bank) *Sequencer {
	return &Sequencer{score: s, mix: m, bank: b}
}

// Beat: the music position, in beats (within the loop, or past Coda while the coda plays).
func (q *Sequencer) Beat() float64 { return q.beat }

// Done: the coda has played out; Advance does nothing from here on.
func (q *Sequencer) Done() bool { return q.done }

// Seek jumps to beat (taken modulo the loop) without playing what lies between; sounding notes are released.
// During the coda it ends the coda and goes back to the loop; a coda asked for and not yet started is dropped.
func (q *Sequencer) Seek(beat float64) {
	q.releaseAll()
	q.beat = math.Mod(math.Mod(beat, LoopBeats)+LoopBeats, LoopBeats)
	q.inCoda, q.done, q.pending = false, false, false
}

// Advance moves the position on by beats (any size, never negative), starting every note whose Start it crosses
// and releasing every note whose end it crosses; the tempo is whatever the game's beats per frame make it.
// More than a bar at once (a lag spike, -scale 4 never gets there) is a Seek: no burst of notes.
func (q *Sequencer) Advance(d float64) {
	if q.done || d <= 0 {
		return
	}
	to := q.beat + d
	if q.inCoda {
		q.coda(to)
		return
	}
	if q.pending {
		if bar := math.Ceil(q.beat/BarBeats) * BarBeats; to >= bar {
			if d <= BarBeats {
				q.play(q.beat, bar)
			}
			q.releaseAll()
			q.pending, q.inCoda = false, true
			q.beat = q.score.Coda
			q.coda(q.beat + to - bar)
			return
		}
	}
	switch {
	case d > BarBeats:
		q.Seek(to)
	case to < LoopBeats:
		q.play(q.beat, to)
		q.beat = to
	default:
		q.play(q.beat, LoopBeats)
		q.releaseAll() // notes that run past the loop's end
		q.beat = to - LoopBeats
		q.play(0, q.beat) // a note at 0 starts right after the wrap
	}
}

// coda moves the coda's position to `to`.
func (q *Sequencer) coda(to float64) {
	from := q.beat
	if to-from > BarBeats {
		q.releaseAll()
	} else {
		q.play(from, min(to, q.score.End))
	}
	q.beat = to
	if to >= q.score.End {
		q.releaseAll()
		q.done = true
	}
}

// PlayCoda: at the next bar line the loop gives way to the coda, played once.
func (q *Sequencer) PlayCoda() {
	if !q.inCoda && !q.done {
		q.pending = true
	}
}

// StartCoda: the coda from its first beat, now (the finale's fanfare starts with the scene).
func (q *Sequencer) StartCoda() {
	q.releaseAll()
	q.beat, q.inCoda, q.done, q.pending = q.score.Coda, true, false, false
}

// SetGhost picks the Concierge's treatment and line: the dry bass on Score.Real in real, BassWet on his own track
// in ghost (from the next note on).
func (q *Sequencer) SetGhost(ghost bool) { q.ghost = ghost }

// play starts the notes with Start in [from, to) and releases every held note that ends by `to`. Half-open, so
// consecutive moves split the line with nothing played twice or skipped: a note exactly on a frame's position
// (at ×1 every eighth is, 15 frames a beat-half) starts on the move that begins there.
func (q *Sequencer) play(from, to float64) {
	for v := range Voices {
		notes := q.score.Notes[v]
		in := Cast[v]
		if v == Concierge && q.ghost {
			in = BassWet
		}
		if v == Concierge && !q.ghost && len(q.score.Real) > 0 {
			notes = q.score.Real // the F major walk under the humans
		}
		i := sort.Search(len(notes), func(i int) bool { return notes[i].Start >= from })
		for ; i < len(notes) && notes[i].Start < to; i++ {
			n := notes[i]
			s, rate := q.bank.Pick(in, n.Pitch)
			if id := q.mix.Play(s, int(v), rate, n.Vel); id != 0 {
				q.on[v] = append(q.on[v], sounding{id, n.Start + n.Len})
			}
		}
		held := q.on[v][:0]
		for _, h := range q.on[v] {
			if h.end <= to {
				q.mix.Release(h.id)
			} else {
				held = append(held, h)
			}
		}
		q.on[v] = held
	}
}

func (q *Sequencer) releaseAll() {
	for v := range q.on {
		q.mix.ReleaseAll(v)
		q.on[v] = q.on[v][:0]
	}
}
