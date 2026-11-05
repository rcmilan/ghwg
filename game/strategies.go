package game

import (
	"image"

	"shift/world"
)

// Strategy decides what the Concierge does next, from what a player sees (D105), through the Pilot
// (D103). The runner calls Act when the Pilot falls idle, then once per game minute while it stays idle.
// A Strategy that also has Cheat(*Game) gets the whole Game before each Act, and its rows are flagged.
type Strategy interface {
	Act(v View, p *Pilot)
}

// Strategies are the ones the CLI and the watch mode know by name (D106).
var Strategies = map[string]Strategy{
	"bestrate": stand{atTerminal, bestRate},
	"idle":     stand{atTerminal, nil},
}

// stand keeps the Concierge at a spot; with a chooser, it books the front Party into the Room chosen
// (or closes the Terminal if none, -1), then walks back.
type stand struct {
	at     func(*world.Hotel) image.Point
	choose func(View) int
}

func atTerminal(h *world.Hotel) image.Point { return h.ConciergeStart }

func (s stand) Act(v View, p *Pilot) {
	switch {
	case v.Terminal.Open && (s.choose == nil || v.Front == nil):
		p.Close()
	case v.Terminal.Open:
		if room := s.choose(v); room >= 0 {
			p.Book(room)
		} else {
			p.Close()
		}
	case s.choose != nil && v.Front != nil:
		p.OpenTerminal()
	case v.Concierge.Div(world.TileSize) != s.at(v.Hotel).Div(world.TileSize):
		p.GoTo(s.at(v.Hotel))
	}
}

// firstFit: the first Vacant Room the front Party fits in.
func firstFit(v View) int {
	for i, r := range v.Rooms {
		if r.Vacant && r.Capacity >= v.Front.Size {
			return i
		}
	}
	return -1
}

// bestRate: the Vacant Room that pays the front Party the most per night. A pair pays half outside the
// bed layout it wished for (D81), which the Terminal's card shows.
func bestRate(v View) int {
	best, room := 0, -1
	for i, r := range v.Rooms {
		rate := r.Rate
		if v.Front.Size == 2 && r.Beds != v.Front.Wish {
			rate /= 2
		}
		if r.Vacant && r.Capacity >= v.Front.Size && rate > best {
			best, room = rate, i
		}
	}
	return room
}
