package game

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"

	"ghosthotel/world"
)

// Father K's ritual look, all of it, apart from Father M's (ritual_draw.go): nothing here is shared with drawM, so
// reworking K cannot move M. The knobs first; world px unless it says frames. Timing the rules own (the chant's
// 30 game minutes, the burst's burstFrames at which the effect lands, his walk and pace speeds) lives in priest.go.
const (
	kHop        = .8           // how high his bow lifts him, world px
	kHopPeriod  = 3 * chantHop // frames of one bow
	kFadeIn     = 60           // frames the pool, ring and motes take to come up at the chant's start
	kRingR      = 14           // the circle's radius on the floor
	kRingW      = 1.5          // its line, world px at zoom 3
	kRingA      = .75          // its brightness
	kPulse      = 45           // frames per radian of the ring's breathing (sin(t/kPulse))
	kPulseLow   = .6           // the ring's dimmest breath, of kRingA (it breathes up to 1)
	kPoolSize   = 1.6          // the pool of light under him, in circle radii across
	kPoolSwell  = .06          // how much the pool swells with the breath
	kPoolA      = .3           // the pool's brightness
	kMotes      = 3            // sparkles rising, no more
	kMoteLife   = 180          // frames a mote takes to rise and fade
	kMoteFrom   = 4            // where a mote starts, above his feet
	kMoteRise   = 22           // how far it rises on top of that
	kMoteSway   = 7            // its sideways drift
	kMoteSize   = 7            // its glow's width
	kMoteA      = .8           // its brightness
	kBurstGrow  = .6           // the floor ring's growth over the burst, of the circle's radius
	kBurstRing  = .5           // its brightness at the start, fading out
	kBurstSize  = 3            // the pulse of light's width, in tiles, at its fullest
	kBurstLift  = 6            // where it is centred, above his feet
	kBurstA     = .55          // its peak brightness
	kLightCut   = .35          // the night light of his chant (ritualLights)
	kLightBurst = .35          // and what the burst adds to it
)

// kTint: a pale moon-blue.
var kTint = [3]float32{.75, .88, 1}

// kLift: how high Father K hovers in his chant: he barely stirs, a slow bow.
func kLift(pr *priest) float64 {
	return kHop * math.Abs(math.Sin(math.Pi*pr.t/kHopPeriod))
}

func (g *Game) drawK(s *ebiten.Image, pr *priest, x, y float64, floor bool) {
	const r = kRingR * zoom
	switch {
	case pr.phase == rChant && floor:
		f := math.Min(1, pr.t/kFadeIn)
		breath := .5 + .5*math.Sin(pr.t/kPulse)
		g.glowAt(s, x, y, kPoolSize*r*(1+kPoolSwell*breath), kTint, kPoolA*f)             // a faint pool under him
		drawRing(s, x, y, r, 1, kRingW, kTint, kRingA*f*(kPulseLow+(1-kPulseLow)*breath)) // the ring, breathing
	case pr.phase == rChant:
		f := math.Min(1, pr.t/kFadeIn)
		for k := range kMotes {
			u := math.Mod(pr.t/kMoteLife+float64(k)/kMotes, 1)
			g.glowAt(s, x+math.Sin(float64(k)*2.4+u*2)*kMoteSway*zoom, y-(kMoteFrom+kMoteRise*u)*zoom, kMoteSize*zoom, kTint, kMoteA*math.Sin(math.Pi*u)*f)
		}
	case pr.phase == rBurst && floor:
		f := pr.t / burstFrames
		drawRing(s, x, y, r*(1+kBurstGrow*f), 1, kRingW, kTint, kBurstRing*(1-f))
	case pr.phase == rBurst:
		f := pr.t / burstFrames
		g.glowAt(s, x, y-kBurstLift*zoom, kBurstSize*world.TileSize*zoom*(.6+.4*f), kTint, kBurstA*math.Sin(math.Pi*f)) // one soft pulse
	}
}
