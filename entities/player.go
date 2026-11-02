package entities

import (
	"image"
	"math"

	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// Actor positions are the center of their foot box, in world px.
const (
	FootW, FootH = 10, 6
	PlayerSpeed  = 1.0 // world px per frame (3 screen px at ×3)
)

// FootBox is the collision box of an actor standing at (x,y).
func FootBox(x, y float64) image.Rectangle {
	x0, y0 := int(math.Floor(x-FootW/2)), int(math.Floor(y-FootH/2))
	return image.Rect(x0, y0, x0+FootW, y0+FootH)
}

type Player struct{ X, Y float64 }

func NewPlayer(start image.Point) Player { return Player{X: float64(start.X), Y: float64(start.Y)} }

// Move reads WASD and moves one axis at a time, so walls stop one axis and the other slides.
func (p *Player) Move(m *world.Tilemap) {
	var dx, dy float64
	if ebiten.IsKeyPressed(ebiten.KeyA) {
		dx--
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) {
		dx++
	}
	if ebiten.IsKeyPressed(ebiten.KeyW) {
		dy--
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) {
		dy++
	}
	speed := PlayerSpeed
	if dx != 0 && dy != 0 {
		speed *= math.Sqrt2 / 2
	}
	if nx := p.X + dx*speed; dx != 0 && !m.BlockedBox(FootBox(nx, p.Y)) {
		p.X = nx
	}
	if ny := p.Y + dy*speed; dy != 0 && !m.BlockedBox(FootBox(p.X, ny)) {
		p.Y = ny
	}
}

func (p Player) Near(x, y, distance float64) bool {
	dx, dy := p.X-x, p.Y-y
	return dx*dx+dy*dy <= distance*distance
}
