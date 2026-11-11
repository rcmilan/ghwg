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

// Actor is someone the doors react to: the Guest of Rooms[Room], or the staff (Room -1): the Concierge, a
// courier, a Priest.
type Actor struct {
	Box  image.Rectangle // foot box, world px
	Room int
}

// Open is how far the leaves have slid: 0 shut, 1 fully open.
func (d Door) Open() float64 { return float64(d.slide) / DoorSlideFrames }

// Solid: a door blocks unless it is fully open.
func (d Door) Solid() bool { return d.slide < DoorSlideFrames }

// Allows: the Entrance lets anyone through; a Room door opens to the staff (actor -1) and to its own Guest
// (K1, 2026-11-11: D21 and D91 shut an Occupied Room to the Concierge; that rule is gone).
func (d Door) Allows(actor int) bool { return d.Room < 0 || actor < 0 || actor == d.Room }

// UpdateDoors slides every door one frame: toward open while an allowed actor is within reach, or while
// anyone stands in the doorway (a door never closes on someone); toward shut otherwise. A boarded Room's
// door stays shut to everyone.
func (h *Hotel) UpdateDoors(actors []Actor, boarded func(room int) bool) {
	for i := range h.Doors {
		d := &h.Doors[i]
		want := false
		for _, a := range actors {
			if d.Room >= 0 && boarded(d.Room) {
				break
			}
			if a.Box.Overlaps(d.Rect) || a.Box.Overlaps(d.Rect.Inset(-doorReach)) && d.Allows(a.Room) {
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

// Shell is the Room's whole RoomW×RoomH footprint in world px, doorway included.
func (r Room) Shell() image.Rectangle {
	o := r.Origin.Mul(TileSize)
	return image.Rectangle{o, o.Add(image.Pt(RoomW*TileSize, RoomH*TileSize))}
}

// Spot: the 2 Corridor cells in front of the door, where the Concierge stands to act on the Room.
func (r Room) Spot() image.Rectangle {
	c := r.Door.Add(image.Pt(0, 2)).Mul(TileSize)
	return image.Rectangle{c, c.Add(image.Pt(TileSize, 2*TileSize))}
}
