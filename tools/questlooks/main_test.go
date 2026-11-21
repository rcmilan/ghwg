package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

const fakeSkin = 0xffcbb0

// fakeSheet: 24 frames of 16x32, each a body block whose top row differs per frame (the bob), with one
// skin pixel as its hand, on the right, lower on odd frames.
func fakeSheet() (*image.NRGBA, func(f int) int, func(f int) image.Point) {
	top := func(f int) int { return 9 + f%3 }
	hand := func(f int) image.Point { return image.Pt(10, 26+f%2) }
	s := image.NewNRGBA(image.Rect(0, 0, 24*frameW, 32))
	for f := range 24 {
		for y := top(f); y < 32; y++ {
			for x := 3; x < 13; x++ {
				s.SetNRGBA(f*frameW+x, y, color.NRGBA{uint8(f * 10), uint8(y * 8), uint8(x * 16), 255})
			}
		}
		s.SetNRGBA(f*frameW+hand(f).X, hand(f).Y, rgb(fakeSkin))
	}
	return s, top, hand
}

// An accessory lands from each frame's head top (or at its hand) in its facing's drawing; every other pixel
// is the sheet's.
func TestAccessoryFollowsHeadAndHand(t *testing.T) {
	s, top, hand := fakeSheet()
	skins["fake"] = []uint32{fakeSkin}
	for _, l := range looks {
		l.base, l.swap = "fake", swap{}
		out, err := dress(s, l)
		if err != nil {
			t.Fatalf("%s: %v", l.name, err)
		}
		for f := range 24 {
			a := l.accessories[0]
			art := a.art[f/perFacing]
			at := image.Pt(0, top(f)+a.over-len(art))
			if a.hand {
				at = hand(f).Add(a.at[f/perFacing])
				at = image.Pt(min(max(0, at.X), frameW-len(art[0])), min(at.Y, 32-len(art)))
			}
			for y := range 32 {
				for x := range frameW {
					want := s.NRGBAAt(f*frameW+x, y)
					if i, j := x-at.X, y-at.Y; j >= 0 && j < len(art) && i >= 0 && i < len(art[j]) {
						if c, ok := inks[rune(art[j][i])]; ok {
							want = rgb(c)
						} else if art[j][i] == erase {
							want = color.NRGBA{}
						}
					}
					if got := out.NRGBAAt(f*frameW+x, y); got != want {
						t.Fatalf("%s frame %d (%d,%d) = %v, want %v", l.name, f, x, y, got, want)
					}
				}
			}
		}
	}
}

// A color swaps by zone: the legs ask the body, the body the head; a color in none stays.
func TestSwapZones(t *testing.T) {
	s := swap{head: map[uint32]uint32{1: 10, 2: 20}, body: map[uint32]uint32{2: 21}, legs: map[uint32]uint32{3: 32}}
	for _, c := range [...]struct {
		c    uint32
		row  int
		want uint32
	}{{1, 0, 10}, {1, bodyFrom, 10}, {1, legsFrom, 10}, {2, bodyFrom - 1, 20}, {2, bodyFrom, 21}, {2, legsFrom, 21}, {3, legsFrom - 1, 3}, {3, legsFrom, 32}, {4, 5, 4}} {
		if got := s.to(c.c, c.row); got != c.want {
			t.Errorf("color %d at row %d = %d, want %d", c.c, c.row, got, c.want)
		}
	}
}

// Every look is drawable on its real base sheets: head drawings 16 wide, known inks, every swapped color one
// the base has, no frame refused; and each look's sheets differ from every other's and from its base's.
func TestLooksOnTheirSheets(t *testing.T) {
	made := map[string]*image.NRGBA{}
	for _, l := range looks {
		for _, a := range l.accessories {
			for f, art := range a.art {
				for j, row := range art {
					if !a.hand && len(row) != frameW {
						t.Fatalf("%s facing %d row %d is %d wide, want %d", l.name, f, j, len(row), frameW)
					}
					for _, r := range row {
						if _, ok := inks[r]; !ok && r != '.' && r != erase {
							t.Fatalf("%s facing %d row %d: unknown ink %q", l.name, f, j, r)
						}
					}
				}
			}
		}
		has := map[uint32]bool{} // the base's colors, over both sheets
		defer func() {
			for _, zone := range [...]map[uint32]uint32{l.swap.head, l.swap.body, l.swap.legs} {
				for c := range zone {
					if !has[c] {
						t.Errorf("%s swaps #%06x, which %s does not have", l.name, c, l.base)
					}
				}
			}
		}()
		for _, kind := range [...]string{"idle_anim", "run"} {
			f, err := os.Open("../../assets/characters/" + l.base + "_" + kind + "_16x16.png")
			if err != nil {
				t.Fatal(err)
			}
			img, err := png.Decode(f)
			f.Close()
			if err != nil {
				t.Fatal(err)
			}
			src := image.NewNRGBA(img.Bounds())
			draw.Draw(src, src.Bounds(), img, image.Point{}, draw.Src)
			for i := 0; i < len(src.Pix); i += 4 {
				if src.Pix[i+3] > 0 {
					has[uint32(src.Pix[i])<<16|uint32(src.Pix[i+1])<<8|uint32(src.Pix[i+2])] = true
				}
			}
			out, err := dress(src, l)
			if err != nil {
				t.Fatalf("%s %s: %v", l.name, kind, err)
			}
			if string(out.Pix) == string(src.Pix) {
				t.Errorf("%s %s is its base's sheet", l.name, kind)
			}
			for other, o := range made {
				if string(o.Pix) == string(out.Pix) {
					t.Errorf("%s %s is %s", l.name, kind, other)
				}
			}
			made[l.name+" "+kind] = out
		}
	}
}

// A head too near the frame's top is refused, not clipped.
func TestDressRefusesClip(t *testing.T) {
	s := image.NewNRGBA(image.Rect(0, 0, frameW, 32))
	s.SetNRGBA(8, 2, color.NRGBA{1, 2, 3, 255})
	if _, err := dress(s, looks[0]); err == nil {
		t.Fatal("a hat that leaves the frame was stamped")
	}
}

func TestSaveKeepsExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.png")
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	if wrote, err := save(path, img, false); err != nil || !wrote {
		t.Fatalf("first save: %v %v", wrote, err)
	}
	if wrote, err := save(path, img, false); err != nil || wrote {
		t.Fatalf("second save without force: wrote %v, err %v", wrote, err)
	}
	if wrote, err := save(path, img, true); err != nil || !wrote {
		t.Fatalf("forced save: %v %v", wrote, err)
	}
}
