package game

import (
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// The ghost dimension's arcade palette (D144): the same sprites, recolored by brightness in a shader.
// Maze tiles (floor, walls, side lines) go black with deep-blue outlines; props band into pink, cyan,
// orange and yellow; actors each keep one flat color (ghostTints).
const paletteSrc = `//kage:unit pixels
package main

var Mode float // 0 prop, 1 maze, 2 actor
var Shade float // maze: how bright its blue outlines are
var Tint vec3 // actor: its color
var Dread float // prop, maze: its Room's shift toward green and purple (D162), 0..1

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	c := imageSrc0At(src)
	if c.a == 0 {
		return vec4(0)
	}
	l := dot(c.rgb/c.a, vec3(0.299, 0.587, 0.114))
	purple, green := vec3(0.62, 0.22, 1), vec3(0.2, 1, 0.35)
	blue := mix(vec3(33, 33, 222)/255, purple, Dread)
	rgb := blue
	if Mode > 1.5 {
		rgb = mix(Tint*0.25, Tint, smoothstep(0.2, 0.55, l))
		if l > 0.9 {
			rgb = vec3(1)
		}
	} else if Mode > 0.5 {
		rgb = blue * Shade * step(l, 0.33)
	} else if l >= 0.88 {
		rgb = vec3(1, 1, 0)
	} else if l >= 0.7 {
		rgb = vec3(1, 0.72, 0.32)
	} else if l >= 0.5 {
		rgb = vec3(0, 1, 1)
	} else if l >= 0.3 {
		rgb = vec3(1, 0.72, 1)
	}
	if Mode < 0.5 && l >= 0.3 { // the bright bands go green, the cool ones purple
		band := purple
		if l >= 0.7 {
			band = green
		}
		rgb = mix(rgb, band, Dread)
	}
	return vec4(rgb, 1) * c.a * color.a
}
`

const (
	modeProp  = 0
	modeMaze  = 1
	modeActor = 2
	floorBlue = 0.3  // the floor's seams: a dim blue grid, so the walls' outlines read first
	realDread = 0.15 // how far a Closed Room tints green in the real dimension
)

// ghostTints: each character's arcade color. The Concierge is the yellow one.
var ghostTints = map[string][]float32{
	"Bob":      {1, 1, 0},
	"Adam":     {1, 0, 0},
	"Amelia":   {1, .72, 1},
	"Alex":     {0, 1, 1},
	"Father K": {1, .72, .32},
	"Father M": {1, .72, .32},
}

// maze: the tile draws as the maze (floor, walls, side lines) in the ghost dimension.
func maze(name string) bool {
	for _, p := range [...]string{"floor_", "wall_", "border_", "void"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// blit draws src onto dst like DrawImage; in the ghost dimension through the palette, as mode (with
// shade for a maze tile, tint for an actor). dread (0..1) is the Room's shift (D162): green and purple in
// ghost, a faint green in real.
func (g *Game) blit(dst, src *ebiten.Image, op *ebiten.DrawImageOptions, mode int, shade float32, tint []float32, dread float32) {
	if !g.ghost {
		o := *op
		o.ColorScale.Scale(1-realDread*dread, 1, 1-realDread*dread, 1)
		dst.DrawImage(src, &o)
		return
	}
	if tint == nil {
		tint = []float32{0, 0, 0}
	}
	b := src.Bounds()
	sop := &ebiten.DrawRectShaderOptions{GeoM: op.GeoM, ColorScale: op.ColorScale, Images: [4]*ebiten.Image{src},
		Uniforms: map[string]any{"Mode": float32(mode), "Shade": shade, "Tint": tint, "Dread": dread}}
	dst.DrawRectShader(b.Dx(), b.Dy(), g.palette, sop)
}
