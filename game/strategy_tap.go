package game

import (
	"image"
	"slices"
)

func init() { Strategies["tapper"] = tapper{} }

// tapSafe: the most Fear a Room's Guest may have for the tapper to tap it again; one more tap stays under fearLimit.
const tapSafe = fearLimit - tapFear - 1

// tapper plays goals, and where the Mirror is open and nothing waits to be served it goes ghost and taps Rooms
// with human Guests inside (Poltergeist tap) until their Fear nears fleeing. 09: the early Ecto the tap adds.
type tapper struct{}

func (tapper) Act(v View, p *Pilot) {
	if !v.Terminal.Open && !v.MirrorLocked && toServe(v) < 0 {
		for i, r := range v.Rooms {
			if !r.Boarded && !r.Vacant && !r.Away && r.Fear <= tapSafe && inside(v, i) {
				p.Flip(true)
				p.Tap(i, (tapSafe-r.Fear)/tapFear+1)
				return
			}
		}
	}
	goals{}.Act(v, p)
}

// inside: some guest is in the Room's shell (a ghost's too: the View does not tell them apart in ghost).
func inside(v View, room int) bool {
	s := v.Hotel.Rooms[room].Shell()
	return slices.ContainsFunc(v.Guests, func(c image.Point) bool { return v.Hotel.Map.Center(c.X, c.Y).In(s) })
}
