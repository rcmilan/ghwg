package game

import (
	"image"
	"math"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// The Poltergeist tap (09, 2026-11-22): SPACE in ghost on the floor beside a Room's outer walls, with human Guests
// inside, shakes one prop and frightens each of them: manual early Ecto, which the Shadow later automates.
// It earns through frighten, the one Fear rule (shadow.go). Placeholders.
const (
	tapFear        = shadowSeen // Fear each human inside takes per tap: one Shadow sighting
	tapShakeFrames = 24         // frames the tapped prop shakes
	tapShakePx     = 1          // world px it jitters either way
)

// poltState: the prop the last tap shook.
type poltState struct {
	cell image.Point
	at   int // g.tick of the tap; 0 before the first
}

// beside: the Room whose outer walls world px at touches from outside, on any side (the door's included), or -1.
// Corridors are 2 cells wide, so no tile is beside two Rooms.
func beside(h *world.Hotel, at image.Point) int {
	for i, r := range h.Rooms {
		if s := r.Shell(); at.In(s.Inset(-world.TileSize)) && !at.In(s) {
			return i
		}
	}
	return -1
}

// scareable: the Guest could take Fear from a tap (not fearless or a Macguffin's, and below its FleeAt).
func scareable(p *Party, guest *entities.Guest) bool {
	return !isMacguffin(p) && !fearless(guest) && guest.Fear < guest.FleeAt
}

// tapTarget: the Room the Concierge could tap from here, and the prop it would shake (the one nearest his feet,
// a cell; (-1,-1) in a bare Room), or -1.
func (g *Game) tapTarget() (int, image.Point) {
	if !g.ghost {
		return -1, image.Point{}
	}
	room := beside(g.hotel, g.feet())
	if room < 0 {
		return -1, image.Point{}
	}
	p, in := g.inside(room)
	ok := false
	for _, guest := range in {
		ok = ok || scareable(p, guest)
	}
	if !ok {
		return -1, image.Point{}
	}
	const ts = world.TileSize
	m, feet := g.hotel.Map, g.feet()
	best, cell := math.MaxFloat64, image.Pt(-1, -1)
	s := g.hotel.Rooms[room].Shell()
	for y := s.Min.Y / ts; y < s.Max.Y/ts; y++ {
		for x := s.Min.X / ts; x < s.Max.X/ts; x++ {
			if id := m.At(world.LayerProps, x, y); id != 0 && !m.TS.Tiles[id].Door {
				if d := math.Hypot(float64(feet.X-x*ts-ts/2), float64(feet.Y-y*ts-ts/2)); d < best {
					best, cell = d, image.Pt(x, y)
				}
			}
		}
	}
	return room, cell
}

// poltTapNow: one tap on the room from where the Concierge stands: the prop shakes and each human inside takes
// tapFear. The cap is the Guest's FleeAt, as a ghost's haunt (match.go terrify): enough taps and the Party steps out,
// then flees, unpaid. False if refused.
func (g *Game) poltTapNow() bool {
	room, cell := g.tapTarget()
	if room < 0 {
		return false
	}
	p, in := g.inside(room)
	for _, guest := range in {
		g.frighten(p, guest, tapFear, guest.FleeAt)
	}
	g.poltTap = poltState{cell, g.tick}
	return true
}

// tapShake: the world px offset of the prop at cell now, 0 unless it is the one just tapped.
func (g *Game) tapShake(cell image.Point, layer int) int {
	t := g.tick - g.poltTap.at
	if g.poltTap.at == 0 || layer != world.LayerProps || cell != g.poltTap.cell || t >= tapShakeFrames {
		return 0
	}
	return tapShakePx * (1 - (t/3)%3) // -> +1,0,-1 on a 3-frame beat
}
