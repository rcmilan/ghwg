package sound

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func renderFixtures(t *testing.T) (*Score, *Bank) {
	t.Helper()
	data, err := Data.ReadFile("data/fugue.mid")
	if err != nil {
		t.Fatal(err)
	}
	sc, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	b, err := LoadBank(Data)
	if err != nil {
		t.Fatal(err)
	}
	return sc, b
}

func pcmPeak(pcm []float32) float32 {
	var p float32
	for _, s := range pcm {
		p = max(p, s, -s)
	}
	return p
}

func TestRenderLengthAndSound(t *testing.T) {
	sc, b := renderFixtures(t)
	var all [Voices]bool
	for v := range all {
		all[v] = true
	}
	pcm := Render(sc, b, all, 0, 2*BarBeats, false)
	want := 2*BarBeats*BeatFrames*Rate/60 + int(TailSecs*Rate)
	if len(pcm) != want {
		t.Errorf("len %d, want %d", len(pcm), want)
	}
	if pcmPeak(pcm) < 0.01 {
		t.Errorf("two bars with every voice are silent (peak %g)", pcmPeak(pcm))
	}
	if p := pcmPeak(Render(sc, b, [Voices]bool{}, 0, 2*BarBeats, false)); p != 0 {
		t.Errorf("no voices on, peak %g", p)
	}
}

func TestRenderCodaEnds(t *testing.T) {
	sc, b := renderFixtures(t)
	var all [Voices]bool
	for v := range all {
		all[v] = true
	}
	pcm := RenderCoda(sc, b, all, 0, false)
	if min := int((sc.End-sc.Coda)*BeatFrames*Rate/60) + int(TailSecs*Rate); len(pcm) < min || pcmPeak(pcm) < 0.01 {
		t.Errorf("coda render: len %d (at least %d), peak %g", len(pcm), min, pcmPeak(pcm))
	}
}

func TestWriteWAV(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteWAV(&buf, []float32{0, 1, -1, 2}); err != nil {
		t.Fatal(err)
	}
	h := buf.Bytes()
	le := binary.LittleEndian
	if len(h) != 44+8 || string(h[:4]) != "RIFF" || string(h[8:16]) != "WAVEfmt " || string(h[36:40]) != "data" {
		t.Fatalf("bad header or size %d", len(h))
	}
	if le.Uint32(h[4:]) != uint32(len(h)-8) || le.Uint32(h[40:]) != 8 {
		t.Errorf("sizes: riff %d data %d", le.Uint32(h[4:]), le.Uint32(h[40:]))
	}
	if le.Uint16(h[20:]) != 1 || le.Uint16(h[22:]) != 1 || le.Uint32(h[24:]) != Rate || le.Uint16(h[34:]) != 16 {
		t.Error("format is not 16-bit PCM mono at Rate")
	}
	if int16(le.Uint16(h[46:])) != 32767 || int16(le.Uint16(h[48:])) != -32767 || int16(le.Uint16(h[50:])) != 32767 { // 2 clamps to 1
		t.Error("samples not clamped")
	}
}
