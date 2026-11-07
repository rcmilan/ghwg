package game

import (
	"image"

	"ghosthotel/world"
)

// Strategy decides what the Concierge does next, from what a player sees (D105), through the Pilot
// (D103). The runner calls Act when the Pilot falls idle, then once per game minute while it stays idle.
// A Strategy that also has Cheat(*Game) gets the whole Game before each Act, and its rows are flagged.
type Strategy interface {
	Act(v View, p *Pilot)
}

// Strategies are the ones the CLI and the watch mode know by name (D106).
var Strategies = map[string]Strategy{
	"never":   stand{atTerminal}, // never haunts: stays real at the Terminal
	"haunter": haunter{},
}

// stand keeps the Concierge at a spot, closing the Terminal if it is open.
type stand struct {
	at func(*world.Hotel) image.Point
}

func atTerminal(h *world.Hotel) image.Point { return h.ConciergeStart }

// haunter haunts every Occupied Room it sees unhaunted, the least frightened first, and waits at the
// Mirror in between.
type haunter struct{}

func (haunter) Act(v View, p *Pilot) {
	if v.Terminal.Open {
		p.Close()
		return
	}
	best := -1
	for i, r := range v.Rooms {
		if !r.Vacant && !r.Haunted && (best < 0 || r.Fear < v.Rooms[best].Fear) {
			best = i
		}
	}
	switch {
	case best >= 0:
		p.Haunt(best)
	case v.Concierge.Div(world.TileSize) != v.Hotel.Mirror.Div(world.TileSize):
		p.GoTo(v.Hotel.Mirror)
	}
}

func (s stand) Act(v View, p *Pilot) {
	switch {
	case v.Terminal.Open:
		p.Close()
	case v.Concierge.Div(world.TileSize) != s.at(v.Hotel).Div(world.TileSize):
		p.GoTo(s.at(v.Hotel))
	}
}
