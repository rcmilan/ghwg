package game

import (
	"image"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
)

// The ghost dimension's arcade palette (D144): the same sprites, recolored by brightness in a shader.
// Maze tiles (floor, walls, side lines) go black with deep-blue outlines; props band into pink, cyan,
// orange and yellow; actors each keep one flat color (ghostTints).
const paletteSrc = `//kage:unit pixels
package main

var Mode float // 0 prop, 1 maze, 2 actor, 3 flat ghost
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
	if Mode > 2.5 { // a ghost guest: one flat color, its shading kept as a darker tone, no white
		rgb = mix(Tint*0.2, Tint, smoothstep(0.22, 0.42, l))
	} else if Mode > 1.5 {
		rgb = mix(Tint*0.18, Tint, smoothstep(0.22, 0.5, l))
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
	if Mode < 0.5 || Mode > 1.5 { // the colors a little more saturated than the sheets' (F0 Ghost palette)
		rgb = clamp(mix(vec3(dot(rgb, vec3(0.299, 0.587, 0.114))), rgb, 1.3), 0, 1)
	}
	return vec4(rgb, 1) * c.a * color.a
}
`

const (
	modeProp  = 0
	modeMaze  = 1
	modeActor = 2
	modeFlat  = 3    // a ghost guest (ghostguest.go)
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
// ghost, a faint green in real. src must be kept across frames (a tile, a sprite, a sheet): in ghost its look is
// baked once and then drawn as a plain sprite, so the GPU batches the hotel's sprites like the real dimension's
// (08 W3 live test: one shader draw per sprite held the web build at 30 FPS in ghost). An image made per frame or
// as big as the floor goes through shade.
func (g *Game) blit(dst, src *ebiten.Image, op *ebiten.DrawImageOptions, mode int, shade float32, tint []float32, dread float32) {
	g.blitPart(dst, src, src.Bounds(), op, mode, shade, tint, dread)
}

// blitPart is blit of r, a part of src (in src's coordinates): a frame of a sheet, a door's sliding leaf.
func (g *Game) blitPart(dst, src *ebiten.Image, r image.Rectangle, op *ebiten.DrawImageOptions, mode int, shade float32, tint []float32, dread float32) {
	if !g.ghost {
		part := src
		if r != src.Bounds() {
			part = src.SubImage(r).(*ebiten.Image)
		}
		g.shade(dst, part, op, mode, shade, tint, dread)
		return
	}
	k := bakeKey{src: src, mode: mode, shade: shade, dread: steps(dread)}
	for i := range min(3, len(tint)) {
		k.tint[i] = steps(tint[i])
	}
	img, ok := g.baked[k]
	if !ok && g.bakes >= bakeBudget { // this frame's bakes are spent: through the shader this once
		part := src
		if r != src.Bounds() {
			part = src.SubImage(r).(*ebiten.Image)
		}
		g.shade(dst, part, op, mode, shade, tint, dread)
		return
	}
	if !ok {
		img = g.bake(k)
	}
	o := &ebiten.DrawImageOptions{GeoM: op.GeoM}
	o.ColorScale.ScaleAlpha(op.ColorScale.A()) // the palette keeps only the alpha of the draw's color
	if r != src.Bounds() {
		img = img.SubImage(r.Sub(src.Bounds().Min)).(*ebiten.Image)
	}
	dst.DrawImage(img, o)
}

// bakeKey: one look of one image. Dread and tint count in 32nds, so a Room's pulse or a ghost's flare
// bakes a handful of looks, not one a frame.
type bakeKey struct {
	src   *ebiten.Image
	mode  int
	shade float32
	tint  [3]float32
	dread float32
}

func steps(v float32) float32 { return float32(math.Round(float64(v)*32)) / 32 }

// bakeBudget: looks baked per drawn frame, so the first ghost frame (or the first after the floor is repainted)
// never stalls on hundreds of them (08 W3 live test: lag on the first shift); the rest go through the shader
// until baked, a few frames later.
const bakeBudget = 32

// bakeCap: ponytail: past this many looks the cache starts over, a backstop against an image made per frame
// reaching blit; a leak shows as rebakes, never as memory.
const bakeCap = 8192

// bake paints k's look of its image once.
func (g *Game) bake(k bakeKey) *ebiten.Image {
	if g.baked == nil || len(g.baked) >= bakeCap {
		g.baked = map[bakeKey]*ebiten.Image{}
	}
	b := k.src.Bounds()
	img := ebiten.NewImage(b.Dx(), b.Dy())
	img.DrawRectShader(b.Dx(), b.Dy(), g.palette, &ebiten.DrawRectShaderOptions{Images: [4]*ebiten.Image{k.src},
		Uniforms: map[string]any{"Mode": float32(k.mode), "Shade": k.shade, "Tint": k.tint[:], "Dread": k.dread}})
	g.baked[k] = img
	g.bakes++
	return img
}

// shade is blit through the shader every draw: for the floor and the images made per frame, each one big draw.
func (g *Game) shade(dst, src *ebiten.Image, op *ebiten.DrawImageOptions, mode int, shade float32, tint []float32, dread float32) {
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

// Levitation (F0): a ghost, and the Concierge in ghost, is drawn hoverUp px off the floor, drifting
// ±hoverDrift every hoverPeriod frames. Drawing only.
const (
	hoverUp     = 3
	hoverDrift  = 1
	hoverPeriod = 2 * tps
)

// hover: how high name floats at tick, world px; each name drifts on its own beat.
func hover(tick int, name string) float64 {
	beat := 0
	for _, c := range name {
		beat += int(c) * 7
	}
	return hoverUp + hoverDrift*math.Sin(2*math.Pi*float64(tick+beat)/hoverPeriod)
}
