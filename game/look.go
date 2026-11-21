package game

import (
	"image"
	"image/color"
	"math"
	"math/rand/v2"
	"strconv"
)

// Recolor (D166): every character sheet's colors are mapped to regions, so hair, skin and clothes can be
// recolored.
type region uint8

const (
	keep region = iota // outline, eyes, as drawn
	hair
	skin
	top
	bottom
	accent // a shirt or collar
	regions
)

// bodyRow: a sheet's rows from here down are the body (D166); a color may be one region on the head and
// another on the body (Adam's green hair and trousers).
const bodyRow = 22

// palettes: each sheet's colors (0xRRGGBB) → its region on the head and on the body.
var palettes = map[string]map[uint32][2]region{
	"Bob": {
		0x3a3a50: {keep, keep}, 0x46465e: {keep, keep}, 0x6c5981: {keep, keep}, 0xd3a38d: {keep, keep},
		0x565972: {keep, top},
		0x555157: {hair, hair}, 0x5d585f: {hair, hair}, 0x716b6e: {hair, hair},
		0xb59e95: {skin, skin}, 0xc9a495: {skin, skin}, 0xffcbb0: {skin, skin}, 0xf6ae9f: {skin, skin}, 0xe19b9b: {skin, skin},
		0x6c6e85: {top, top}, 0x787d93: {top, top},
		0xc2b8d5: {accent, accent}, 0xd8d0e0: {accent, accent},
	},
	"Adam": {
		0x3a3a50: {keep, keep}, 0x46465e: {keep, keep}, 0x565972: {keep, keep}, 0xd3a38d: {keep, keep}, 0x6c5981: {keep, keep}, 0x787d93: {keep, keep},
		0x959d58: {hair, hair}, 0x5f694a: {hair, hair}, 0x687253: {hair, bottom}, 0x5d6043: {hair, bottom},
		0xbba386: {skin, skin}, 0xffcbb0: {skin, skin}, 0xf6ae9f: {skin, skin}, 0xe19b9b: {skin, skin}, 0xffb893: {skin, skin}, 0xf69784: {skin, skin},
		0x805e8e: {top, top}, 0x9f74a8: {top, top},
	},
	"Alex": {
		0x3a3a50: {keep, keep}, 0x46465e: {keep, keep}, 0x565972: {keep, keep}, 0xd3a38d: {keep, keep}, 0x6c5981: {keep, keep},
		0x8d7051: {hair, hair}, 0x957350: {hair, hair}, 0xba8d5e: {hair, hair}, 0x8a6552: {hair, hair}, 0x6f5446: {hair, hair},
		0xe19b9b: {skin, skin}, 0xf6ae9f: {skin, skin}, 0xffcbb0: {skin, skin}, 0xffb893: {skin, skin}, 0xf69784: {skin, skin},
		0xa2394b: {top, top}, 0xae4a52: {top, top}, 0x6f494d: {top, top}, 0x5a444a: {hair, top},
		0x8b8bab: {accent, accent}, 0x9da3b7: {accent, accent},
		0x787d93: {bottom, bottom}, 0x8091a5: {bottom, bottom},
	},
	"Amelia": {
		0x3a3a50: {keep, keep}, 0x46465e: {keep, keep}, 0x565972: {keep, keep}, 0x6c5981: {keep, keep}, 0x787d93: {keep, keep},
		0x8a6552: {hair, hair}, 0x957350: {hair, hair}, 0xba8d5e: {hair, hair}, 0x8d7051: {hair, hair}, 0x674d49: {hair, hair},
		0xbf8b78: {skin, skin}, 0xb57972: {skin, skin}, 0xa77a67: {skin, skin}, 0xaa5e56: {skin, skin}, 0xb2736d: {skin, skin}, 0xb58472: {skin, skin},
		0xa85377: {top, top}, 0xb95d72: {top, top},
		0xb35e3f: {bottom, bottom}, 0xc78c59: {bottom, bottom}, 0xab4a36: {bottom, bottom},
	},
}

// look: a base color per region, 0 for as drawn. A region's mean lightness lands on its base's; each
// shade keeps its offset from that mean, so the shading survives.
type look [regions]uint32

// Placeholder ramps for Guests (D166).
var (
	hairs   = [...]uint32{0x2b2630, 0x5a3a28, 0x8a5a34, 0xd8b45c, 0xb04a2a, 0xa8a8b0}
	skins   = [...]uint32{0xffd2b4, 0xf0b088, 0xc88a60, 0x9a6040, 0x664030}
	tops    = [...]uint32{0xc83c3c, 0x3c6ec8, 0x3ca05a, 0xe0a030, 0x8a4ab4, 0x40b4b4, 0xe070a0, 0xe8e8e8}
	bottoms = [...]uint32{0x3a4a78, 0x2a2a30, 0xb49a6a, 0x707078, 0x6a4a34}
)

// The Priests (D166): Bob in a black cassock with a white collar, Father K dark-haired, Father M white.
var (
	fatherK = struct {
		name string
		look look
	}{nameFatherK, look{hair: 0x3a2a24, top: 0x24202a, accent: 0xf2f2f6}}
	fatherM = struct {
		name string
		look look
	}{nameFatherM, look{hair: 0xe4e2e8, top: 0x24202a, accent: 0xf2f2f6}}
)

// dressed: a new Guest's name, base character and a random look's seed (D166), from the talk's random
// source so the sim's stays untouched.
func (g *Game) dressed(base string) string {
	return base + "#" + strconv.FormatUint(g.chat.Uint64(), 10)
}

func randomLook(r *rand.Rand) look {
	return look{hair: hairs[r.IntN(len(hairs))], skin: skins[r.IntN(len(skins))], top: tops[r.IntN(len(tops))], bottom: bottoms[r.IntN(len(bottoms))]}
}

// recolor: a copy of sheet with each region l sets moved onto its base color.
func recolor(sheet *image.RGBA, pal map[uint32][2]region, l look) *image.RGBA {
	var sum, n [regions]float64
	for c, rs := range pal {
		_, _, lt := hsl(c)
		for _, r := range rs {
			sum[r] += lt
			n[r]++
		}
	}
	out := image.NewRGBA(sheet.Bounds())
	copy(out.Pix, sheet.Pix)
	b := sheet.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			p := sheet.RGBAAt(x, y)
			r := pal[uint32(p.R)<<16|uint32(p.G)<<8|uint32(p.B)][min(1, y/bodyRow)]
			if p.A == 0 || l[r] == 0 || r == keep {
				continue
			}
			h, s, bl := hsl(l[r])
			_, _, lt := hsl(uint32(p.R)<<16 | uint32(p.G)<<8 | uint32(p.B))
			out.SetRGBA(x, y, rgb(h, s, min(1, max(0, bl+lt-sum[r]/n[r])), p.A))
		}
	}
	return out
}

func hsl(c uint32) (h, s, l float64) {
	r, g, b := float64(c>>16&255)/255, float64(c>>8&255)/255, float64(c&255)/255
	hi, lo := max(r, g, b), min(r, g, b)
	l = (hi + lo) / 2
	if d := hi - lo; d > 0 {
		s = d / (1 - math.Abs(2*l-1))
		switch hi {
		case r:
			h = math.Mod((g-b)/d+6, 6)
		case g:
			h = (b-r)/d + 2
		default:
			h = (r-g)/d + 4
		}
	}
	return h * 60, s, l
}

func rgb(h, s, l float64, a uint8) color.RGBA {
	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	var r, g, b float64
	switch int(h/60) % 6 {
	case 0:
		r, g = c, x
	case 1:
		r, g = x, c
	case 2:
		g, b = c, x
	case 3:
		g, b = x, c
	case 4:
		r, b = x, c
	default:
		r, b = c, x
	}
	m := l - c/2
	u := func(v float64) uint8 { return uint8(math.Round((v + m) * 255)) }
	return color.RGBA{u(r), u(g), u(b), a}
}
