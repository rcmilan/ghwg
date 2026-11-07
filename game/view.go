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
}

type TerminalView struct {
	Open    bool
	Message string
}

func (g *Game) view() View {
	v := View{
		Title: g.state == StateTitle, Ghost: g.ghost,
		Clock: clockText(g.clock), Day: day(g.clock), Money: g.money,
		Concierge: image.Pt(int(g.player.X), int(g.player.Y)), Hotel: g.hotel,
		Terminal: TerminalView{g.state == StateTerminal, g.message},
	}
	for i, r := range g.hotel.Rooms {
		v.Rooms = append(v.Rooms, RoomView{r.Number, r.Type.Name, r.Type.Beds, g.vacant(i), len(r.Stays), r.Type.Rate})
	}
	for _, guest := range g.guests() {
		if at := image.Pt(int(guest.X), int(guest.Y)); g.fogAt(at) < 1 {
			v.Guests = append(v.Guests, at.Div(world.TileSize))
		}
	}
	return v
}
