package game

import (
	"image"

	"ghosthotel/entities"
	"ghosthotel/world"
)

// Beds and sleep (D167). Each Stay Spot's bed is a lane up from the Spot to the pillow.
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

// inLane: foot box r lies in a bed's lane, from its Stay Spot's cell up to the sleep spot, where the bed
// lets the Concierge in (cosmetic only; D91 keeps him out of Occupied Rooms).
func (g *Game) inLane(r image.Rectangle) bool {
	for _, room := range g.hotel.Rooms {
		for _, b := range room.Lanes {
			if r.In(image.Rect(b.Min.X, sleepSpot(b).Y-entities.FootH/2, b.Max.X, b.Max.Y+world.TileSize)) {
				return true
			}
		}
	}
	return false
}

// route: a guest's path to `to` (Hotel.Path in its own Room, around avoid). From a bed it first walks
// down to the bed's foot, never off its side.
func (g *Game) route(guest *entities.Guest, to image.Point, avoid ...image.Point) []image.Point {
	from, foot := image.Pt(int(guest.X), int(guest.Y)), []image.Point(nil)
	if b, ok := g.bedAt(guest.X, guest.Y); ok {
		from = image.Pt((b.Min.X+b.Max.X)/2, b.Max.Y+world.TileSize/2)
		foot = []image.Point{from}
	}
	path := g.hotel.Path(from, to, guest.RoomIndex, avoid...)
	if path == nil {
		return nil
	}
	return append(foot, path...)
}

// toBed: a path into the Guest's own bed (lane i of its Room): up from the bed's foot, or along it when
// already lying in it.
func (g *Game) toBed(guest *entities.Guest, r world.Room, i int) []image.Point {
	if b, ok := g.bedAt(guest.X, guest.Y); ok && b == r.Lanes[i] {
		return []image.Point{sleepSpot(b)}
	}
	return append(g.route(guest, r.Stays[i]), sleepSpot(r.Lanes[i]))
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
