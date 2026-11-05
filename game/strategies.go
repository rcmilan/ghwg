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
	"autoall":  autoAll{},
}

// autoAll sets every Room auto at its Door Panel, then stands at the Terminal booking nothing: the
// zero-manual baseline (04 D122).
type autoAll struct{}

func (autoAll) Act(v View, p *Pilot) {
	for i, r := range v.Rooms {
		if !r.Auto {
			p.SetAuto(i, true)
			return
		}
	}
	stand{atTerminal, nil}.Act(v, p)
}

// stand keeps the Concierge at a spot; with a chooser, it books the Room and the Party at the counter
// chosen (or closes the Terminal if none, -1), then walks back.
type stand struct {
	at     func(*world.Hotel) image.Point
	choose func(View) (room, party int)
}

func atTerminal(h *world.Hotel) image.Point { return h.ConciergeStart }

func (s stand) Act(v View, p *Pilot) {
	switch {
	case v.Terminal.Open && (s.choose == nil || len(v.Counter) == 0):
		p.Close()
	case v.Terminal.Open:
		if room, party := s.choose(v); room >= 0 {
			p.Book(room, party)
		} else {
			p.Close()
		}
	case s.choose != nil && len(v.Counter) > 0:
		p.OpenTerminal()
	case v.Concierge.Div(world.TileSize) != s.at(v.Hotel).Div(world.TileSize):
		p.GoTo(s.at(v.Hotel))
	}
}

// firstFit: the first Vacant Room the first Party at the counter fits in; the next Party if none.
func firstFit(v View) (room, party int) {
	for k, f := range v.Counter {
		for i, r := range v.Rooms {
			if r.Vacant && r.Capacity >= f.Size {
				return i, k
			}
		}
	}
	return -1, -1
}

// bestRate: the Vacant Room and the Party at the counter it pays the most per night; ties to the Party
// that came first. A pair pays half outside the bed layout it wished for (D81), which the Terminal shows.
func bestRate(v View) (room, party int) {
	best, room, party := 0, -1, -1
	for k, f := range v.Counter {
		for i, r := range v.Rooms {
			rate := r.Rate
			if f.Size == 2 && r.Beds != f.Wish {
				rate /= 2
			}
			if r.Vacant && r.Capacity >= f.Size && rate > best {
				best, room, party = rate, i, k
			}
		}
	}
	return room, party
}
