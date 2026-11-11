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
	"never":  stand{atTerminal}, // stays real at the Terminal
	"server": server{},
	"smart":  smart{},
}

// stand keeps the Concierge at a spot, closing the Terminal if it is open.
type stand struct {
	at func(*world.Hotel) image.Point
}

func atTerminal(h *world.Hotel) image.Point { return h.ConciergeStart }

func (s stand) Act(v View, p *Pilot) {
	switch {
	case v.Terminal.Open:
		p.Close()
	case v.Concierge.Div(world.TileSize) != s.at(v.Hotel).Div(world.TileSize):
		p.GoTo(s.at(v.Hotel))
	}
}

// server stays real, keeps the supplies stocked and serves every Room that asks (T198); it waits at the
// Terminal in between.
type server struct{}

func (server) Act(v View, p *Pilot) {
	if openRooms(v, p) || restock(v, p) {
		return
	}
	if i := toServe(v); i >= 0 && !v.Terminal.Open {
		serveRoom(v, p, i)
		return
	}
	stand{atTerminal}.Act(v, p)
}

// toServe: the first asking Room whose item is in the hands or in stock, or -1.
func toServe(v View) int {
	for i, r := range v.Rooms {
		if r.Asking && (v.Hands == r.Wants || v.Stock[r.Wants] > 0) {
			return i
		}
	}
	return -1
}

// serveRoom fetches the Room's item, unless it's already in the hands, and delivers it.
func serveRoom(v View, p *Pilot, room int) {
	if v.Hands != v.Rooms[room].Wants {
		p.Fetch(v.Rooms[room].Wants)
	}
	p.Serve(room)
}

// smart keeps the supplies stocked, serves whenever a Room asks, cleans empty Rooms in ghost (Ecto from the
// dirt), calls a Weak Priest to a Closed Room, and waits at the Mirror in between.
type smart struct{}

func (smart) Act(v View, p *Pilot) {
	if v.Terminal.Open {
		p.Close()
		return
	}
	if openRooms(v, p) || restock(v, p) {
		return
	}
	for i, r := range v.Rooms {
		if r.Closed && !r.Priest && v.Money >= priestWeak {
			p.CallPriest(i, false)
			return
		}
	}
	if i := toServe(v); i >= 0 {
		serveRoom(v, p, i)
		return
	}
	for i, r := range v.Rooms { // empty and dirty: clean it before its Party walks away, in ghost for the Ecto (dirt.go)
		if (r.Vacant || r.Away) && len(r.Dirt) > 0 {
			p.Flip(true)
			p.Clean(i, r.Dirt[0])
			return
		}
	}
	if v.Concierge.Div(world.TileSize) != v.Hotel.Mirror.Div(world.TileSize) {
		p.GoTo(v.Hotel.Mirror)
	}
}
