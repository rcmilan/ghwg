// Command sprites makes the Wave 2–3 sprites (D199) from the free Interiors sheet: it cuts boxes,
// recolors them, and draws text pixel maps in colors sampled from the sheet. It writes
// assets/sprites/<name>.png and never overwrites an existing file unless run with -force,
// so hand-polished PNGs survive.
//
//	go run ./tools/sprites [-sheet path] [-force]
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const defaultSheet = "../Modern_Interiors_Free_v2.2/Modern tiles_Free/Interiors_free/16x16/Interiors_free_16x16.png"

type cut struct {
	name  string
	r     image.Rectangle   // box on the sheet; trimmed to its real pixels
	remap map[uint32]uint32 // 0xRRGGBB → 0xRRGGBB, both sheet colors
}

func box(x, y, w, h int) image.Rectangle { return image.Rect(x, y, x+w, y+h) }

// Recolors: each target is a color of the sheet, so the result stays in its palette.
var (
	// The lavender-gray filing cabinet, as a white fridge (the white ramp of the sheet's bathtub).
	toWhite = map[uint32]uint32{
		0xbdd4f2: 0xffffff, 0xa4b7ce: 0xebe4f2, 0x99a3c6: 0xd8d0e0, 0x8e8dba: 0xc6bdd5,
		0x6e7494: 0x8b8bab, 0x5e6276: 0x6c6e85,
	}
	// Wooden counters with a pale stone top; the drawer fronts keep their wood.
	toKitchen = map[uint32]uint32{
		0xf1ce8e: 0xf8f8f8, 0xe0b870: 0xebe4f2, 0xdaa463: 0xd8d0e0, 0xdfac6c: 0xd8d0e0,
	}
	// The mirror cabinet, as a steel oven with a glowing window.
	toOven = map[uint32]uint32{
		0xd9a16a: 0xe9e9e9, 0xc78c59: 0xcecece, 0xb97a4f: 0xc0c0c0, 0xb5754d: 0xb5b5b5,
		0xb1653e: 0xa4a4a4, 0xa85f46: 0x808088, 0x6b5052: 0x55555a,
		0xf8f8f8: 0xf2b22b, 0xdbecf7: 0xed931e, 0xc4dae8: 0x6b383b,
	}
)

// cuts is T235's table, picked by the user and measured on the sheet (x, y, w, h).
var cuts = []cut{
	{"fridge", box(192, 641, 15, 31), toWhite},
	{"drawer_a", box(52, 885, 25, 21), toKitchen},
	{"drawer_b", box(80, 885, 32, 21), toKitchen},
	{"shelf_a", box(32, 296, 16, 31), nil},
	{"shelf_b", box(48, 296, 16, 31), nil},
	{"food_1", box(180, 197, 9, 8), nil},
	{"food_2", box(180, 212, 9, 9), nil},
	{"food_3", box(196, 213, 9, 8), nil},
	{"food_4", box(179, 227, 11, 11), nil},
	{"food_5", box(196, 227, 9, 10), nil},
	{"painting_scary_a", box(2, 1130, 13, 15), nil},
	{"painting_scary_b", box(18, 1130, 13, 15), nil},
	{"painting_abstract", box(34, 1130, 13, 15), nil},
	{"painting_a", box(2, 1162, 13, 15), nil},
	{"painting_b", box(2, 1194, 13, 15), nil},
	{"painting_c", box(2, 1226, 13, 15), nil},
	{"painting_wide_a", box(130, 330, 28, 13), nil},
	{"painting_wide_b", box(130, 362, 28, 13), nil},
	{"plant_small", box(2, 790, 12, 14), nil},
	{"plant_big", box(167, 713, 18, 31), nil},
	{"plant_medium", box(193, 722, 14, 23), nil},
	{"plant_palm", box(213, 704, 23, 31), nil},
	{"lamp_a", box(192, 856, 15, 33), nil},
	{"lamp_b", box(208, 856, 15, 33), nil},
	{"lamp_c", box(193, 904, 13, 33), nil},
	{"lamp_d", box(208, 904, 15, 33), nil},
	{"vanity", box(34, 779, 28, 30), nil},
	{"mirror_standing", box(102, 1080, 20, 32), nil},
	{"wardrobe_broken", box(162, 916, 27, 39), nil},
	{"fireplace", box(64, 1088, 32, 40), nil},
	{"vase_a", box(1, 1068, 15, 16), nil},
	{"vase_b", box(17, 1068, 15, 16), nil},
	{"vase_c", box(33, 1068, 15, 16), nil},
	{"vase_d", box(1, 1100, 15, 16), nil},
	{"vase_e", box(17, 1100, 15, 16), nil},
	{"vase_f", box(33, 1100, 15, 16), nil},
	{"cabinet_mirror", box(0, 917, 16, 20), toOven},
	{"cabinet", box(0, 949, 16, 20), nil},
}

// palette maps pixel-map characters to sheet pixels; '.' is transparent (or the base shows).
var palette = map[byte]image.Point{
	'o': {16, 12},   // 3a3a50 outline
	'O': {178, 387}, // 46465e soft outline
	'W': {5, 93},    // ffffff
	'w': {87, 392},  // ebe4f2 white cloth, light
	'v': {69, 33},   // d8d0e0
	'u': {215, 456}, // c6bdd5
	'l': {86, 392},  // b2aecb
	'L': {72, 392},  // 8b8bab
	'b': {46, 783},  // dbecf7 water, light
	'c': {215, 368}, // 83b8da
	'B': {224, 160}, // 4985cc
	'r': {225, 466}, // e63f38 red
	'p': {181, 216}, // ec5555
	'R': {138, 393}, // a13a30
	'g': {211, 580}, // a4dc77 green
	'G': {233, 457}, // 9bc246
	'H': {182, 288}, // 2b792b
	'y': {80, 427},  // fff59a yellow
	'Y': {76, 407},  // f8d239
	's': {147, 417}, // cecece steel
	'S': {147, 418}, // a4a4a4
	'm': {16, 25},   // 787878
	'k': {130, 140}, // 1e1e2e dark window
	'K': {73, 392},  // 565972 window glint
	'a': {124, 141}, // d9a16a wood, light
	'e': {93, 20},   // c78c59 wood
	'D': {88, 395},  // a85f46 wood, dark
	'n': {197, 736}, // 6b5052 wood, darkest
	'Z': {78, 407},  // f2b22b gold, shade
}

type drawing struct {
	name string
	base image.Rectangle // optional sheet area drawn under the map, untrimmed
	rows string
}

// drawings is T236: icons and props drawn in the sheet's outline and palette.
var drawings = []drawing{
	{name: "icon_water", rows: `
		...oo...
		..oBBo..
		..oBBo..
		..obco..
		.obbcco.
		obWbccco
		obWbccco
		oBBBBBBo
		obWbccco
		obWbccco
		obbbccco
		obbcccco
		.oooooo.`},
	{name: "icon_towel", rows: `
		.ooooooooooo.
		owwwwwwwwcwvo
		owwwwwwwwcvvo
		ouvvvvvvvBvuo
		oowwwwwwwcwvo
		owwwwwwwwcwuo
		oouuuuuuuBuuo
		owwwwwwwwcwvo
		ovvvvvvvvBvuo
		.ooooooooooo.`},
	{name: "lock_red", rows: `
		.oooooo.
		osssssSo
		osrrrrSo
		osrpprSo
		osRRRRSo
		osssssSo
		osmmmmSo
		osssssSo
		osSSSSSo
		.oooooo.`},
	{name: "lock_green", rows: `
		.oooooo.
		osssssSo
		osggggSo
		osgWWgSo
		osHHHHSo
		osssssSo
		osmmmmSo
		osssssSo
		osSSSSSo
		.oooooo.`},
	{name: "icon_thermometer", rows: `
		..ooo..
		..oWo..
		..oWo..
		..oro..
		..oro..
		..oro..
		..oro..
		..oro..
		.oorpo.
		orrrrpo
		orrrrro
		oRrrrro
		.oRRRo.
		..ooo..`},
	{name: "mirror_covered", base: box(48, 1058, 16, 32), rows: `
		................
		................
		................
		................
		..oooooooooooo..
		.owwwwwwwwwwwvo.
		owwwwwwwwwwwwvvo
		owWwwwwvwwwwwvuo
		owWwwwwvwwwwwvuo
		owWwwwvuwwwwwvuo
		owWwwwvuwwwwvvuo
		owwwwwvuwwwwvuuo
		owwwwvuuwwwwvuuo
		owwwwvuuwwwvvuuo
		owwwwvuuwwwvuuuo
		owwwvuuuwwwvuuuo
		owwwvuuuwwvvuuuo
		owwvuuuuwwvuuuuo
		owwvuuuuwwvuuuuo
		owwvuuuuwwvuuuuo
		owvvuuuuwvvuuuuo
		owvuuuulwvuuuulo
		ovvuuuulvvuuuulo
		ouvuuuulvvuuuulo
		oouuooolloouuloo
		................
		................
		................
		................
		................
		................
		................`},
	{name: "icon_key", rows: `
			..oooo..
			.oyYYZo.
			oyYooZZo
			oYo..oZo
			oYo..oZo
			oYZooZZo
			.oZZZZo.
			..oYZo..
			..oYZo..
			..oYZooo
			..oYZYZo
			..oYZooo
			..oYZYZo
			..ooooo.`},
	{name: "door_boarded", rows: `
			oooooooooooooooo
			oDDDDDDDDDDDDDDo
			oDaaaaaaaaaaaaDo
			oDaeeeeeeeeeeaDo
			oDaooooooooooaDo
			oDaokKKkkkkkoaDo
			oDaokKkkkkkkoaDo
			oooooooooooooooo
			aaSaaaaaaaaaaSaa
			aaaeaaeaaeaaeaaa
			DDDDDDDDDDDDDDDD
			oooooooooooooooo
			oDaokkkkkkkkoaDo
			oDaokkkkkkkkoaDo
			oDaokkkkkkkkoaDo
			oDaooooooooooaDo
			oDaeeeeeeeeeeaDo
			oooooooooooooooo
			aaSaaaaaaaaaaSaa
			aaaeaaeaaeaaeaaa
			DDDDDDDDDDDDDDDD
			oooooooooooooooo
			oDaeeeeeeeeeeaDo
			oDaeeeeeeeeeeaDo
			oDaeeeeeeeeeeaDo
			oooooooooooooooo
			aaSaaaaaaaaaaSaa
			aaaeaaeaaeaaeaaa
			DDDDDDDDDDDDDDDD
			oooooooooooooooo
			oDaeeeeeeeeeeaDo
			oooooooooooooooo`},
	{name: "stock_box", rows: `
			.oooooooooooooooooo.
			oaaaaaaaaaaaaaaaaaao
			oaaaaaaaaaaaaaaaaaao
			oeeeeeeeeeeeeeeeeeeo
			oDDDDDDDDDDDDDDDDDDo
			oaeeeeeeeeeeeeeeeeDo
			oannnnnnnnnnnnnnnnDo
			oanwwwwwwwwwwwwwvnDo
			oanwwwwwwwwwwwwwvnDo
			oanwwwwwwwwwwwwwvnDo
			oanwwwwwwwwwwwwwvnDo
			oanwwwwwwwwwwwwwvnDo
			oanwwwwwwwwwwwwwvnDo
			oanwwwwwwwwwwwwwvnDo
			oanwwwwwwwwwwwwwvnDo
			oanwwwwwwwwwwwwwvnDo
			oanwwwwwwwwwwwwwvnDo
			oanwwwwwwwwwwwwwvnDo
			oanwwwwwwwwwwwwwvnDo
			oanvvvvvvvvvvvvvvnDo
			oannnnnnnnnnnnnnnnDo
			oDDDDDDDDDDDDDDDDDDo
			oooooooooooooooooooo`},
	{name: "sparkle", rows: `
		..........o..
		....o....oWo.
		...oWo....o..
		...oWo.......
		.ooyWyoo.....
		oWWWWWWWo....
		.ooyWyoo.....
		...oWo.......
		...oWo...o...
		....o...oyo..
		.........o...`},
}

// trim shrinks r to the bounding box of its non-transparent pixels.
func trim(src image.Image, r image.Rectangle) image.Rectangle {
	out := image.Rectangle{}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			if _, _, _, a := src.At(x, y).RGBA(); a > 0 {
				out = out.Union(image.Rect(x, y, x+1, y+1))
			}
		}
	}
	return out
}

// soft replaces the sheet's flat grey drop shadow (#A79796), which doesn't fit the floors: a translucent
// dark in the same shape (02-rooms S7, as 02-rooms/tools/props.py).
var soft = color.NRGBA{24, 20, 32, 60}

// nrgba reads a sheet pixel, with the grey drop shadow softened.
func nrgba(src image.Image, x, y int) color.NRGBA {
	p := color.NRGBAModel.Convert(src.At(x, y)).(color.NRGBA)
	if p == (color.NRGBA{0xA7, 0x97, 0x96, 255}) {
		return soft
	}
	return p
}

// cutOut copies c's trimmed box from the sheet, remapping its colors.
func cutOut(sheet image.Image, c cut) *image.NRGBA {
	r := trim(sheet, c.r)
	out := image.NewNRGBA(image.Rect(0, 0, r.Dx(), r.Dy()))
	for y := 0; y < r.Dy(); y++ {
		for x := 0; x < r.Dx(); x++ {
			p := nrgba(sheet, r.Min.X+x, r.Min.Y+y)
			if to, ok := c.remap[uint32(p.R)<<16|uint32(p.G)<<8|uint32(p.B)]; ok {
				p.R, p.G, p.B = uint8(to>>16), uint8(to>>8), uint8(to)
			}
			out.SetNRGBA(x, y, p)
		}
	}
	return out
}

// render draws d's pixel map over its base, each character in the color of its palette pixel.
func render(sheet image.Image, d drawing, pal map[byte]image.Point) *image.NRGBA {
	rows := strings.Fields(d.rows)
	w, h := d.base.Dx(), d.base.Dy()
	if d.base.Empty() {
		w, h = len(rows[0]), len(rows)
	}
	for y, row := range rows {
		if len(row) != w || y >= h {
			log.Panicf("%s: row %d is %d wide (want %d) or past height %d", d.name, y, len(row), w, h)
		}
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if !d.base.Empty() {
				out.SetNRGBA(x, y, nrgba(sheet, d.base.Min.X+x, d.base.Min.Y+y))
			}
		}
	}
	for y, row := range rows {
		for x := 0; x < len(row); x++ {
			if row[x] == '.' {
				continue
			}
			p, ok := pal[row[x]]
			if !ok {
				log.Panicf("%s: no palette color for %q at %d,%d", d.name, row[x], x, y)
			}
			out.SetNRGBA(x, y, nrgba(sheet, p.X, p.Y))
		}
	}
	return out
}

// sprites makes every sprite by name.
func sprites(sheet image.Image) map[string]*image.NRGBA {
	out := map[string]*image.NRGBA{}
	for _, c := range cuts {
		out[c.name] = cutOut(sheet, c)
	}
	for _, d := range drawings {
		out[d.name] = render(sheet, d, palette)
	}
	return out
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
	sheetPath := flag.String("sheet", defaultSheet, "path to Interiors_free_16x16.png")
	force := flag.Bool("force", false, "overwrite existing PNGs (destroys hand polish)")
	flag.Parse()

	f, err := os.Open(*sheetPath)
	if err != nil {
		log.Fatal(err)
	}
	sheet, err := png.Decode(f)
	f.Close()
	if err != nil {
		log.Fatal(err)
	}
	dir := filepath.Join("assets", "sprites")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatal(err)
	}
	all := sprites(sheet)
	names := make([]string, 0, len(all))
	for n := range all {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		wrote, err := save(filepath.Join(dir, n+".png"), all[n], *force)
		if err != nil {
			log.Fatal(err)
		}
		if wrote {
			fmt.Println("wrote", n)
		} else {
			fmt.Println("kept ", n)
		}
	}
}
