package game

import (
	"slices"
	"strings"
	"testing"
)

// T120, D166: every opaque color of every character sheet has a region; a recolor moves only the pixels
// of the regions it sets, and keeps their shades in order.
func TestRecolor(t *testing.T) {
	sheets, err := loadCharacters(append([]string{"Bob", witchName}, guestCast[:]...)...)
	if err != nil {
		t.Fatal(err)
	}
	key := func(r, g, b uint8) uint32 { return uint32(r)<<16 | uint32(g)<<8 | uint32(b) }
	for name, pair := range sheets {
		pal := palettes[name]
		for _, sheet := range pair {
			for i := 0; i < len(sheet.Pix); i += 4 {
				if p := sheet.Pix[i : i+4]; p[3] > 0 {
					if _, ok := pal[key(p[0], p[1], p[2])]; !ok {
						t.Fatalf("%s: #%06x has no region", name, key(p[0], p[1], p[2]))
					}
				}
			}
		}
		src := pair[0]
		out := recolor(src, pal, look{hair: 0xd8b45c})
		type shade struct{ was, now float64 }
		var shades []shade
		for y := range src.Bounds().Dy() {
			for x := range src.Bounds().Dx() {
				a, b := src.RGBAAt(x, y), out.RGBAAt(x, y)
				r := pal[key(a.R, a.G, a.B)][min(1, y/bodyRow)]
				if a.A == 0 || r != hair {
					if a != b {
						t.Fatalf("%s: (%d,%d), not hair, changed %v → %v", name, x, y, a, b)
					}
					continue
				}
				_, _, was := hsl(key(a.R, a.G, a.B))
				_, _, now := hsl(key(b.R, b.G, b.B))
				shades = append(shades, shade{was, now})
			}
		}
		slices.SortFunc(shades, func(a, b shade) int { return int((a.was - b.was) * 1e6) })
		for i := 1; i < len(shades); i++ {
			if shades[i].now < shades[i-1].now-1.0/255 {
				t.Fatalf("%s: hair shades out of order after recolor: %v then %v", name, shades[i-1], shades[i])
			}
		}
		if len(shades) == 0 {
			t.Fatalf("%s: no hair", name)
		}
	}
}

// T122, D166: each Guest comes in a look of its own (the sim table, unchanged, shows the sim's randomness
// is untouched).
func TestGuestLooks(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	seen := map[string]bool{}
	for range 6 {
		p := g.walkIn()
		if p == nil {
			break
		}
		for _, guest := range p.Guests {
			base, seed, ok := strings.Cut(guest.Character, "#")
			if !ok || !slices.Contains(guestCast[:], base) || seed == "" || seen[guest.Character] {
				t.Fatalf("a Guest named %q, want a base character, # and a fresh look", guest.Character)
			}
			seen[guest.Character] = true
		}
		g.parties = nil
	}
	if len(seen) < 6 {
		t.Fatalf("%d Guests walked in, want at least 6", len(seen))
	}
}
