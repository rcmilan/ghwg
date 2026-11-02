package game

import "math"

// The view shows viewW×viewH world px, drawn ×zoom straight onto the 960×540 screen. Positions round to
// screen px (1/zoom world px), so sub-pixel motion like diagonals scrolls smoothly.
const (
	viewW, viewH = 320, 180
	zoom         = 3
	camLerp      = 0.15
)

// Camera is the view's top-left corner in world px.
type Camera struct{ X, Y float64 }

// Follow eases the view toward centering (tx,ty), never showing past the map edges.
func (c *Camera) Follow(tx, ty float64, mapW, mapH int) {
	x, y := clampView(tx, ty, mapW, mapH)
	c.X += (x - c.X) * camLerp
	c.Y += (y - c.Y) * camLerp
}

// Snap centers the view on (tx,ty) at once (shift start).
func (c *Camera) Snap(tx, ty float64, mapW, mapH int) { c.X, c.Y = clampView(tx, ty, mapW, mapH) }

// Origin is the view's top-left corner in screen px for a player at (px,py). It rounds the camera's lag
// behind the player rather than the camera itself, so the player's pixel and the view's pixel step on the
// same frames: the player holds still on screen while the map scrolls (no diagonal flicker).
func (c Camera) Origin(px, py float64, mapW, mapH int) (int, int) {
	ox := screenPx(px) - screenPx(px-c.X)
	oy := screenPx(py) - screenPx(py-c.Y)
	return min(max(ox, 0), (mapW-viewW)*zoom), min(max(oy, 0), (mapH-viewH)*zoom)
}

// screenPx converts world px to the nearest screen px.
func screenPx(w float64) int { return int(math.Round(w * zoom)) }

func clampView(tx, ty float64, mapW, mapH int) (float64, float64) {
	return min(max(tx-viewW/2, 0), float64(mapW-viewW)), min(max(ty-viewH/2, 0), float64(mapH-viewH))
}
