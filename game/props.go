package game

import (
	"image"
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"

	"ghosthotel/assets"
	"ghosthotel/world"
)

// Props in batches (08 W3 browser profile: the y-sorted pass drew ~400 props a frame one call each, 40% of a
// busy 4x4's frame). Every prop picture (its tile, its part of the walls' top band, the covered Mirror's halves)
// is packed on one prop sheet when the floor is painted, and a run of props between two actors goes out as one
// draw, through propSrc: per prop, the real dimension's fog and dread tint, or the ghost dimension's palette.
const (
	propSheetSize = 2048
	propSrc       = `//kage:unit pixels
package main

var Ghost float      // 1 in the ghost dimension: the arcade palette
var Night vec3       // the fog's tone
var FogAlpha float   // the fog's strength at full
var RealDread float  // how far dread tints a prop green in the real dimension

// custom: mode, shade, dread, fog (blit's, per prop)
func Fragment(dst vec4, src vec2, color vec4, custom vec4) vec4 {
	c := imageSrc0At(src)
	if c.a == 0 {
		return vec4(0)
	}
	if Ghost > 0.5 {
		return vec4(arcade(c, custom.x, custom.y, vec3(0), custom.z), 1) * c.a
	}
	rgb := c.rgb / c.a
	if custom.w > 0 {
		rgb = mix(rgb, Night, FogAlpha*custom.w)
	} else {
		rgb *= vec3(1-RealDread*custom.z, 1, 1-RealDread*custom.z)
	}
	return vec4(rgb*c.a, c.a)
}
` + arcadeSrc
)

// shelf packs pictures in rows on a size px page, a pixel apart so none bleeds into the next.
type shelf struct{ x, y, row int }

// place: where a w x h picture goes, false once the page is full.
func (s *shelf) place(w, h, size int) (image.Point, bool) {
	if s.x+w > size {
		s.x, s.y, s.row = 0, s.y+s.row+1, 0
	}
	if w > size || s.y+h > size {
		return image.Point{}, false
	}
	at := image.Pt(s.x, s.y)
	s.x += w + 1
	s.row = max(s.row, h)
	return at, true
}

// propSheet: the props' pictures on one image, and the draw being gathered.
type propSheet struct {
	img   *ebiten.Image
	tile  map[world.TileID]image.Rectangle
	cover [2]image.Rectangle
	band  []image.Rectangle // by index in g.props; empty without one
	vs    []ebiten.Vertex
	is    []uint32
	uni   map[string]any
}

// packProps lays out the prop sheet for g.props and the floor's tiles: when the map is new, before paintFloor.
// The bands come from the CPU (world.Bands) in one write, not a map-sized upload (08 W3: a growth's first frame
// took 110 ms as wasm, most of it here).
func (g *Game) packProps() {
	ps := &g.propSheet
	if ps.img == nil {
		ps.img = ebiten.NewImageWithOptions(image.Rect(0, 0, propSheetSize, propSheetSize), &ebiten.NewImageOptions{Unmanaged: true})
	}
	ps.img.Clear()
	ps.tile, ps.band = map[world.TileID]image.Rectangle{}, make([]image.Rectangle, len(g.props))
	var sh shelf
	put := func(src *ebiten.Image) image.Rectangle {
		b := src.Bounds()
		at, ok := sh.place(b.Dx(), b.Dy(), propSheetSize)
		if !ok {
			panic("props: the prop sheet is full; raise propSheetSize")
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(at.X), float64(at.Y))
		ps.img.DrawImage(src, op)
		return image.Rectangle{at, at.Add(b.Size())}
	}
	m := g.hotel.Map
	tile := func(id world.TileID) {
		if _, ok := ps.tile[id]; !ok && id != 0 {
			ps.tile[id] = put(g.tiles[id])
		}
	}
	for y := range m.H {
		for x := range m.W {
			tile(m.At(world.LayerFloor, x, y))
		}
	}
	var parts []int // the props with a band
	for i, p := range g.props {
		tile(p.id)
		if !world.BandRect(m.TS.Tiles[p.id].Name).Empty() {
			parts = append(parts, i)
		}
	}
	for k, name := range [2]string{"prop_mirror_r0", "prop_mirror_r1"} {
		ps.cover[k] = put(g.coverHalf(name))
	}
	defer func() {
	}()
	if len(parts) == 0 {
		return
	}
	bands, y0, bs := m.BandMask(), sh.y+sh.row+1, shelf{} // the bands' rows go under the tiles
	srcs := make([]image.Rectangle, len(parts))
	for k, i := range parts {
		p := g.props[i]
		srcs[k] = world.BandRect(m.TS.Tiles[p.id].Name).Add(m.Origin(p.cx, p.cy))
		at, ok := bs.place(srcs[k].Dx(), srcs[k].Dy(), propSheetSize)
		if !ok {
			panic("props: the prop sheet is full; raise propSheetSize")
		}
		ps.band[i] = image.Rectangle{at, at.Add(srcs[k].Size())}.Add(image.Pt(0, y0))
	}
	strip := image.NewRGBA(image.Rect(0, 0, propSheetSize, bs.y+bs.row))
	if y0+strip.Rect.Dy() > propSheetSize {
		panic("props: the prop sheet is full; raise propSheetSize")
	}
	ink := [...]color.RGBA{1: world.BandColor, 2: world.BandOutline}
	for k, i := range parts {
		to := ps.band[i].Min.Sub(image.Pt(0, y0))
		for y := range srcs[k].Dy() {
			for x := range srcs[k].Dx() {
				if v := bands.At(srcs[k].Min.X+x, srcs[k].Min.Y+y); v != 0 {
					c := ink[v]
					copy(strip.Pix[strip.PixOffset(to.X+x, to.Y+y):], []uint8{c.R, c.G, c.B, c.A})
				}
			}
		}
	}
	ps.img.SubImage(strip.Rect.Add(image.Pt(0, y0))).(*ebiten.Image).WritePixels(strip.Pix)
}

// paintFloor pre-draws the map's floor from the prop sheet, in one call. H1: the hall's rows stand Slip px east
// of the grid, so their first tile is drawn once more to its west: the street runs to the map's edge.
func (g *Game) paintFloor() *ebiten.Image {
	m, ps := g.hotel.Map, &g.propSheet
	img := reuse(&g.floorBuf, nil, image.Pt(m.W*world.TileSize, m.H*world.TileSize))
	img.Clear()
	vs, is := ps.vs[:0], ps.is[:0] // the props' buffers, empty between runs: no new ones each growth
	for y := range m.H {
		for x := range m.W {
			if id := m.At(world.LayerFloor, x, y); id != 0 {
				o := m.Origin(x, y)
				vs, is = addQuad(vs, is, ps.tile[id], o.X, o.Y, 1, [4]float32{})
			}
		}
	}
	for y := m.SlipRow; m.Slip > 0 && y < m.H; y++ {
		vs, is = addQuad(vs, is, ps.tile[m.At(world.LayerFloor, 0, y)], m.Slip-world.TileSize, y*world.TileSize, 1, [4]float32{})
	}
	img.DrawTriangles32(vs, is, ps.img, nil)
	ps.vs, ps.is = vs[:0], is[:0]
	return img
}

// reuse: an image of size sz, a part of *buf, which is made again (with room to spare) only when sz outgrows it;
// was, if already that size, is kept. The hotel view, the darkness and the floor are map-sized textures: made anew
// at each growth, or each time the darkness went from the screen to the hotel view and back (08 W3).
func reuse(buf **ebiten.Image, was *ebiten.Image, sz image.Point) *ebiten.Image {
	if was != nil && was.Bounds().Size() == sz {
		return was
	}
	if b := *buf; b == nil || b.Bounds().Dx() < sz.X || b.Bounds().Dy() < sz.Y {
		if b != nil {
			b.Deallocate()
		}
		*buf = ebiten.NewImage(sz.X*5/4, sz.Y*5/4)
	}
	return (*buf).SubImage(image.Rectangle{Max: sz}).(*ebiten.Image)
}

// addQuad adds src (on the prop sheet) drawn at (x, y), k times its size, with custom for propSrc.
func addQuad(vs []ebiten.Vertex, is []uint32, src image.Rectangle, x, y, k int, custom [4]float32) ([]ebiten.Vertex, []uint32) {
	n := uint32(len(vs))
	for _, c := range [4][2]int{{0, 0}, {1, 0}, {0, 1}, {1, 1}} {
		vs = append(vs, ebiten.Vertex{
			DstX: float32(x + c[0]*src.Dx()*k), DstY: float32(y + c[1]*src.Dy()*k),
			SrcX: float32(src.Min.X + c[0]*src.Dx()), SrcY: float32(src.Min.Y + c[1]*src.Dy()),
			ColorR: 1, ColorG: 1, ColorB: 1, ColorA: 1,
			Custom0: custom[0], Custom1: custom[1], Custom2: custom[2], Custom3: custom[3],
		})
	}
	return vs, append(is, n, n+1, n+2, n+1, n+3, n+2)
}

// queueProp adds props[i] at screen px (x, y), its cell's corner, to the run being gathered.
func (g *Game) queueProp(i, x, y int) {
	p, ps := g.props[i], &g.propSheet
	name := g.ts.Tiles[p.id].Name
	mode, shade := float32(modeProp), float32(0)
	if maze(name) {
		mode, shade = modeMaze, 1
	}
	dread, fog := g.dreadAt(image.Pt(p.cx, p.cy)), float32(g.propFog(p))
	r := ps.tile[p.id]
	if g.covered(p) { // the lobby Mirror, until it's installed
		r = ps.cover[0]
		if strings.HasPrefix(name, "prop_mirror_r1") {
			r = ps.cover[1]
		}
	}
	custom := [4]float32{mode, shade, dread, fog}
	ps.vs, ps.is = addQuad(ps.vs, ps.is, r, x, y, zoom, custom)
	if b := ps.band[i]; !b.Empty() {
		ps.vs, ps.is = addQuad(ps.vs, ps.is, b, x+p.bx*zoom, y+p.by*zoom, zoom, custom)
	}
}

// flushProps draws the run gathered so far onto dst, in one call.
func (g *Game) flushProps(dst *ebiten.Image) {
	ps := &g.propSheet
	if len(ps.is) == 0 {
		return
	}
	if ps.uni == nil {
		n := assets.Night
		ps.uni = map[string]any{"Night": []float32{float32(n.R) / 255, float32(n.G) / 255, float32(n.B) / 255}, "FogAlpha": float32(fogAlpha), "RealDread": float32(realDread)}
	}
	ps.uni["Ghost"] = float32(0)
	if g.ghost {
		ps.uni["Ghost"] = float32(1)
	}
	dst.DrawTrianglesShader32(ps.vs, ps.is, g.propShader, &ebiten.DrawTrianglesShaderOptions{Images: [4]*ebiten.Image{ps.img}, Uniforms: ps.uni})
	ps.vs, ps.is = ps.vs[:0], ps.is[:0]
}
