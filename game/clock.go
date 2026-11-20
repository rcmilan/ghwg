package game

import "fmt"

// The Clock (D71): game time in frames since Day 1, 00:00. A game minute is 10 frames, so a game hour
// is ~10 s and a day ~4 min.
const (
	minuteFrames = 10
	hourFrames   = 60 * minuteFrames
	dayFrames    = 24 * hourFrames
	runStart     = 8 * hourFrames // a Run starts on Day 1 at 08:00 (D72)
	viewSlow     = 0.25           // the world's time scale while the hotel view is open (08 F0): slow motion, no pause
	fastScale    = 4              // holding T runs the world ×4: a dev aid (T014, D1; it was ×10 until a live test froze the computer, cause not found)
)

func day(c int) int  { return c/dayFrames + 1 }
func hour(c int) int { return c % dayFrames / hourFrames }

// runText: the run timer, real time played this Run (T381 refine 7): "RUN 1:02:03".
func runText(frames int) string {
	s := frames / tps
	return fmt.Sprintf("RUN %d:%02d:%02d", s/3600, s/60%60, s%60)
}

// clockText is the Terminal's readout (D74): "DAY 3  14:20".
func clockText(c int) string {
	return fmt.Sprintf("DAY %d  %02d:%02d", day(c), hour(c), c%hourFrames/minuteFrames)
}

// advanceClock runs the Clock one frame; the Terminal pauses it by not stepping the world at all (D73). Each
// new game minute scares the haunted (haunt.go), lets Guests ask for service (serve.go) and may bring a Walk-in.
func (g *Game) advanceClock() {
	if g.clock++; g.clock%minuteFrames != 0 {
		return
	}
	g.scare()
	g.updateAsks()
	g.updateRoomTalk()
	g.hauntGhosts()
	g.maybeWalkIn()
	g.maybeGhostIn()
}

// The world time scale (D1): the world runs g.scale frames of world time a real frame (fastScale while T is held).
// Frames stay the unit of world time: speeds, fades and animations advance by g.dt, a step's share of a frame;
// the Clock and g.tick count whole frames, each ending in some step (g.ticked), where the per-frame logic runs.
// A step over one frame is cut into steps of at most one, so nobody walks through a wall or past a waypoint
// and no minute's events are skipped.

// timeScale: the world's frames this real frame.
func (g *Game) timeScale() float64 {
	if g.fast {
		return fastScale
	}
	return g.scale
}

// now: world time in frames, smooth between ticks; what draws a fade or a rise reads it.
func (g *Game) now() float64 { return float64(g.tick) + g.acc }

// DevScale is main's -scale: the world runs at s (0.25 a quarter speed) for a live test.
func (g *Game) DevScale(s float64) { g.scale = s }
