package sound

import (
	"encoding/binary"
	"math"
	"sync"
)

// Mixer is a small polyphonic sampler: every note and effect plays through it, and it is the one io.Reader
// behind the one Ebitengine player (32-bit float, little endian, stereo). The game goroutine calls Play,
// Release and the gains; the audio side calls Read; a mutex keeps them apart.
type Mixer struct {
	mu      sync.Mutex
	playing []playing
	gain    [Channels]ramp
	bus     [2]float32
	speed   float64 // the music channels' tape speed: tempo and pitch together (Father P's race)
	next    ID
}

// ID names a playing sound, for Release.
type ID uint32

type playing struct {
	id      ID
	s       *Sample
	pos     float64 // into s.PCM, fractional
	rate    float64
	gain    float32
	ch      int
	release bool    // fading out
	env     float32 // 1 while held, falling to 0 over releaseSecs once released
}

// ramp: a channel gain that moves toward its target over rampSecs, so a voice fading in or out never clicks.
type ramp struct{ now, to float32 }

const (
	releaseSecs = 0.08
	rampSecs    = 0.05
	maxPlaying  = 32  // past it the oldest sound is dropped
	headroom    = 0.5 // the master gain before the clamp
)

// NewMixer: every channel and bus at full gain.
func NewMixer() *Mixer {
	m := &Mixer{bus: [2]float32{1, 1}, speed: 1}
	for i := range m.gain {
		m.gain[i] = ramp{1, 1}
	}
	return m
}

// Play starts s on channel ch at rate (1 sounds its Root; RateFor gives another pitch) and gain; it plays to its
// end unless released.
func (m *Mixer) Play(s *Sample, ch int, rate, gain float32) ID {
	if s == nil || len(s.PCM) == 0 {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.playing) >= maxPlaying {
		m.playing = m.playing[1:]
	}
	m.next++
	m.playing = append(m.playing, playing{id: m.next, s: s, rate: float64(rate), gain: gain, ch: ch, env: 1})
	return m.next
}

// Release fades the sound out over releaseSecs (a note-off).
func (m *Mixer) Release(id ID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.playing {
		if m.playing[i].id == id {
			m.playing[i].release = true
		}
	}
}

// ReleaseAll releases every sound on channel ch (a voice cut, a jump in the score).
func (m *Mixer) ReleaseAll(ch int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.playing {
		if m.playing[i].ch == ch {
			m.playing[i].release = true
		}
	}
}

// SetGain sets channel ch's gain (0..1), reached over rampSecs.
func (m *Mixer) SetGain(ch int, g float32) {
	m.mu.Lock()
	m.gain[ch].to = g
	m.mu.Unlock()
}

// SetBus sets a slider's gain (0..1), at once.
func (m *Mixer) SetBus(b Bus, g float32) {
	m.mu.Lock()
	m.bus[b] = g
	m.mu.Unlock()
}

// SetSpeed sets the music channels' tape speed (1 normal, 2 an octave up and twice as fast; effects keep 1).
// The sequencer's tempo is the game's to match.
func (m *Mixer) SetSpeed(r float64) {
	m.mu.Lock()
	m.speed = r
	m.mu.Unlock()
}

// Playing: how many sounds are on (tests, the benchmark).
func (m *Mixer) Playing() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.playing)
}

// Read mixes the next len(p)/8 stereo frames into p. It never returns an error: silence is still audio.
func (m *Mixer) Read(p []byte) (int, error) {
	frames := len(p) / 8
	m.mu.Lock()
	defer m.mu.Unlock()
	step := float32(1 / (rampSecs * Rate))
	fall := float32(1 / (releaseSecs * Rate))
	for f := range frames {
		var g [Channels]float32
		for c := range m.gain {
			r := &m.gain[c]
			switch {
			case r.now < r.to:
				r.now = min(r.to, r.now+step)
			case r.now > r.to:
				r.now = max(r.to, r.now-step)
			}
			g[c] = r.now * m.bus[busOf(c)]
		}
		var sum float32
		for i := range m.playing {
			v := &m.playing[i]
			k := int(v.pos)
			if k+1 >= len(v.s.PCM) || v.env <= 0 {
				v.env = -1 // done
				continue
			}
			fr := float32(v.pos - float64(k))
			x := v.s.PCM[k]*(1-fr) + v.s.PCM[k+1]*fr
			sum += x * v.gain * v.env * g[v.ch]
			if v.ch < int(Voices) {
				v.pos += v.rate * m.speed
			} else {
				v.pos += v.rate
			}
			if v.release {
				v.env -= fall
			}
		}
		sum = max(-1, min(1, sum*headroom))
		bits := math.Float32bits(sum)
		binary.LittleEndian.PutUint32(p[f*8:], bits)
		binary.LittleEndian.PutUint32(p[f*8+4:], bits)
	}
	live := m.playing[:0]
	for _, v := range m.playing {
		if v.env >= 0 {
			live = append(live, v)
		}
	}
	m.playing = live
	return frames * 8, nil
}
