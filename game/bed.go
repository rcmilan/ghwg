package game

import (
	"image"
	"strings"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// Beds and sleep (D167). Each Stay Spot's bed is a lane up from the Spot to the pillow. A bed is entered from
// its side, never from its foot (K1, 2026-11-11).
const sleepDepth = 12 // world px from a bed's bottom edge up to its sleeper's feet: the head on the pillow

func asleep(c int) bool { return hour(c) < wakeHour || hour(c) >= sleepHour }

// sleepSpot: where a sleeper's feet rest in the bed.
func sleepSpot(bed image.Rectangle) image.Point {
	return image.Pt((bed.Min.X+bed.Max.X)/2, bed.Max.Y-sleepDepth)
}

// bedAt: the bed that world point (x,y) lies in.
func (g *Game) bedAt(x, y float64) (image.Rectangle, bool) {
	at := image.Pt(int(x), int(y))
	for _, r := range g.hotel.Rooms {
		for _, b := range r.Lanes {
			if at.In(b) {
				return b, true
			}
		}
	}
	return image.Rectangle{}, false
}

// bedSide: the free floor cell beside the bed's bottom row, nearest its lane, where a sleeper steps in and
// out: its center, level with the sleep spot. A Double's wall-side sleeper crosses the other half; Twin
// beds may share the column between them. The bed's foot if it has no free side (no shipped Room Type).
func (g *Game) bedSide(b image.Rectangle) image.Point {
	const ts = world.TileSize
	m, x, y := g.hotel.Map, b.Min.X/ts, b.Max.Y/ts-1
	bed := func(x int) bool {
		return strings.HasPrefix(m.TS.Tiles[m.At(world.LayerProps, x, y)].Name, "prop_bed_")
	}
	best := 0
	for _, d := range [...]int{-1, 1} {
		n := x + d
		for bed(n) {
			n += d
		}
		if !m.Solid(n, y) && (best == 0 || abs(n-x) < abs(best-x)) {
			best = n
		}
	}
	if best == 0 {
		return image.Pt((b.Min.X+b.Max.X)/2, b.Max.Y+ts/2)
	}
	return image.Pt(best*ts+ts/2, sleepSpot(b).Y)
}

// inLane: foot box r lies in a bed's bottom row, between its side cell and its sleep spot, where the bed lets
// the Concierge lie in it. Its foot stays shut.
func (g *Game) inLane(r image.Rectangle) bool {
	const ts = world.TileSize
	for _, room := range g.hotel.Rooms {
		for _, b := range room.Lanes {
			side := g.bedSide(b).X
			if r.In(image.Rect(min(b.Min.X, side-ts/2), b.Max.Y-ts, max(b.Max.X, side+ts/2), b.Max.Y)) {
				return true
			}
		}
	}
	return false
}

// route: a guest's path to `to` (Hotel.Path in its own Room, around avoid). From a bed it first steps off
// its side, never off its foot.
func (g *Game) route(guest *entities.Guest, to image.Point, avoid ...image.Point) []image.Point {
	from, side := image.Pt(int(guest.X), int(guest.Y)), []image.Point(nil)
	if b, ok := g.bedAt(guest.X, guest.Y); ok {
		from = g.bedSide(b)
		side = []image.Point{from}
	}
	path := g.hotel.Path(from, to, guest.RoomIndex, avoid...)
	if path == nil {
		return nil
	}
	return append(side, path...)
}

// toBed: a path into the Guest's own bed (lane i of its Room): round to the bed's side and in sideways, or
// along it when already lying in it.
func (g *Game) toBed(guest *entities.Guest, r world.Room, i int) []image.Point {
	i = bedOf(g.realmOf(guest), r, i)
	if b, ok := g.bedAt(guest.X, guest.Y); ok && b == r.Lanes[i] {
		return []image.Point{sleepSpot(b)}
	}
	return append(g.route(guest, g.bedSide(r.Lanes[i])), sleepSpot(r.Lanes[i]))
}

// bedOf: the bed of the Room's i-th Guest of a realm: a ghost takes the last bed first, so in a twin Room a
// lone ghost sleeps beside a lone human, not on it (the layers never collide, but the picture reads).
func bedOf(realm Realm, r world.Room, i int) int {
	if realm == Ghost {
		return len(r.Lanes) - 1 - i
	}
	return i
}

// bedtime: at sleepHour each Guest of the staying Party walks into its bed, facing down; at wakeHour it
// gets up to its Stay Spot, and its day of activities (D168) runs until bed again.
func (g *Game) bedtime(p *Party) {
	r := g.hotel.Rooms[p.Room]
	if night := asleep(g.clock); night != p.abed {
		p.abed = night
		for i, guest := range p.Guests {
			if night {
				guest.SetPath(g.toBed(guest, r, i))
			} else {
				g.idle(p, i)
			}
		}
	}
	if !p.abed {
		g.pastime(p)
		return
	}
	for _, guest := range p.Guests {
		if len(guest.Path) > 0 {
			g.walk(guest, arriveSpeed)
		} else {
			guest.Anim.Facing = entities.FaceDown
		}
	}
}
