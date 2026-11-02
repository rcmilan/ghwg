package entities

import (
	"ghosthotel/world"
	"github.com/hajimehoshi/ebiten/v2"
)

type Player struct{ X, Y float64 }

const PlayerRadius = 14.0

func NewPlayer() Player { return Player{X: 180, Y: 430} }

// Move permits free lobby movement but refuses a locked room's doorway/interior.
func (p *Player) Move(layout world.Layout, rooms []world.Room) {
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
	if dx == 0 && dy == 0 {
		return
	}
	speed := 3.0
	if dx != 0 && dy != 0 {
		speed = 2.12
	}
	nx, ny := p.X+dx*speed, p.Y+dy*speed
	if layout.IsBlocked(nx, ny, rooms) {
		return
	}
	if nx > float64(layout.InteriorBounds.Min.X) && nx < float64(layout.InteriorBounds.Max.X) && ny > float64(layout.InteriorBounds.Min.Y) && ny < float64(layout.InteriorBounds.Max.Y) {
		p.X, p.Y = nx, ny
	}
}

func (p Player) Near(x, y, distance float64) bool {
	dx, dy := p.X-x, p.Y-y
	return dx*dx+dy*dy <= distance*distance
}
