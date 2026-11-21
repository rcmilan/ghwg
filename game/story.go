package game

import "slices"

// StoryMode is main's -story (web: ?story), for testing the questline fast; never on by default (decisions.md,
// "Story mode and credits"): each Growth fires free as its beat ends, all 7 beats are open at any Level, and quest
// guests check in at once.
var StoryMode bool

// storyGrowMax: the hotel's limit in story mode, a side in Rooms: seven Growths from the 2x1 start fit.
const storyGrowMax = 7

// storyGrow runs each game minute: the first Growth, and each one after a beat is done, fires free with its own
// cinematic, alternating a column and a floor.
func (g *Game) storyGrow() {
	q := g.quest
	if !StoryMode || g.state != StatePlay || g.growing != nil || g.growNext != 0 || q.Beat >= beatCount || slices.Contains(q.Done, false) {
		return
	}
	g.unlock(Mirror) // ghost quest guests (beat 2 on) check in only through it
	c, r := g.hotel.Grid()
	if c > r && r < maxGrid(g.level) || c >= maxGrid(g.level) {
		g.growNext = growFloor
	} else {
		g.growNext = growCol
	}
	g.beginGrow()
}

// storyOpen: a quest guest finds no Room free: the first Room still without a key opens for it (a Growth's new ones).
func (g *Game) storyOpen() {
	if !StoryMode {
		return
	}
	if i := slices.Index(g.roomOwned, false); i >= 0 {
		g.openRoom(i)
	}
}
