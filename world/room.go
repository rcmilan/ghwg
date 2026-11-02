package world

import "image"

// LayoutConfig is the single source of truth for the hotel plan.
type LayoutConfig struct{ GridRows, GridCols, OriginX, OriginY, RoomWidth, RoomHeight, RoomGap, HallWallY, PassageWidth int }
type Room struct {
	Name   string
	Bounds image.Rectangle
	Door   image.Rectangle
	Locked bool
}

func (r Room) Contains(x, y float64) bool {
	return x >= float64(r.Bounds.Min.X) && x < float64(r.Bounds.Max.X) && y >= float64(r.Bounds.Min.Y) && y < float64(r.Bounds.Max.Y)
}

type Layout struct {
	Config                                 LayoutConfig
	Rooms                                  []Room
	Walls                                  []image.Rectangle
	Passage, Counter, Exit, InteriorBounds image.Rectangle
}

func DefaultLayout() Layout {
	// The 44px grid gap is a deliberately walkable central corridor. The lower
	// rooms also have a 34px corridor before the hall wall.
	return NewLayout(LayoutConfig{GridRows: 2, GridCols: 2, OriginX: 45, OriginY: 50, RoomWidth: 420, RoomHeight: 120, RoomGap: 44, HallWallY: 380, PassageWidth: 100})
}
func NewLayout(c LayoutConfig) Layout {
	if c.GridRows < 1 {
		c.GridRows = 1
	}
	if c.GridCols < 1 {
		c.GridCols = 1
	}
	rooms := make([]Room, 0, c.GridRows*c.GridCols)
	for row := 0; row < c.GridRows; row++ {
		for col := 0; col < c.GridCols; col++ {
			x, y := c.OriginX+col*(c.RoomWidth+c.RoomGap), c.OriginY+row*(c.RoomHeight+c.RoomGap)
			bounds := image.Rect(x, y, x+c.RoomWidth, y+c.RoomHeight)
			doorX := x + c.RoomWidth/2 - 35
			rooms = append(rooms, Room{Name: "ROOM " + string(rune('A'+len(rooms))), Bounds: bounds, Door: image.Rect(doorX, bounds.Max.Y-8, doorX+70, bounds.Max.Y+12)})
		}
	}
	minX, maxX := c.OriginX-10, c.OriginX+c.GridCols*c.RoomWidth+(c.GridCols-1)*c.RoomGap+10
	passageX := (minX + maxX - c.PassageWidth) / 2
	passage := image.Rect(passageX, c.HallWallY, passageX+c.PassageWidth, c.HallWallY+28)
	return Layout{Config: c, Rooms: rooms, Walls: []image.Rectangle{image.Rect(minX, c.HallWallY, passage.Min.X, c.HallWallY+28), image.Rect(passage.Max.X, c.HallWallY, maxX, c.HallWallY+28)}, Passage: passage, Counter: image.Rect(100, 380, 350, 445), Exit: image.Rect(440, 482, 520, 510), InteriorBounds: image.Rect(35, 45, 925, 500)}
}
func (l Layout) PassageCenter() (float64, float64) {
	return float64(l.Passage.Min.X + l.Passage.Dx()/2), float64(l.Passage.Min.Y + l.Passage.Dy()/2)
}
func (l Layout) ExitCenter() (float64, float64) {
	return float64(l.Exit.Min.X + l.Exit.Dx()/2), float64(l.Exit.Min.Y + l.Exit.Dy()/2)
}
func (l Layout) CounterCenter() (float64, float64) {
	return float64(l.Counter.Min.X + l.Counter.Dx()/2), float64(l.Counter.Min.Y + l.Counter.Dy()/2)
}
func (l Layout) IsBlocked(x, y float64, rooms []Room) bool {
	for _, wall := range l.Walls {
		if x >= float64(wall.Min.X) && x < float64(wall.Max.X) && y >= float64(wall.Min.Y) && y < float64(wall.Max.Y) {
			return true
		}
	}
	for _, room := range rooms {
		// Rooms are physical solids for everyone. Guests wait immediately
		// outside their assigned doorway; no actor can cut through a room.
		if room.Contains(x, y) {
			return true
		}
	}
	return false
}
