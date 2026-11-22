package sound

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"math/rand/v2"
	"path"
	"sort"
	"strconv"
	"strings"

	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/vorbis"
)

// Instrument is a sample set (decisions.md "Voices and instruments"); BassWet is the Concierge's ghost treatment,
// the bass with long reverb baked in.
type Instrument int

const (
	Bass Instrument = iota
	BassWet
	Sax
	Vibes
	Celesta
	Trumpet
	Whistle
	Instruments // the count
)

// InstrumentNames are the file name prefixes in data/ (data/README.md).
var InstrumentNames = [Instruments]string{"bass", "basswet", "sax", "vibes", "celesta", "trumpet", "whistle"}

// Cast: each Voice's instrument in real (the Concierge turns to BassWet in ghost: Sequencer.SetGhost).
var Cast = [Voices]Instrument{Bass, Sax, Vibes, Celesta, Trumpet, Whistle}

// Bank holds every sample, decoded once.
type Bank struct {
	Inst  [Instruments][]*Sample // each sorted by Root
	Steps []*Sample              // footsteps, picked at random
}

// LoadBank decodes data/*.ogg (data/README.md names them) into a Bank; an instrument with no file plays
// SineBank's placeholder, so the game sounds before the samples land. A malformed file is an error naming it.
func LoadBank(fsys fs.FS) (*Bank, error) {
	l, err := NewBankLoader(fsys)
	for err == nil {
		var b *Bank
		if b, err = l.Step(); b != nil {
			return b, nil
		}
	}
	return nil, err
}

// BankLoader decodes the bank a file at a time: in wasm the whole bank takes about 2 s of Vorbis decoding, so
// the game spreads it over frames (the title waits for a key anyway) instead of freezing its first frame.
type BankLoader struct {
	fsys  fs.FS
	files []string
	b     *Bank
}

// NewBankLoader lists the files, the bass first: the title plays the Concierge alone.
func NewBankLoader(fsys fs.FS) (*BankLoader, error) {
	names, err := fs.Glob(fsys, "data/*.ogg")
	if err != nil {
		return nil, err
	}
	first := func(f string) bool { return strings.HasPrefix(path.Base(f), "bass") }
	sort.SliceStable(names, func(i, j int) bool { return first(names[i]) && !first(names[j]) })
	return &BankLoader{fsys: fsys, files: names, b: &Bank{}}, nil
}

// Step decodes the next file; once every file is in it returns the finished Bank (the placeholders filling
// what is missing), nil until then.
func (l *BankLoader) Step() (*Bank, error) {
	b := l.b
	if len(l.files) > 0 {
		file := l.files[0]
		l.files = l.files[1:]
		name := strings.TrimSuffix(path.Base(file), ".ogg")
		s, err := decode(l.fsys, file)
		if err != nil {
			return nil, fmt.Errorf("sound: %s: %w", file, err)
		}
		in, root, ok := parseName(name)
		switch {
		case ok && in < 0:
			b.Steps = append(b.Steps, s)
		case ok:
			s.Root = root
			b.Inst[in] = append(b.Inst[in], s)
		default:
			return nil, fmt.Errorf("sound: %s: not a name data/README.md knows", file)
		}
		if len(l.files) > 0 {
			return nil, nil
		}
	}
	sine := SineBank()
	for i := range b.Inst {
		if len(b.Inst[i]) == 0 {
			b.Inst[i] = sine.Inst[i]
		}
		sort.Slice(b.Inst[i], func(x, y int) bool { return b.Inst[i][x].Root < b.Inst[i][y].Root })
	}
	if len(b.Steps) == 0 {
		b.Steps = sine.Steps
	}
	return b, nil
}

// parseName reads `<instrument>_<midi>` (root set), or `step_<n>` (in -1).
func parseName(name string) (in Instrument, root int, ok bool) {
	pre, num, found := strings.Cut(name, "_")
	n, err := strconv.Atoi(num)
	if !found || err != nil {
		return 0, 0, false
	}
	if pre == "step" {
		return -1, 0, true
	}
	for i, nm := range InstrumentNames {
		if nm == pre {
			return Instrument(i), n, true
		}
	}
	return 0, 0, false
}

// decode reads one Ogg as mono float32 at Rate: the stereo stream is resampled, then L and R are averaged.
func decode(fsys fs.FS, file string) (*Sample, error) {
	f, err := fsys.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	st, err := vorbis.DecodeF32(f)
	if err != nil {
		return nil, err
	}
	raw, err := io.ReadAll(audio.ResampleReaderF32(st, st.Length(), st.SampleRate(), Rate))
	if err != nil {
		return nil, err
	}
	if len(raw) < 8 {
		return nil, errors.New("no audio")
	}
	pcm := make([]float32, len(raw)/8)
	for i := range pcm {
		l := math.Float32frombits(binary.LittleEndian.Uint32(raw[i*8:]))
		r := math.Float32frombits(binary.LittleEndian.Uint32(raw[i*8+4:]))
		pcm[i] = (l + r) / 2
	}
	return &Sample{PCM: pcm}, nil
}

// SineBank: placeholder sines one octave apart for every instrument, a noise tick for a step.
func SineBank() *Bank {
	b := &Bank{}
	for i := range b.Inst {
		for p := 36; p <= 96; p += 12 {
			b.Inst[i] = append(b.Inst[i], Sine(p, 1.5))
		}
	}
	tick := &Sample{PCM: make([]float32, Rate/20), Root: 60}
	r := rand.New(rand.NewPCG(1, 2))
	for i := range tick.PCM {
		tick.PCM[i] = float32((r.Float64()*2 - 1) * math.Exp(-float64(i)/float64(Rate/200)))
	}
	b.Steps = []*Sample{tick}
	return b
}

// Pick: the sample of in nearest to pitch, and the rate that sounds pitch from it (nil if none).
func (b *Bank) Pick(in Instrument, pitch int) (*Sample, float32) {
	var best *Sample
	for _, s := range b.Inst[in] {
		if best == nil || abs(s.Root-pitch) < abs(best.Root-pitch) {
			best = s
		}
	}
	if best == nil {
		return nil, 0
	}
	return best, RateFor(pitch, best.Root)
}

func abs(x int) int { return max(x, -x) }
