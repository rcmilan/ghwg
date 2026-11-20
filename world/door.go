package world

import (
	"cmp"
	"image"
	"slices"
)

const (
	DoorSlideFrames = 15 // frames from shut to fully open (~0.25s at 60 TPS)
	doorReach       = 12 // world px around the leaves within which an allowed actor opens a door
)

// Door is a sliding door: a Room door (1×2 cells) or the Entrance (2×2 cells).
type Door struct {
	Room  int             // index into Hotel.Rooms; -1 for the Entrance
	Rect  image.Rectangle // world px covered by the leaves
	slide float64         // 0 shut .. DoorSlideFrames fully open
}

// Actor is someone the doors react to: the Guest of Rooms[Room], or the staff (Room -1): the Concierge, a
// courier, a Priest.
type Actor struct {
	Box  image.Rectangle // foot box, world px
	Room int
}

// Open is how far the leaves have slid: 0 shut, 1 fully open.
func (d Door) Open() float64 { return d.slide / DoorSlideFrames }

// Solid: a door blocks unless it is fully open.
func (d Door) Solid() bool { return d.slide < DoorSlideFrames }

// Allows: the Entrance lets anyone through; a Room door opens to the staff (actor -1) and to its own Guest
// (K1, 2026-11-11: D21 and D91 shut an Occupied Room to the Concierge; that rule is gone).
func (d Door) Allows(actor int) bool { return d.Room < 0 || actor < 0 || actor == d.Room }

// UpdateDoors slides every door dt frames: toward open while an allowed actor is within reach, or while
// anyone stands in the doorway (a door never closes on someone); toward shut otherwise. A boarded Room's
// door stays shut to everyone.
func (h *Hotel) UpdateDoors(actors []Actor, boarded func(room int) bool, dt float64) {
	// Doors sorted by top edge: an actor looks only at the doors on its own rows, not at all of them.
	b := &h.pf
	idx, maxH := b.doors[:0], 0
	for i, d := range h.Doors {
		idx = append(idx, i)
		maxH = max(maxH, d.Rect.Dy())
	}
	slices.SortFunc(idx, func(i, j int) int { return cmp.Or(h.Doors[i].Rect.Min.Y-h.Doors[j].Rect.Min.Y, i-j) })
	b.doors = idx
	want := append(b.want[:0], make([]bool, len(idx))...)
	b.want = want
	for _, a := range actors {
		lo, _ := slices.BinarySearchFunc(idx, a.Box.Min.Y-doorReach-maxH, func(i, y int) int { return cmp.Compare(h.Doors[i].Rect.Min.Y, y+1) })
		for _, i := range idx[lo:] {
			d := &h.Doors[i]
			if d.Rect.Min.Y >= a.Box.Max.Y+doorReach {
				break
			}
			if !want[i] && (a.Box.Overlaps(d.Rect) || a.Box.Overlaps(d.Rect.Inset(-doorReach)) && d.Allows(a.Room)) {
				want[i] = true
			}
		}
	}
	for i := range h.Doors {
		d := &h.Doors[i]
		if want[i] && !(d.Room >= 0 && boarded(d.Room)) {
			d.slide = min(d.slide+dt, DoorSlideFrames)
		} else {
			d.slide = max(d.slide-dt, 0)
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

// Mat: the doormat, the 3×2 Corridor cells below the door: SPACE on it delivers to the Room (08 F0).
func (r Room) Mat() image.Rectangle {
	c := r.Door.Add(image.Pt(-1, 2)).Mul(TileSize)
	return image.Rectangle{c, c.Add(image.Pt(3*TileSize, 2*TileSize))}
}

// Plate: the mat's cell below the guest-card plate, one tile west of the door, where SPACE will show the guest
// card (G2); until then it does nothing there.
func (r Room) Plate() image.Rectangle {
	c := r.Door.Add(image.Pt(-1, 2)).Mul(TileSize)
	return image.Rectangle{c, c.Add(image.Pt(TileSize, TileSize))}
}
