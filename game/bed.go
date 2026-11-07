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

// bedtime: at sleepHour each Guest of the staying Party walks from its Stay Spot up its bed's lane; at
// wakeHour back down to the Spot. In bed it faces down.
func (g *Game) bedtime(p *Party) {
	r := g.hotel.Rooms[p.Room]
	if night := asleep(g.clock); night != p.abed {
		p.abed = night
		for i, guest := range p.Guests {
			path := []image.Point{r.Stays[i]}
			if night {
				path = append(g.hotel.Path(image.Pt(int(guest.X), int(guest.Y)), r.Stays[i], p.Room), sleepSpot(r.Lanes[i]))
			}
			guest.SetPath(path)
		}
	}
	for _, guest := range p.Guests {
		if len(guest.Path) > 0 {
			g.walk(guest, arriveSpeed)
		} else if p.abed {
			guest.Anim.Facing = entities.FaceDown
		}
	}
}
