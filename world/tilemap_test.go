package world

import (
	"image"
	"testing"
)

func TestBlockedBox(t *testing.T) {
	ts := loadTileset(t)
	m := NewTilemap(ts, 3, 1)
	set := func(x int, name string) {
		id, ok := ts.ID(name)
		if !ok {
			t.Fatalf("no tile %q", name)
		}
		m.Set(LayerDecor, x, 0, id)
	}
	set(0, "counter_mid") // rect (0,8,16,8): front strip
	set(1, "border_west") // rect (0,0,6,16)
	set(2, "border_east") // rect (10,0,6,16)
	// ~10x6 foot box
	foot := func(x, y int) image.Rectangle { return image.Rect(x, y, x+10, y+6) }

	cases := []struct {
		name string
		box  image.Rectangle
		want bool
	}{
		{"behind the counter (top half)", foot(3, 2), false},
		{"on the counter front strip", foot(3, 5), true},
		{"just right of the west side line", foot(16+6, 4), false},
		{"1px into the west side line", foot(16+5, 4), true},
		{"just left of the east side line", foot(32, 4), false},
		{"1px into the east side line", foot(32+1, 4), true},
		{"outside the map", foot(-1, 2), true},
	}
	for _, c := range cases {
		if got := m.BlockedBox(c.box); got != c.want {
			t.Errorf("%s %v: blocked=%v, want %v", c.name, c.box, got, c.want)
		}
	}
}
