// Command witchhat makes Morwenna's sheets (08 W5): Amelia's idle and run sheets with a short pointed
// witch hat stamped on every frame, on the head's top row, so it bobs with her. It writes
// assets/characters/Morwenna_{idle_anim,run}_16x16.png and never overwrites an existing file unless run
// with -force, so hand-polished PNGs survive.
//
//	go run ./tools/witchhat [-force]
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
)

// Frames are 16x32, 6 per facing, in entities.Facing's order: right, up, left, down.
const frameW, perFacing = 16, 6

// inks: the hat's colors. The outlines are the sheet's own; the violets and the band are the hat's alone
// (game/witch.go keeps them as drawn under any look).
var inks = map[rune]uint32{
	'k': 0x3a3a50, 'o': 0x46465e, // outline: dark below, lighter on top, as the sheet's heads
	'd': 0x4d4263, 'b': 0x5a4d75, 'h': 0x6c5981, // the felt: shade, body, the top-left light
	'p': 0x9f74a8, 'q': 0x805e8e, // the band
}

// over: the hat's last rows sit on the hair, from the head's top row down.
const over = 4

// hats: one drawing per facing, 16 wide, '.' leaving the sheet's pixel. The tip bends back.
var hats = [4][]string{
	{ // right
		"....ooo.........",
		"...ohhbo........",
		"...kkbbbo.......",
		".....obbdk......",
		"....ohbbbdk.....",
		"....ohbbbbdk....",
		"...ohbbbbbdk....",
		"...oppppppqk....",
		".oohbbbbbbbdkkk.",
		"ohhbbbbbbbbbbddk",
		".kkkkkkkkkkkkkk.",
	},
	{ // up
		"......oo........",
		".....ohbo.......",
		".....ohbdk......",
		"....ohbbdk......",
		"....ohbbbdk.....",
		"...ohbbbbdk.....",
		"...ohbbbbbdk....",
		"...oppppppqk....",
		".oohbbbbbbbdkkk.",
		"ohhbbbbbbbbbbddk",
		".kkkkkkkkkkkkkk.",
	},
	{ // left
		".........ooo....",
		"........ohbbo...",
		".......ohbbkk...",
		"......ohbdk.....",
		".....ohbbbdk....",
		"....ohbbbbdk....",
		"....ohbbbbbdk...",
		"....oppppppqk...",
		".ooohbbbbbbbdkk.",
		"ohhbbbbbbbbbbddk",
		".kkkkkkkkkkkkkk.",
	},
	{ // down
		"........oo......",
		".......ohbo.....",
		"......ohbdk.....",
		"......ohbbdk....",
		".....ohbbbdk....",
		".....ohbbbbdk...",
		"....ohbbbbbdk...",
		"....oppppppqk...",
		".ooohbbbbbbbdkk.",
		"ohhbbbbbbbbbbddk",
		".kkkkkkkkkkkkkk.",
	},
}

// stamp: a copy of sheet with each frame's hat drawn from its head's top row (the frame's topmost opaque
// row) up. It never moves the body: a hat that would leave the frame is an error.
func stamp(sheet image.Image) (*image.NRGBA, error) {
	b := sheet.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), sheet, b.Min, draw.Src)
	for f := range b.Dx() / frameW {
		hat := hats[f/perFacing%len(hats)]
		top := headTop(out, f)
		y0 := top + over - len(hat)
		if top < 0 || y0 < 0 {
			return nil, fmt.Errorf("frame %d: head top at row %d, the hat needs %d rows above it", f, top, len(hat)-over)
		}
		for j, row := range hat {
			for i, r := range row {
				if c, ok := inks[r]; ok {
					out.SetNRGBA(f*frameW+i, y0+j, color.NRGBA{uint8(c >> 16), uint8(c >> 8), uint8(c), 255})
				}
			}
		}
	}
	return out, nil
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
	flag.Parse()
	for _, kind := range [...]string{"idle_anim", "run"} {
		f, err := os.Open(*dir + "/Amelia_" + kind + "_16x16.png")
		if err != nil {
			log.Fatal(err)
		}
		sheet, err := png.Decode(f)
		f.Close()
		if err != nil {
			log.Fatal(err)
		}
		out, err := stamp(sheet)
		if err != nil {
			log.Fatalf("%s: %v", kind, err)
		}
		path := *dir + "/Morwenna_" + kind + "_16x16.png"
		wrote, err := save(path, out, *force)
		if err != nil {
			log.Fatal(err)
		}
		if wrote {
			fmt.Println("wrote", path)
		} else {
			fmt.Println("kept", path, "(use -force to overwrite)")
		}
	}
}
