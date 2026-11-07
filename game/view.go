package game

import (
	"image"

	"ghosthotel/world"
)

// View is what a player sees, nothing more (D105): a Strategy decides from it and the Pilot steers by it.
type View struct {
	Title     bool // the title screen is up
	Ghost     bool // the hotel is in the ghost dimension
	Clock     string
	Day       int
	Money     int
	Spook     int
	Rooms     []RoomView    // in the Terminal's order
	Guests    []image.Point // every guest's cell not hidden by the Fog (none is, in the ghost dimension)
	Concierge image.Point   // world px
	Terminal  TerminalView
	Hotel     *world.Hotel // the map as drawn: walls, doors, anchors. Read it, never write it.
}

type RoomView struct {
	Number, Type, Beds string
	Vacant             bool
	Capacity, Rate     int
	// Seen only in the ghost dimension: its props shake, and its Guests glow with Fear.
	Haunted bool
	Fear    int // its most frightened Guest in sight
}

type TerminalView struct {
	Open    bool
	Message string
}

func (g *Game) view() View {
	v := View{
		Title: g.state == StateTitle, Ghost: g.ghost,
		Clock: clockText(g.clock), Day: day(g.clock), Money: g.money, Spook: g.spook,
		Concierge: image.Pt(int(g.player.X), int(g.player.Y)), Hotel: g.hotel,
		Terminal: TerminalView{g.state == StateTerminal, g.message},
	}
	for i, r := range g.hotel.Rooms {
		rv := RoomView{Number: r.Number, Type: r.Type.Name, Beds: r.Type.Beds, Vacant: g.vacant(i), Capacity: len(r.Stays), Rate: r.Type.Rate}
		if g.ghost {
			rv.Haunted = g.haunting(i)
			for _, guest := range g.guests() {
				if guest.RoomIndex == i {
					rv.Fear = max(rv.Fear, guest.Fear)
				}
			}
		}
		v.Rooms = append(v.Rooms, rv)
	}
	for _, guest := range g.guests() {
		if at := image.Pt(int(guest.X), int(guest.Y)); g.fogAt(at) < 1 {
			v.Guests = append(v.Guests, at.Div(world.TileSize))
		}
	}
	return v
}
