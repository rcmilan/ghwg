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

// Strategies are the ones the CLI and the watch mode know by name (D106). With booking gone (D139) the
// Concierge has no decision yet: S3 brings the first one, haunting.
var Strategies = map[string]Strategy{
	"idle": stand{atTerminal},
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
