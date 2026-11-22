// Command questlooks makes the quest characters' sheets (08 W5 R2): each is a LimeZu base sheet with its
// colors swapped (hair, skin, clothes) and one accessory stamped on every frame, following the head's top
// row or the hand, so it bobs and swings with the body. It writes
// assets/characters/<Name>_{idle_anim,run}_16x16.png, one pair per look in looks.go, and the Cat's sheet
// (cat.go, assets/sprites/cat.png), and never overwrites an existing file unless run with -force, so hand-polished
// PNGs survive. The base sheets are only read.
//
//	go run ./tools/questlooks [-force] [-only Magenta,Tim,Cat]
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Frames are 16x32, 6 per facing, in entities.Facing's order: right, up, left, down.
const frameW, perFacing = 16, 6

// A frame's rows, counted from its head's top row, fall in three zones: a color may be hair on the head,
// a jacket on the body and shoes on the legs (Alex's, Adam's).
const (
	bodyFrom = 13
	legsFrom = 18
)

// swap: a base color → the look's, per zone. A zone without the color asks the one above it (legs, body,
// head); a color in none stays as drawn (the outlines, the eyes).
type swap struct{ head, body, legs map[uint32]uint32 }

func (s swap) to(c uint32, row int) uint32 {
	zones := []map[uint32]uint32{s.head}
	if row >= bodyFrom {
		zones = append(zones, s.body)
	}
	if row >= legsFrom {
		zones = append(zones, s.legs)
	}
	for _, z := range slices.Backward(zones) {
		if n, ok := z[c]; ok {
			return n
		}
	}
	return c
}

// erase: in a drawing, the sheet's pixel cleared (the hair a hat pulled low would leave beside its cone).
const erase = 'x'

// accessory: one drawing per facing, '.' leaving the sheet's pixel.
// On the head (hand false) each drawing is 16 wide and its last `over` rows sit on the head, from its top
// row down. In the hand, the drawing's top-left lands at the hand's pixel plus at[facing].
type accessory struct {
	hand bool
	over int
	at   [4]image.Point
	art  [4][]string
}

// skirt: the legs closed into one cloth, from the hips down to `hem` rows above the feet's last row (0: to
// the floor), in a light and a shade. It is drawn across each row's legs, so it swings with the stride.
type skirt struct {
	cloth [2]uint32
	hem   int
}

type look struct {
	name, base  string
	swap        swap
	skirt       *skirt
	accessories []accessory
}

func rgb(c uint32) color.NRGBA { return color.NRGBA{uint8(c >> 16), uint8(c >> 8), uint8(c), 255} }

func hex(c color.NRGBA) uint32 { return uint32(c.R)<<16 | uint32(c.G)<<8 | uint32(c.B) }

// dress: a copy of sheet as l: its colors swapped, then its accessories stamped on every frame. It never
// moves the body: an accessory that would leave its frame is an error, and so is a frame with no head.
func dress(sheet image.Image, l look) (*image.NRGBA, error) {
	b := sheet.Bounds()
	src := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(src, src.Bounds(), sheet, b.Min, draw.Src)
	out := image.NewNRGBA(src.Bounds())
	for f := range b.Dx() / frameW {
		top := headTop(src, f)
		if top < 0 {
			return nil, fmt.Errorf("frame %d is empty", f)
		}
		for y := range b.Dy() {
			for x := f * frameW; x < (f+1)*frameW; x++ {
				p := src.NRGBAAt(x, y)
				if p.A > 0 {
					n := rgb(l.swap.to(hex(p), y-top))
					n.A = p.A
					p = n
				}
				out.SetNRGBA(x, y, p)
			}
		}
	}
	for f := range b.Dx() / frameW {
		if l.skirt != nil {
			l.skirt.on(src, out, f, legs[l.base])
		}
	}
	for _, a := range l.accessories {
		for f := range b.Dx() / frameW {
			face := f / perFacing % 4
			art := a.art[face]
			x0, y0 := 0, headTop(src, f)+a.over-len(art)
			if a.hand {
				h, ok := heldAt(src, f, skins[l.base])
				if !ok {
					return nil, fmt.Errorf("frame %d: no hand in its facing to hold the accessory", f)
				}
				// held low in a stride, it is lifted to stay in the frame
				x0, y0 = min(max(0, h.X+a.at[face].X), frameW-len(art[0])), min(h.Y+a.at[face].Y, b.Dy()-len(art))
			}
			for j, row := range art {
				for i, r := range row {
					c, ok := inks[r]
					if !ok && r != erase {
						continue
					}
					if x, y := x0+i, y0+j; x < 0 || x >= frameW || y < 0 || y >= b.Dy() {
						return nil, fmt.Errorf("frame %d: the accessory leaves the frame at (%d,%d)", f, x, y)
					}
					if r == erase {
						out.SetNRGBA(f*frameW+x0+i, y0+j, color.NRGBA{})
						continue
					}
					out.SetNRGBA(f*frameW+x0+i, y0+j, rgb(c))
				}
			}
		}
	}
	return out, nil
}

// skirtRows: how many rows up from the feet's last row a skirt may start.
const skirtRows = 4

// on draws the skirt on frame f of out: on each of the legs' rows (src's pixels of the legs' colors), cloth
// from the first leg pixel to the last, the shade on its right; then an outline wherever cloth is left with
// nothing beside or under it.
func (s skirt) on(src, out *image.NRGBA, f int, legs []uint32) {
	h := src.Bounds().Dy()
	foot := h - 1
	for ; foot >= 0 && rowEmpty(src, f, foot); foot-- {
	}
	cloth := map[image.Point]bool{}
	for y := foot - skirtRows + 1; y <= foot-s.hem; y++ {
		lo, hi := frameW, -1
		for x := range frameW {
			if p := src.NRGBAAt(f*frameW+x, y); p.A > 0 && slices.Contains(legs, hex(p)) {
				lo, hi = min(lo, x), max(hi, x)
			}
		}
		for x := lo; x <= hi; x++ {
			c := s.cloth[0]
			if x == hi {
				c = s.cloth[1]
			}
			out.SetNRGBA(f*frameW+x, y, rgb(c))
			cloth[image.Pt(x, y)] = true
		}
	}
	for p := range cloth {
		for _, d := range [...]image.Point{{-1, 0}, {1, 0}, {0, 1}} {
			if q := p.Add(d); !cloth[q] && q.X >= 0 && q.X < frameW && q.Y < h && (out.NRGBAAt(f*frameW+q.X, q.Y).A == 0 || d.Y == 1 && s.hem == 0) {
				out.SetNRGBA(f*frameW+q.X, q.Y, rgb(inks['k']))
			}
		}
	}
}

func rowEmpty(s *image.NRGBA, f, y int) bool {
	for x := f * frameW; x < (f+1)*frameW; x++ {
		if s.NRGBAAt(x, y).A > 0 {
			return false
		}
	}
	return true
}

// headTop: frame f's topmost row with an opaque pixel, -1 if it has none.
func headTop(s *image.NRGBA, f int) int {
	for y := range s.Bounds().Dy() {
		for x := f * frameW; x < (f+1)*frameW; x++ {
			if s.NRGBAAt(x, y).A > 0 {
				return y
			}
		}
	}
	return -1
}

// heldAt: frame f's hand, or, where the frame hides it (an arm swung behind the body), the hand of the
// nearest frame of its facing that shows one, moved with the body's bob.
func heldAt(s *image.NRGBA, f int, skin []uint32) (image.Point, bool) {
	first := f / perFacing * perFacing
	for d := range perFacing {
		for _, n := range [...]int{f - d, f + d} {
			if n < first || n >= first+perFacing || n >= s.Bounds().Dx()/frameW {
				continue
			}
			if h, ok := handAt(s, n, skin); ok {
				return h.Add(image.Pt(0, headTop(s, f)-headTop(s, n))), true
			}
		}
	}
	return image.Point{}, false
}

// handAt: the pixel of frame f's holding hand, in the frame's coordinates: the lowest skin pixel below
// the head, of the leading arm in profile and of the arm on the screen's right from the front or the back.
func handAt(s *image.NRGBA, f int, skin []uint32) (image.Point, bool) {
	top, face := headTop(s, f), f/perFacing%4
	best, found := image.Point{}, false
	for y := top + bodyFrom + 1; y < s.Bounds().Dy(); y++ {
		for x := range frameW {
			p := s.NRGBAAt(f*frameW+x, y)
			if p.A == 0 || !slices.Contains(skin, hex(p)) {
				continue
			}
			lead := x // how far toward the holding side
			if face == 2 {
				lead = frameW - 1 - x
			}
			if face%2 == 1 && x < frameW/2 { // front and back: the other arm
				continue
			}
			bl := best.X
			if face == 2 {
				bl = frameW - 1 - best.X
			}
			if !found || lead > bl || lead == bl && y > best.Y {
				best, found = image.Pt(x, y), true
			}
		}
	}
	return best, found
}

// save writes img to path as PNG, unless the file exists and force is false.
func save(path string, img image.Image, force bool) (bool, error) {
	if _, err := os.Stat(path); err == nil && !force {
		return false, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return false, err
	}
	if err := png.Encode(f, img); err != nil {
		f.Close()
		return false, err
	}
	return true, f.Close()
}

func main() {
	dir := flag.String("dir", "assets/characters", "the character sheets' folder")
	force := flag.Bool("force", false, "overwrite existing PNGs (destroys hand polish)")
	only := flag.String("only", "", "make only these (a comma list of names: the looks', Cat)")
	flag.Parse()
	wanted := func(name string) bool { return *only == "" || slices.Contains(strings.Split(*only, ","), name) }
	write := func(path string, img image.Image) {
		wrote, err := save(path, img, *force)
		if err != nil {
			log.Fatal(err)
		}
		if wrote {
			fmt.Println("wrote", path)
		} else {
			fmt.Println("kept", path, "(use -force to overwrite)")
		}
	}
	if wanted("Cat") {
		write(filepath.Join(*dir, "..", "sprites", "cat.png"), catSheet())
	}
	for _, l := range looks {
		if !wanted(l.name) {
			continue
		}
		for _, kind := range [...]string{"idle_anim", "run"} {
			f, err := os.Open(*dir + "/" + l.base + "_" + kind + "_16x16.png")
			if err != nil {
				log.Fatal(err)
			}
			sheet, err := png.Decode(f)
			f.Close()
			if err != nil {
				log.Fatal(err)
			}
			out, err := dress(sheet, l)
			if err != nil {
				log.Fatalf("%s %s: %v", l.name, kind, err)
			}
			write(*dir+"/"+l.name+"_"+kind+"_16x16.png", out)
		}
	}
}
