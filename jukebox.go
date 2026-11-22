package main

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"ghosthotel/game"
	"ghosthotel/sound"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// The music test harness (09 T404): -music runs the jukebox, -render mixes a WAV. Both are their own thing in
// package main, so they never touch the game's own music logic.

// parseVoices reads -music: "all" or a comma list of voice names, any case.
func parseVoices(s string) ([sound.Voices]bool, error) {
	var on [sound.Voices]bool
	for _, w := range strings.Split(s, ",") {
		w = strings.TrimSpace(w)
		if strings.EqualFold(w, "all") {
			for v := range on {
				on[v] = true
			}
			continue
		}
		v := -1
		for i, n := range sound.VoiceNames {
			if strings.EqualFold(w, n) {
				v = i
			}
		}
		if v < 0 {
			return on, fmt.Errorf("-music %q: want all or voices from %s", w, strings.Join(sound.VoiceNames[:], ", "))
		}
		on[v] = true
	}
	return on, nil
}

// loadScore reads the score from path, or the embedded one when path is empty; it lints it and prints the
// problems, returning the counts of error: and warn: lines.
func loadScore(path string) (sc *sound.Score, errs, warns int, err error) {
	var data []byte
	if path == "" {
		data, err = sound.Data.ReadFile("data/fugue.mid")
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return nil, 0, 0, err
	}
	sc, names, err := sound.ParseTracks(data)
	if err != nil {
		return nil, 0, 0, err
	}
	for _, l := range sound.Lint(sc, names) {
		fmt.Println(l)
		switch {
		case strings.HasPrefix(l, "error:"):
			errs++
		case strings.HasPrefix(l, "warn:"):
			warns++
		}
	}
	return sc, errs, warns, nil
}

// jukebox is the music alone on a screen: one Mixer, one Sequencer, no Game.
type jukebox struct {
	bank *sound.Bank
	mix  *sound.Mixer
	seq  *sound.Sequencer
	sc   *sound.Score

	on          [sound.Voices]bool
	ghost       bool
	playing     bool
	started     bool // the browser starts audio only after a key press
	codaAsked   bool
	scale       float64
	frame       int
	path        string
	mtime       time.Time
	next        *sound.Score // the file changed: it takes over at the next bar line
	errs, warns int
	loadErr     string
}

// tempo: the game's own (game.RealTempo): the real dimension reads the score faster, the coda
// runs at its written pace.
func (j *jukebox) tempo() float64 {
	if j.ghost || j.inCoda() {
		return 1
	}
	return game.RealTempo
}

func runJukebox(voices, midi string, bar int, ghost bool, scale float64) {
	on, err := parseVoices(voices)
	if err != nil {
		log.Fatal(err)
	}
	bank, err := sound.LoadBank(sound.Data)
	if err != nil {
		log.Fatal(err)
	}
	j := &jukebox{bank: bank, mix: sound.NewMixer(), on: on, ghost: ghost, playing: true, scale: scale, path: midi}
	if j.sc, j.errs, j.warns, err = loadScore(midi); err != nil {
		log.Fatal(err)
	}
	if fi, err := os.Stat(midi); err == nil {
		j.mtime = fi.ModTime()
	}
	j.seq = sound.NewSequencer(j.sc, j.mix, bank)
	j.seq.SetGhost(ghost)
	j.seq.Seek(float64(max(bar-1, 0) * sound.BarBeats))
	for v := range j.on {
		j.setVoice(v)
	}
	if err := sound.Open(j.mix); err != nil {
		log.Printf("sound: %v", err)
	}
	ebiten.SetWindowSize(960, 540)
	ebiten.SetWindowTitle(game.Name + " jukebox")
	if err := ebiten.RunGame(j); err != nil {
		log.Fatal(err)
	}
}

func (j *jukebox) setVoice(v int) {
	g := float32(0)
	if j.on[v] {
		g = 1
	}
	j.mix.SetGain(v, g)
}

func (j *jukebox) bar() int { return int(j.seq.Beat() / sound.BarBeats) }

func (j *jukebox) inCoda() bool { return j.codaAsked && j.seq.Beat() >= j.sc.Coda }

// jump goes to the start of the bar d bars from this one, leaving the coda.
func (j *jukebox) jump(d int) {
	j.seq.Seek(float64((j.bar() + d) * sound.BarBeats))
	j.codaAsked = false
}

func (j *jukebox) Update() error {
	for _, k := range inpututil.AppendJustPressedKeys(nil) {
		j.started = true
		switch {
		case k == ebiten.KeyEscape:
			return ebiten.Termination
		case k >= ebiten.Key1 && k < ebiten.Key1+ebiten.Key(sound.Voices):
			v := int(k - ebiten.Key1)
			j.on[v] = !j.on[v]
			j.setVoice(v)
		case k == ebiten.KeyA:
			j.jump(-1)
		case k == ebiten.KeyD:
			j.jump(1)
		case k == ebiten.KeySpace:
			if j.playing = !j.playing; !j.playing {
				for v := range int(sound.Voices) { // nothing hangs while it waits
					j.mix.ReleaseAll(v)
				}
			}
		case k == ebiten.KeyC:
			j.codaAsked = true
			j.seq.PlayCoda()
		case k == ebiten.KeyG:
			j.ghost = !j.ghost
			j.seq.SetGhost(j.ghost)
		}
	}
	j.reload()
	if !j.started || !j.playing {
		return nil
	}
	prev := j.bar()
	j.seq.Advance(j.scale * j.tempo() / sound.BeatFrames)
	if j.seq.Done() { // the coda played out: back to bar 1
		j.seq.Seek(0)
		j.codaAsked = false
	} else if j.bar() != prev && j.next != nil && !j.codaAsked {
		beat := j.seq.Beat()
		j.seq.Seek(beat) // lets go of the old score's held notes
		j.sc, j.next = j.next, nil
		j.seq = sound.NewSequencer(j.sc, j.mix, j.bank)
		j.seq.SetGhost(j.ghost)
		j.seq.Seek(beat)
	}
	return nil
}

// reload polls the -midi file about once a second (a stat is cheap) and parses it when it changed; a file that
// does not parse leaves the old score playing and shows why.
func (j *jukebox) reload() {
	if j.path == "" {
		return
	}
	if j.frame++; j.frame%60 != 0 {
		return
	}
	fi, err := os.Stat(j.path)
	if err != nil || !fi.ModTime().After(j.mtime) {
		return
	}
	j.mtime = fi.ModTime()
	sc, errs, warns, err := loadScore(j.path)
	if err != nil {
		j.loadErr = err.Error()
		fmt.Println("reload:", err)
		return
	}
	j.next, j.errs, j.warns, j.loadErr = sc, errs, warns, ""
}

func (j *jukebox) Draw(screen *ebiten.Image) {
	if !j.started {
		ebitenutil.DebugPrint(screen, "\n\n  PRESS A KEY")
		return
	}
	var b strings.Builder
	for v, n := range sound.VoiceNames {
		state := "off"
		if j.on[v] {
			state = "ON "
		}
		fmt.Fprintf(&b, "  [%d] %-9s %s\n", v+1, n, state)
	}
	treat := "REAL 150 BPM"
	if j.ghost {
		treat = "GHOST 60 BPM"
	}
	pos := fmt.Sprintf("BAR %d BEAT %d", j.bar()+1, int(j.seq.Beat())%sound.BarBeats+1)
	switch {
	case j.inCoda():
		pos = "CODA"
	case j.codaAsked:
		pos += "  (CODA next bar)"
	}
	pause := ""
	if !j.playing {
		pause = "  PAUSED"
	}
	fmt.Fprintf(&b, "\n  %s  x%.2f  %s%s\n", pos, j.scale, treat, pause)
	if j.path != "" {
		fmt.Fprintf(&b, "  %s: %d error, %d warn", j.path, j.errs, j.warns)
		if j.next != nil {
			b.WriteString("  (new score at the next bar)")
		}
		b.WriteString("\n")
		if j.loadErr != "" {
			fmt.Fprintf(&b, "  RELOAD FAILED, old score kept: %s\n", j.loadErr)
		}
	} else if j.errs+j.warns > 0 {
		fmt.Fprintf(&b, "  %d error, %d warn (see the console)\n", j.errs, j.warns)
	}
	b.WriteString("\n  1-6 voice   A/D bar back/forward   SPACE play/pause\n  C coda   G real/ghost (tempo, line)   Esc quit")
	ebitenutil.DebugPrint(screen, "\n"+b.String())
}

func (j *jukebox) Layout(w, h int) (int, int) { return 480, 270 }

// renderWAV is -render: mix the score headless to a 16-bit mono WAV and return.
func renderWAV(path, voices, midi string, bar, bars int, coda, ghost bool) {
	if voices == "" {
		voices = "all"
	}
	on, err := parseVoices(voices)
	if err != nil {
		log.Fatal(err)
	}
	bank, err := sound.LoadBank(sound.Data)
	if err != nil {
		log.Fatal(err)
	}
	sc, _, _, err := loadScore(midi)
	if err != nil {
		log.Fatal(err)
	}
	var pcm []float32
	if coda {
		pcm = sound.RenderCoda(sc, bank, on, float64(bars*sound.BarBeats), ghost)
	} else {
		if bars == 0 { // "through the coda's end" only means something with -coda
			bars = 8
		}
		pcm = sound.Render(sc, bank, on, float64(max(bar-1, 0)*sound.BarBeats), float64(bars*sound.BarBeats), ghost)
	}
	f, err := os.Create(path)
	if err == nil {
		err = sound.WriteWAV(f, pcm)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%s: %.1f s\n", path, float64(len(pcm))/sound.Rate)
}
