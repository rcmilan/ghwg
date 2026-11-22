package sound

import "testing"

// testBank: one long distinct sample per instrument, so a pick shows which set played.
func testBank() *Bank {
	b := &Bank{}
	for i := range b.Inst {
		b.Inst[i] = []*Sample{Sine(60, 3)}
	}
	return b
}

// fixture: a voice per channel is enough; Magenta (1) holds a note at 0 and one at 8, Concierge one at 4.
func fixture() (*Sequencer, *Mixer, *Bank) {
	sc := &Score{Coda: LoopBeats, End: LoopBeats + 8}
	sc.Notes[Concierge] = []Note{{Pitch: 48, Start: 4, Len: 2, Vel: .5}}
	sc.Notes[Magenta] = []Note{{Pitch: 60, Start: 0, Len: 2, Vel: 1}, {Pitch: 62, Start: 8, Len: 1, Vel: 1}}
	sc.Notes[Tim] = []Note{{Pitch: 72, Start: LoopBeats + 4, Len: 2, Vel: 1}} // in the coda
	m, b := NewMixer(), testBank()
	return NewSequencer(sc, m, b), m, b
}

// settle reads long enough for every released note to fade out.
func settle(m *Mixer) { m.Read(make([]byte, 8*Rate/4)) }

func TestSequencerNotes(t *testing.T) {
	q, m, _ := fixture()
	q.Advance(1) // [0,1): Magenta's note at 0 starts: the music begins on its first beat
	if m.Playing() != 1 {
		t.Fatalf("the note at 0 starts on the first move, got %d", m.Playing())
	}
	q.Advance(4) // [1,5): Concierge starts at 4, Magenta's note ends at 2
	settle(m)
	if m.Playing() != 1 {
		t.Fatalf("one note starts in [1,5), got %d", m.Playing())
	}
	q.Advance(1.5) // [5,6.5): Concierge's ends at 6 (6.5 > 6)
	settle(m)
	if m.Playing() != 0 {
		t.Fatalf("the note ended, got %d", m.Playing())
	}
}

func TestSequencerLoopWrap(t *testing.T) {
	q, m, _ := fixture()
	q.Seek(LoopBeats - 1)
	q.Advance(.5)
	q.Advance(.75) // crosses the wrap: the note at 0 starts right after it
	if q.Beat() != .25 {
		t.Fatalf("beat wraps to .25, got %v", q.Beat())
	}
	if m.Playing() != 1 {
		t.Fatalf("the note at beat 0 restarts after the wrap, got %d", m.Playing())
	}
	q.Advance(2) // ends at 2
	settle(m)
	if m.Playing() != 0 {
		t.Fatalf("released, got %d", m.Playing())
	}
	// landing exactly on the wrap still starts the note at 0 on the next advance
	q.Seek(LoopBeats - .5)
	q.Advance(.5)
	q.Advance(.1)
	if m.Playing() != 1 {
		t.Fatalf("landing on the wrap, got %d", m.Playing())
	}
}

func TestSequencerBigAdvanceIsSeek(t *testing.T) {
	q, m, _ := fixture()
	q.Advance(.5)
	q.Advance(60) // far more than a bar: no notes between, the held one released
	settle(m)
	if m.Playing() != 0 {
		t.Fatalf("no burst, got %d", m.Playing())
	}
	if q.Beat() != 60.5 {
		t.Fatalf("beat %v", q.Beat())
	}
	q.Advance(LoopBeats) // and wraps
	if q.Beat() != 60.5 {
		t.Fatalf("a whole loop lands where it began, got %v", q.Beat())
	}
}

func TestSequencerSeek(t *testing.T) {
	q, m, _ := fixture()
	q.Advance(.1)
	q.Advance(3.9)
	q.Advance(.5) // Concierge sounding
	q.Seek(8)     // a note sits exactly at 8
	settle(m)
	if m.Playing() != 0 {
		t.Fatalf("Seek released the sounding note, got %d", m.Playing())
	}
	q.Seek(-4)
	if q.Beat() != LoopBeats-4 {
		t.Fatalf("modulo, got %v", q.Beat())
	}
	q.Advance(2.5)
	q.Advance(2.5) // 156..161: wraps, restarts the note at 0
	if m.Playing() != 1 {
		t.Fatalf("got %d", m.Playing())
	}
}

func TestSequencerCoda(t *testing.T) {
	q, m, _ := fixture()
	q.Seek(10.5)
	q.PlayCoda()
	q.Advance(1) // 11.5: still the loop
	if q.Beat() != 11.5 || q.Done() {
		t.Fatalf("waits for the bar line, beat %v", q.Beat())
	}
	q.Advance(1) // crosses 12: jumps to the coda, .5 in
	if q.Beat() != LoopBeats+.5 {
		t.Fatalf("beat %v", q.Beat())
	}
	q.Advance(4) // coda 0.5..4.5: Tim's note at 4
	if m.Playing() != 1 {
		t.Fatalf("coda note starts, got %d", m.Playing())
	}
	q.Advance(3.5) // to End
	if !q.Done() {
		t.Fatalf("coda reached its end")
	}
	settle(m)
	if m.Playing() != 0 {
		t.Fatalf("all released, got %d", m.Playing())
	}
	q.Advance(1)
	if m.Playing() != 0 || q.Beat() != LoopBeats+8 {
		t.Fatalf("Advance does nothing after Done, beat %v", q.Beat())
	}
}

func TestSequencerCodaSeekEnds(t *testing.T) {
	q, _, _ := fixture()
	q.PlayCoda()
	q.Advance(4)
	q.Advance(1)
	if q.Beat() < LoopBeats {
		t.Fatalf("in the coda, got %v", q.Beat())
	}
	q.Seek(8)
	q.Advance(1)
	if q.Beat() != 9 || q.Done() {
		t.Fatalf("Seek ends the coda, beat %v", q.Beat())
	}
}

func TestSequencerGhost(t *testing.T) {
	q, m, b := fixture()
	b.Inst[Bass] = []*Sample{Sine(48, 3)}
	b.Inst[BassWet] = []*Sample{Sine(48, 3)}
	q.Advance(.1)
	q.Advance(4)
	if concierge(m) != b.Inst[Bass][0] {
		t.Fatal("real: dry bass")
	}
	q.SetGhost(true)
	q.Seek(0)
	q.Advance(.1)
	q.Advance(4)
	if got := concierge(m); got != b.Inst[BassWet][0] {
		t.Fatal("ghost: BassWet")
	}
}

// concierge: the sample the Concierge's channel started last.
func concierge(m *Mixer) *Sample {
	var s *Sample
	for _, p := range m.playing {
		if p.ch == int(Concierge) {
			s = p.s
		}
	}
	return s
}

func BenchmarkMixerSixVoices(b *testing.B) {
	m := NewMixer()
	s := Sine(60, 3)
	for v := range int(Voices) {
		m.Play(s, v, 1, .5)
		m.Play(s, v, 1.5, .5)
	}
	buf := make([]byte, Rate/60*8)
	b.ResetTimer()
	for range b.N {
		m.Read(buf)
	}
}

// At ×1 the game advances 1/30 beat a frame, so notes on straight and swung eighths sit exactly on a frame's
// position: walking frame by frame must start every one of them, none skipped or doubled.
func TestSequencerFrameSteps(t *testing.T) {
	sc := &Score{Coda: LoopBeats, End: LoopBeats}
	for _, at := range []float64{0, .5, 2.0 / 3, 1, 1.5, 1 + 2.0/3} {
		sc.Notes[Magda] = append(sc.Notes[Magda], Note{Pitch: 70, Start: at, Len: .1, Vel: 1})
	}
	m := NewMixer()
	q := NewSequencer(sc, m, testBank())
	started := 0
	for range 2 * BeatFrames {
		before := m.next
		q.Advance(1.0 / BeatFrames)
		started += int(m.next - before)
	}
	if started != len(sc.Notes[Magda]) {
		t.Fatalf("every note on a frame boundary starts once: %d of %d", started, len(sc.Notes[Magda]))
	}
}

// StartCoda jumps into the coda at once and plays its first notes on the next move.
func TestStartCoda(t *testing.T) {
	sc := &Score{Coda: LoopBeats, End: LoopBeats + 4}
	sc.Notes[Concierge] = []Note{{Pitch: 36, Start: 1, Len: 1, Vel: 1}, {Pitch: 41, Start: LoopBeats, Len: 2, Vel: 1}}
	m := NewMixer()
	q := NewSequencer(sc, m, SineBank())
	q.Seek(10)
	q.StartCoda()
	q.Advance(0.1)
	if q.Beat() != LoopBeats+0.1 || m.Playing() != 1 {
		t.Fatalf("beat %v, %d playing; want the coda's first note", q.Beat(), m.Playing())
	}
	q.Advance(4)
	if !q.Done() {
		t.Error("the coda never ended")
	}
}

// In real the Concierge walks Score.Real (the F major changes), in ghost his own track.
func TestSequencerRealLine(t *testing.T) {
	sc := &Score{Coda: LoopBeats, End: LoopBeats}
	sc.Notes[Concierge] = []Note{{Pitch: 38, Start: 0, Len: 1, Vel: 1}}
	sc.Real = []Note{{Pitch: 41, Start: 0, Len: 1, Vel: 1}, {Pitch: 43, Start: 1, Len: 1, Vel: 1}}
	for _, c := range []struct {
		ghost bool
		want  int
	}{{false, 2}, {true, 1}} {
		m := NewMixer()
		q := NewSequencer(sc, m, SineBank())
		q.SetGhost(c.ghost)
		q.Advance(1.5)
		if n := m.Playing(); n != c.want {
			t.Errorf("ghost %v: %d notes started, want %d", c.ghost, n, c.want)
		}
	}
}
