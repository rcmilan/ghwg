package main

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

var (
	red  = color.NRGBA{200, 10, 10, 255}
	blue = color.NRGBA{10, 10, 200, 255}
)

// fakeSheet is 8×8 and transparent except a 2×3 sprite at (3,2) and two palette swatches.
func fakeSheet() *image.NRGBA {
	s := image.NewNRGBA(image.Rect(0, 0, 8, 8))
	for y := 2; y < 5; y++ {
		for x := 3; x < 5; x++ {
			s.Set(x, y, color.NRGBA{uint8(x * 30), uint8(y * 30), 7, 255})
		}
	}
	s.Set(0, 7, red)
	s.Set(7, 7, blue)
	return s
}

func TestCutEqualsSheetPixels(t *testing.T) {
	s := fakeSheet()
	got := cutOut(s, cut{name: "x", r: image.Rect(1, 1, 6, 6)}) // padded box, trimmed to the sprite
	if got.Bounds() != image.Rect(0, 0, 2, 3) {
		t.Fatalf("trimmed bounds = %v, want 2×3", got.Bounds())
	}
	for y := 0; y < 3; y++ {
		for x := 0; x < 2; x++ {
			if got.NRGBAAt(x, y) != s.NRGBAAt(3+x, 2+y) {
				t.Fatalf("pixel %d,%d = %v, want %v", x, y, got.NRGBAAt(x, y), s.NRGBAAt(3+x, 2+y))
			}
		}
	}
}

func TestCutRemapsColors(t *testing.T) {
	s := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	s.Set(0, 0, red)
	got := cutOut(s, cut{r: s.Bounds(), remap: map[uint32]uint32{0xc80a0a: 0x0a0ac8}})
	if got.NRGBAAt(0, 0) != blue {
		t.Fatalf("remapped = %v, want %v", got.NRGBAAt(0, 0), blue)
	}
}

// 02-rooms S7: the sheet's flat grey drop shadow becomes a translucent dark, in cuts and under maps.
func TestShadowSoftened(t *testing.T) {
	s := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	s.Set(0, 0, color.NRGBA{0xA7, 0x97, 0x96, 255})
	s.Set(1, 0, red)
	got := cutOut(s, cut{r: s.Bounds()})
	if got.NRGBAAt(0, 0) != soft || got.NRGBAAt(1, 0) != red {
		t.Fatalf("cut = %v %v, want %v %v", got.NRGBAAt(0, 0), got.NRGBAAt(1, 0), soft, red)
	}
	if got := render(s, drawing{base: s.Bounds(), rows: ".."}, nil); got.NRGBAAt(0, 0) != soft {
		t.Fatalf("base = %v, want %v", got.NRGBAAt(0, 0), soft)
	}
}

func TestPixelMapRendersItsPalette(t *testing.T) {
	pal := map[byte]image.Point{'r': {0, 7}, 'b': {7, 7}}
	got := render(fakeSheet(), drawing{rows: "r.b\nbr."}, pal)
	want := [][]color.NRGBA{{red, {}, blue}, {blue, red, {}}}
	if got.Bounds() != image.Rect(0, 0, 3, 2) {
		t.Fatalf("bounds = %v", got.Bounds())
	}
	for y, row := range want {
		for x, c := range row {
			if got.NRGBAAt(x, y) != c {
				t.Fatalf("pixel %d,%d = %v, want %v", x, y, got.NRGBAAt(x, y), c)
			}
		}
	}
}

func TestPixelMapDrawsOverBase(t *testing.T) {
	pal := map[byte]image.Point{'b': {7, 7}}
	s := fakeSheet()
	got := render(s, drawing{base: image.Rect(3, 2, 5, 5), rows: "b.\n..\n.b"}, pal)
	if got.NRGBAAt(0, 0) != blue || got.NRGBAAt(1, 2) != blue || got.NRGBAAt(1, 0) != s.NRGBAAt(4, 2) {
		t.Fatalf("map over base wrong: %v %v %v", got.NRGBAAt(0, 0), got.NRGBAAt(1, 2), got.NRGBAAt(1, 0))
	}
}

func TestExistingFileUntouchedWithoutForce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.png")
	os.WriteFile(path, []byte("polished"), 0o644)
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))

	if wrote, err := save(path, img, false); err != nil || wrote {
		t.Fatalf("save without force: wrote=%v err=%v", wrote, err)
	}
	if b, _ := os.ReadFile(path); string(b) != "polished" {
		t.Fatal("existing file was changed without -force")
	}
	if wrote, err := save(path, img, true); err != nil || !wrote {
		t.Fatalf("save with force: wrote=%v err=%v", wrote, err)
	}
	if b, _ := os.ReadFile(path); !bytes.HasPrefix(b, []byte("\x89PNG")) {
		t.Fatal("-force did not overwrite")
	}
}

// Every pixel of every sprite is a color of the sheet (or of the Lobby mirror, which is on it).
func TestSpritesUseOnlySheetColors(t *testing.T) {
	f, err := os.Open(filepath.FromSlash("../../" + defaultSheet))
	if err != nil {
		t.Skip("sheet not found:", err)
	}
	defer f.Close()
	sheet, err := png.Decode(f)
	if err != nil {
		t.Fatal(err)
	}
	inSheet := map[color.NRGBA]bool{}
	b := sheet.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			inSheet[color.NRGBAModel.Convert(sheet.At(x, y)).(color.NRGBA)] = true
		}
	}
	for name, img := range sprites(sheet) {
		r := img.Bounds()
		if r.Empty() {
			t.Errorf("%s is empty", name)
		}
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				if c := img.NRGBAAt(x, y); c.A > 0 && c != soft && !inSheet[c] {
					t.Errorf("%s pixel %d,%d = %v is not a sheet color", name, x, y, c)
					return
				}
			}
		}
	}
}
