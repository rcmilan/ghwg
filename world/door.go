package world

import "image"

const (
	DoorSlideFrames = 15 // frames from shut to fully open (~0.25s at 60 TPS)
	doorReach       = 12 // world px around the leaves within which an allowed actor opens a door
)

// Door is a sliding door: a Room door (1×2 cells) or the Entrance (2×2 cells).
type Door struct {
	Room  int             // index into Hotel.Rooms; -1 for the Entrance
	Rect  image.Rectangle // world px covered by the leaves
	slide int             // 0 shut .. DoorSlideFrames fully open
}

// Actor is someone the doors react to: the Guest of Rooms[Room], or the Concierge (Room -1).
type Actor struct {
	Box  image.Rectangle // foot box, world px
	Room int
}

// Open is how far the leaves have slid: 0 shut, 1 fully open.
func (d Door) Open() float64 { return float64(d.slide) / DoorSlideFrames }

// Solid: a door blocks unless it is fully open.
func (d Door) Solid() bool { return d.slide < DoorSlideFrames }

// Allows applies D21 to an actor (a Guest's own Room index, or -1 for the Concierge): the Entrance lets
// anyone through; a Room door lets its own Guest through always, and the Concierge while the Room is
// Vacant or from inside it.
func (d Door) Allows(actor int, inside, occupied bool) bool {
	switch {
	case d.Room < 0:
		return true
	case actor >= 0:
		return actor == d.Room
	default:
		return !occupied || inside
	}
}

// UpdateDoors slides every door one frame: toward open while an allowed actor is within reach, or while
// anyone stands in the doorway (a door never closes on someone); toward shut otherwise.
func (h *Hotel) UpdateDoors(actors []Actor, occupied func(room int) bool) {
	for i := range h.Doors {
		d := &h.Doors[i]
		want := false
		for _, a := range actors {
			if a.Box.Overlaps(d.Rect) {
				want = true
				break
			}
			if !a.Box.Overlaps(d.Rect.Inset(-doorReach)) {
				continue
			}
			inside := d.Room >= 0 && center(a.Box).In(h.Rooms[d.Room].Shell())
			if d.Allows(a.Room, inside, d.Room >= 0 && occupied(d.Room)) {
				want = true
				break
			}
		}
		if want {
			d.slide = min(d.slide+1, DoorSlideFrames)
		} else {
			d.slide = max(d.slide-1, 0)
		}
	}
}

// DoorBlocks reports whether box r overlaps a door that is not fully open.
func (h *Hotel) DoorBlocks(r image.Rectangle) bool {
	for _, d := range h.Doors {
		if d.Solid() && d.Rect.Overlaps(r) {
			return true
		}
	}
	return false
}

// Shell is the Room's whole 13×9 footprint in world px, doorway included.
func (r Room) Shell() image.Rectangle {
	o := r.Origin.Mul(TileSize)
	return image.Rectangle{o, o.Add(image.Pt(RoomW*TileSize, RoomH*TileSize))}
}

func center(r image.Rectangle) image.Point { return r.Min.Add(r.Max).Div(2) }
