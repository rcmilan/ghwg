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
	"never":        stand{atTerminal}, // never haunts: stays real at the Terminal
	"haunter":      haunter{},
	"haunt-priest": hauntPriest{},
	"server":       server{},
	"smart":        smart{},
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

// hauntPriest haunts a Room only while one more haunt leaves its Guests under the Fear limit, so its
// Parties stay and pay, and calls a Weak Priest to a Closed Room as soon as it has the money.
type hauntPriest struct{}

func (hauntPriest) Act(v View, p *Pilot) {
	if v.Terminal.Open {
		p.Close()
		return
	}
	for i, r := range v.Rooms {
		if r.Closed && !r.Priest && v.Money >= priestWeak {
			p.CallPriest(i, false)
			return
		}
	}
	for i, r := range v.Rooms {
		if !r.Vacant && !r.Haunted && r.Fear+hauntMinutes*fearPerMinute < fearLimit {
			p.Haunt(i)
			return
		}
	}
	if v.Concierge.Div(world.TileSize) != v.Hotel.Mirror.Div(world.TileSize) {
		p.GoTo(v.Hotel.Mirror)
	}
}

// server stays real and serves every Room that asks; it waits at the Terminal in between.
type server struct{}

func (server) Act(v View, p *Pilot) {
	if i := toServe(v); i >= 0 && !v.Terminal.Open {
		p.Serve(i)
		return
	}
	stand{atTerminal}.Act(v, p)
}

// toServe: the first Room a serve would land on, or -1.
func toServe(v View) int {
	for i, r := range v.Rooms {
		if r.Asking {
			return i
		}
	}
	return -1
}

// smart serves whenever a Room asks, haunts each Party once (so none flees), and calls a Weak Priest to a
// Closed Room: Spook for the least money. Fear shows only in ghost, so it shifts to look before it haunts.
type smart struct{}

func (smart) Act(v View, p *Pilot) {
	if v.Terminal.Open {
		p.Close()
		return
	}
	for i, r := range v.Rooms {
		if r.Closed && !r.Priest && v.Money >= priestWeak {
			p.CallPriest(i, false)
			return
		}
	}
	if i := toServe(v); i >= 0 {
		p.Serve(i)
		return
	}
	for i, r := range v.Rooms {
		if r.Vacant || r.Away || r.Haunted {
			continue
		}
		switch {
		case !v.Ghost:
			p.Flip(true)
			return
		case r.Fear+hauntMinutes*fearPerMinute < fearLimit:
			p.Haunt(i)
			return
		}
	}
	if v.Concierge.Div(world.TileSize) != v.Hotel.Mirror.Div(world.TileSize) {
		p.GoTo(v.Hotel.Mirror)
	}
}
