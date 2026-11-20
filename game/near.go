package game

import (
	"image"
	"iter"

	"ghosthotel/entities"
)

// walkersNear is walkers (same ones, same order) cut to those standing in box (world px): the scans that
// only care about someone a few cells off (bumpsAhead, yields, steer) skip the rest before the loop body
// runs, and skip a whole Party of the other realm at once.
// ponytail: still a scan of every walker, but a cheap one; a spatial grid if 10x10+ ever matters.
func (g *Game) walkersNear(realm Realm, box image.Rectangle) iter.Seq[*entities.Guest] {
	in := func(o *entities.Guest) bool {
		return int(o.X) >= box.Min.X && int(o.X) <= box.Max.X && int(o.Y) >= box.Min.Y && int(o.Y) <= box.Max.Y
	}
	return func(yield func(*entities.Guest) bool) {
		for _, p := range g.parties {
			if p.Realm != realm || p.state == out {
				continue
			}
			for _, guest := range p.Guests {
				if in(guest) && !((p.state == goingOut || p.state == leaving) && len(guest.Path) == 0) && !yield(guest) {
					return
				}
			}
		}
		if realm == Human {
			for _, c := range g.couriers {
				if in(&c.Guest) && !yield(&c.Guest) {
					return
				}
			}
			for _, k := range g.keepers {
				if in(&k.Guest) && !yield(&k.Guest) {
					return
				}
			}
		}
	}
}

// around: the box of everything within r world px of any of the points.
func around(r int, pts ...image.Point) image.Rectangle {
	lo, hi := pts[0], pts[0]
	for _, p := range pts[1:] {
		lo, hi = image.Pt(min(lo.X, p.X), min(lo.Y, p.Y)), image.Pt(max(hi.X, p.X), max(hi.Y, p.Y))
	}
	return image.Rectangle{Min: lo.Sub(image.Pt(r, r)), Max: hi.Add(image.Pt(r, r))}
}
