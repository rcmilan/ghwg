package sound

import (
	"encoding/binary"
	"io"
	"math"
)

// TailSecs is the end of a render: the last notes' release rings out before the file stops.
const TailSecs = 0.3

// Render mixes the score the way the game would drive it: a Sequencer advancing 1/BeatFrames of a beat each
// 60 Hz frame, the Mixer read Rate/60 samples per frame. Only the voices in `on` are heard; the result is mono
// PCM at Rate (-1..1), beats long from beat `from` of the loop, plus TailSecs.
func Render(sc *Score, b *Bank, on [Voices]bool, from, beats float64, ghost bool) []float32 {
	m, q := rig(sc, b, on, ghost)
	q.Seek(from)
	return run(m, q, beats, false)
}

// RenderCoda is Render of the coda alone: the loop's last instant, then the coda from its first bar. beats<=0
// plays it out to its end.
func RenderCoda(sc *Score, b *Bank, on [Voices]bool, beats float64, ghost bool) []float32 {
	m, q := rig(sc, b, on, ghost)
	q.Seek(sc.Coda - 1e-6)
	q.PlayCoda()
	return run(m, q, beats, true)
}

func rig(sc *Score, b *Bank, on [Voices]bool, ghost bool) (*Mixer, *Sequencer) {
	m := NewMixer()
	for v := range Voices {
		if !on[v] {
			m.gain[v] = ramp{} // silent from the first sample, no fade-in from full
		}
	}
	q := NewSequencer(sc, m, b)
	q.SetGhost(ghost)
	return m, q
}

func run(m *Mixer, q *Sequencer, beats float64, untilDone bool) []float32 {
	var pcm []float32
	read := func(n int) {
		buf := make([]byte, n*8)
		m.Read(buf)
		for i := range n {
			pcm = append(pcm, math.Float32frombits(binary.LittleEndian.Uint32(buf[i*8:])))
		}
	}
	frames := int(math.Ceil(beats * BeatFrames))
	done := 0 // samples read so far, to spread Rate/60 (367.5) over the frames without drift
	for f := 1; !q.Done() && (f <= frames || untilDone && beats <= 0); f++ {
		q.Advance(1.0 / BeatFrames)
		n := f*Rate/60 - done
		done += n
		read(n)
	}
	q.Seek(q.Beat()) // let go of whatever is held, then the release rings out
	read(int(TailSecs * Rate))
	return pcm
}

// WriteWAV writes pcm (-1..1, clamped) as a 16-bit PCM mono WAV at Rate.
func WriteWAV(w io.Writer, pcm []float32) error {
	n := uint32(len(pcm) * 2)
	h := make([]byte, 44, 44+n)
	copy(h, "RIFF")
	binary.LittleEndian.PutUint32(h[4:], 36+n)
	copy(h[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(h[16:], 16)
	binary.LittleEndian.PutUint16(h[20:], 1) // PCM
	binary.LittleEndian.PutUint16(h[22:], 1) // mono
	binary.LittleEndian.PutUint32(h[24:], Rate)
	binary.LittleEndian.PutUint32(h[28:], Rate*2)
	binary.LittleEndian.PutUint16(h[32:], 2)
	binary.LittleEndian.PutUint16(h[34:], 16)
	copy(h[36:], "data")
	binary.LittleEndian.PutUint32(h[40:], n)
	for _, s := range pcm {
		h = binary.LittleEndian.AppendUint16(h, uint16(int16(max(-1, min(1, s))*32767)))
	}
	_, err := w.Write(h)
	return err
}
