package sound

import (
	"encoding/binary"
	"math"
	"testing"
)

// peak: the loudest sample in n frames read from m.
func peak(m *Mixer, n int) float32 {
	buf := make([]byte, n*8)
	m.Read(buf)
	var p float32
	for i := 0; i < len(buf); i += 4 {
		p = max(p, float32(math.Abs(float64(math.Float32frombits(binary.LittleEndian.Uint32(buf[i:]))))))
	}
	return p
}

func TestMixer(t *testing.T) {
	m := NewMixer()
	if p := peak(m, 100); p != 0 {
		t.Fatalf("an idle mixer is silent, got %v", p)
	}
	id := m.Play(Sine(69, .5), 0, 1, 1)
	if p := peak(m, 2000); p < .1 {
		t.Fatalf("a playing sine is heard, peak %v", p)
	}
	m.SetBus(Music, 0)
	if p := peak(m, 100); p != 0 {
		t.Fatalf("the Music bus at 0 mutes channel 0, got %v", p)
	}
	m.SetBus(Music, 1)
	m.Release(id)
	peak(m, int(releaseSecs*Rate)+10)
	if n := m.Playing(); n != 0 {
		t.Fatalf("a released note is gone after the release, %d left", n)
	}
	m.Play(Sine(69, .01), SFX, 1, 1)
	peak(m, Rate/50)
	if n := m.Playing(); n != 0 {
		t.Fatalf("a sound ends with its sample, %d left", n)
	}
	for range maxPlaying + 5 {
		m.Play(Sine(60, 1), 1, 1, .1)
	}
	if n := m.Playing(); n != maxPlaying {
		t.Fatalf("at most %d sounds, got %d", maxPlaying, n)
	}
}

func TestRateFor(t *testing.T) {
	if r := RateFor(72, 60); math.Abs(float64(r)-2) > 1e-6 {
		t.Fatalf("an octave up plays twice as fast, got %v", r)
	}
	b := SineBank()
	if s, r := b.Pick(Sax, 62); s.Root != 60 || math.Abs(float64(r)-float64(RateFor(62, 60))) > 1e-6 {
		t.Fatalf("Pick takes the nearest root, got %d at %v", s.Root, r)
	}
}

// The tape speed runs the music channels faster, effects at their own rate.
func TestMixerSpeed(t *testing.T) {
	m := NewMixer()
	s := &Sample{PCM: make([]float32, 100), Root: 60}
	m.Play(s, int(Concierge), 1, 1)
	m.Play(s, SFX, 1, 1)
	m.SetSpeed(2)
	m.Read(make([]byte, 60*8))
	if n := m.Playing(); n != 1 {
		t.Errorf("%d playing after 60 frames at speed 2, want 1 (the effect)", n)
	}
}
