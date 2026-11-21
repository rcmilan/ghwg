package game

import (
	"math/rand/v2"
	"slices"
	"testing"
)

// 08 W5: Magenta's sheets are Amelia's under a hat. Under any random look the hat keeps its pixels and
// the rest dresses as Amelia would; she has her own base and is never a random walk-in.
func TestWitchKeepsHat(t *testing.T) {
	sheets, err := loadCharacters("Amelia", witchName)
	if err != nil {
		t.Fatal(err)
	}
	for seed := range uint64(20) {
		l := randomLook(rand.New(rand.NewPCG(seed, 0)))
		for i, base := range sheets["Amelia"] {
			src := sheets[witchName][i]
			got, plain := recolor(src, palettes[witchName], l), recolor(base, palettes["Amelia"], l)
			hat := 0
			for y := range src.Bounds().Dy() {
				for x := range src.Bounds().Dx() {
					want := plain.RGBAAt(x, y)
					if src.RGBAAt(x, y) != base.RGBAAt(x, y) { // a hat pixel
						want = src.RGBAAt(x, y)
						hat++
					}
					if got.RGBAAt(x, y) != want {
						t.Fatalf("seed %d sheet %d (%d,%d) = %v, want %v", seed, i, x, y, got.RGBAAt(x, y), want)
					}
				}
			}
			if hat < 24*40 {
				t.Fatalf("sheet %d: %d hat pixels, want a hat on all 24 frames", i, hat)
			}
		}
	}
	if slices.Contains(guestCast[:], witchName) {
		t.Fatal("Magenta is in the random walk-ins' cast")
	}
	h := namedHumans[slices.IndexFunc(namedHumans, func(h namedHuman) bool { return h.Key == "magenta" })]
	if h.Members[0].Sprite != witchName {
		t.Fatalf("Magenta's sprite is %q, want %q", h.Members[0].Sprite, witchName)
	}
}
