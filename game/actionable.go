package game

import "image"

// spaceAct: what SPACE would do right now at the Concierge's spot (08 F0).
type spaceAct int

const (
	actNone spaceAct = iota
	actTerminal
	actMirror
	actRoomMirror
	actSupply
	actDeliver
	actTalk
	actClean
	actView
)

// actionable: what SPACE would do now and the world rect of the prop it acts on (for the glow). F0-C fills it from
// the SPACE dispatch's own checks; the glow and the Terminal's "!" badge draw from it.
func (g *Game) actionable() (spaceAct, image.Rectangle) { return actNone, image.Rectangle{} }
