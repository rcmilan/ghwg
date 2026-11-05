package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// input is every key the game reads (D102): the keyboard when a person plays, a script when the Pilot does.
type input interface {
	Pressed(k ebiten.Key) bool
	JustPressed(k ebiten.Key) bool
}

type keyboard struct{}

func (keyboard) Pressed(k ebiten.Key) bool     { return ebiten.IsKeyPressed(k) }
func (keyboard) JustPressed(k ebiten.Key) bool { return inpututil.IsKeyJustPressed(k) }

// script is a keyboard played by code: hold sets the keys down for the next frame, and a key is just
// pressed when it is down now and was up the frame before.
type script struct{ now, last map[ebiten.Key]bool }

func (s *script) hold(keys ...ebiten.Key) {
	s.last, s.now = s.now, s.last
	if s.now == nil {
		s.now = map[ebiten.Key]bool{}
	}
	clear(s.now)
	for _, k := range keys {
		s.now[k] = true
	}
}

func (s *script) Pressed(k ebiten.Key) bool     { return s.now[k] }
func (s *script) JustPressed(k ebiten.Key) bool { return s.now[k] && !s.last[k] }

// move is the WASD direction, each axis -1, 0 or 1.
func move(in input) (dx, dy float64) {
	if in.Pressed(ebiten.KeyA) {
		dx--
	}
	if in.Pressed(ebiten.KeyD) {
		dx++
	}
	if in.Pressed(ebiten.KeyW) {
		dy--
	}
	if in.Pressed(ebiten.KeyS) {
		dy++
	}
	return dx, dy
}
