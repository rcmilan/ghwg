package main

import (
	"image"
	"image/color"
	"path/filepath"
	"testing"
)

// fakeSheet: 24 frames of 16x32, each a body block whose top row differs per frame (the bob).
func fakeSheet() (*image.NRGBA, func(f int) int) {
	top := func(f int) int { return 9 + f%3 }
	s := image.NewNRGBA(image.Rect(0, 0, 24*frameW, 32))
	for f := range 24 {
		for y := top(f); y < 32; y++ {
			for x := 3; x < 13; x++ {
				s.SetNRGBA(f*frameW+x, y, color.NRGBA{uint8(f * 10), uint8(y * 8), uint8(x * 16), 255})
			}
		}
	}
	return s, top
}

// The hat lands from each frame's head top up, in its facing's drawing; every other pixel is the sheet's.
func TestStampFollowsHead(t *testing.T) {
	s, top := fakeSheet()
	out, err := stamp(s)
	if err != nil {
		t.Fatal(err)
	}
	for f := range 24 {
		hat := hats[f/perFacing]
		y0 := top(f) + over - len(hat)
		for y := range 32 {
			for x := range frameW {
				want := s.NRGBAAt(f*frameW+x, y)
				if j := y - y0; j >= 0 && j < len(hat) {
					if c, ok := inks[rune(hat[j][x])]; ok {
						want = color.NRGBA{uint8(c >> 16), uint8(c >> 8), uint8(c), 255}
					}
				}
				if got := out.NRGBAAt(f*frameW+x, y); got != want {
					t.Fatalf("frame %d (%d,%d) = %v, want %v", f, x, y, got, want)
				}
			}
		}
	}
}

func TestHatsWellFormed(t *testing.T) {
	for f, hat := range hats {
		for j, row := range hat {
			if len(row) != frameW {
				t.Fatalf("facing %d row %d is %d wide, want %d", f, j, len(row), frameW)
			}
			for _, r := range row {
				if _, ok := inks[r]; !ok && r != '.' {
					t.Fatalf("facing %d row %d: unknown ink %q", f, j, r)
				}
			}
		}
	}
}

// A head too near the frame's top is refused, not clipped.
func TestStampRefusesClip(t *testing.T) {
	s := image.NewNRGBA(image.Rect(0, 0, frameW, 32))
	s.SetNRGBA(8, 2, color.NRGBA{1, 2, 3, 255})
	if _, err := stamp(s); err == nil {
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
