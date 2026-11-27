package game

import (
	"log"
	"math"
	"time"

	"ghosthotel/sound"
	"ghosthotel/world"
)

// 09 Sound (context/09-sound/): the game's side of the audio. The sound package mixes; the game decides each
// frame which steps sound (steps.go, T403 STEPS) and the two sliders (the pause menu, T407 PAUSE). A headless Game (sim, tests, -shot) never opens
// sound: snd.mix stays nil and every hook returns at once.
type soundState struct {
	mix    *sound.Mixer
	bank   *sound.Bank       // nil until loader is done
	loader *sound.BankLoader // the samples still decoding (loadSound)
	vol    [2]int            // the sliders by sound.Bus, in steps 0..volSteps; session only (decisions.md "Options")
	tone   bool              // dev -tone: the seam's test tone, a beep each second
	frame  int               // real frames since the sound opened
	steps  stepState         // steps.go (T403)
}

const (
	volSteps   = 10
	volDefault = 8 // 80%
)

// OpenSound starts the audio: main calls it for a person's game only (never the sim, tests or -shot). The
// samples load over the next frames (loadSound): the steps wait for them.
func (g *Game) OpenSound() error {
	s := &g.snd
	s.mix = sound.NewMixer()
	l, err := sound.NewBankLoader(sound.Data)
	if err != nil {
		return err
	}
	s.loader = l
	return sound.Open(s.mix)
}

// loadBudget: the most of a frame the samples' decoding takes (the first file is decoded whatever it costs).
const loadBudget = 6 * time.Millisecond

// loadSound decodes samples until the frame's budget is spent.
func (g *Game) loadSound() {
	s := &g.snd
	for start := time.Now(); s.loader != nil && (time.Since(start) < loadBudget); {
		b, err := s.loader.Step()
		switch {
		case err != nil:
			log.Printf("sound: %v", err)
			s.loader = nil
		case b != nil:
			s.loader, s.bank = nil, b
		}
	}
}

// DevTone is main's -tone: a beep each second, to prove the audio path (the web build included).
func (g *Game) DevTone() { g.snd.tone = true }

// soundInit: the sliders' defaults, set on every Game (headless too, so the pause menu reads them).
func (s *soundState) init() { s.vol = [2]int{volDefault, volDefault} }

// updateSound runs once per Update, after everything else.
func (g *Game) updateSound() {
	s := &g.snd
	if s.mix == nil {
		return
	}
	g.loadSound()
	if s.tone && s.frame%60 == 0 {
		s.mix.Play(sound.Sine(69, .3), sound.SFX, 1, .8)
	}
	s.frame++
	paused := g.state == StatePause // silence while paused (decisions.md "Options"); the sliders apply on leaving
	for b := range s.vol {
		v := float32(s.vol[b]) / volSteps
		if paused {
			v = 0
		}
		s.mix.SetBus(sound.Bus(b), v)
	}
	g.updateSteps()
}

// ding: the bell a Priest's ritual ends on (live test 3).
var ding = sound.Bell(81, 2.5)

// ritualDing: the ritual's single bell as its effect lands, fainter with distance, unheard past hearRange (no wall
// check: a bell carries).
func (g *Game) ritualDing(pr *priest) {
	d := math.Hypot(pr.X-g.player.X, pr.Y-g.player.Y) / (hearRange * world.TileSize)
	if g.snd.mix != nil && d < 1 {
		g.snd.mix.Play(ding, sound.SFX, 1, float32(.5*(1-d)))
	}
}

// clockBeat: the music position the clock gives, in beats since Day 1 00:00 (fractional within a frame). The
// music follows it strictly (decisions.md "Time"): MIX takes it modulo the loop, STEPS snaps to its 16ths.
func (g *Game) clockBeat() float64 { return (float64(g.clock) + g.acc) / sound.BeatFrames }
