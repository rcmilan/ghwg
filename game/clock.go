package game

import "fmt"

// The Clock (D71): game time in frames since Day 1, 00:00. A game minute is 10 frames, so a game hour
// is ~10 s and a day ~4 min.
const (
	minuteFrames = 10
	hourFrames   = 60 * minuteFrames
	dayFrames    = 24 * hourFrames
	runStart     = 8 * hourFrames // a Run starts on Day 1 at 08:00 (D72)
	fastClock    = 10             // holding T runs the clock ×10: a dev aid for multi-day stays (T014)
)

func day(c int) int  { return c/dayFrames + 1 }
func hour(c int) int { return c % dayFrames / hourFrames }

// clockText is the Terminal's readout (D74): "DAY 3  14:20".
func clockText(c int) string {
	return fmt.Sprintf("DAY %d  %02d:%02d", day(c), hour(c), c%hourFrames/minuteFrames)
}

// advanceClock runs the Clock one frame (fastClock frames while T is held); the Terminal pauses it by not
// stepping the world at all (D73). Each new game minute scares the haunted (haunt.go) and may bring a Walk-in.
func (g *Game) advanceClock() {
	n := 1
	if g.fast {
		n = fastClock
	}
	for ; n > 0; n-- {
		g.clock++
		if g.clock%minuteFrames != 0 {
			continue
		}
		g.scare()
		g.maybeWalkIn()
	}
}
