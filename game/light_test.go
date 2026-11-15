package game

import (
	"image"
	"math"
	"slices"
	"testing"

	"ghosthotel/world"
)

// D94, T037, T169: the tint is clear at noon, tinted at dawn and dusk, deep blue at night, and never
// jumps between two game minutes, midnight included.
func TestTint(t *testing.T) {
	at := func(h, m int) [4]float64 { return tint(h*hourFrames + m*minuteFrames) }
	if c := at(12, 0); c[3] != 0 {
		t.Errorf("noon: %v, want clear", c)
	}
	if c := at(2, 0); c[3] < 0.6 || c[2] <= c[0] {
		t.Errorf("02:00: %v, want deep blue", c)
	}
	for _, h := range []int{6, 19} {
		if c := at(h, 0); c[3] == 0 || c[3] >= at(2, 0)[3] {
			t.Errorf("%02d:00: %v, want a tint lighter than the night", h, c)
		}
	}
	for c := range dayFrames + minuteFrames { // one step past midnight: the wrap is continuous too
		a, b := tint(c), tint(c+minuteFrames)
		for i := range a {
			if math.Abs(a[i]-b[i]) > 0.02 {
				t.Fatalf("frame %d: the tint jumps from %v to %v in a game minute", c, a, b)
			}
		}
	}
}

// T170: the entrance hall is lit through the evening, fades off just before 01:00 and stays dark until
// daylight.
func TestLobbyLit(t *testing.T) {
	at := func(h, m int) float64 { return lobbyLit(h*hourFrames + m*minuteFrames) }
	for _, hm := range [][2]int{{12, 0}, {18, 30}, {22, 0}, {0, 0}, {0, 45}} {
		if l := at(hm[0], hm[1]); l != 1 {
			t.Errorf("%02d:%02d: the Lobby is %.0f%% lit, want on", hm[0], hm[1], l*100)
		}
	}
	for _, hm := range [][2]int{{1, 0}, {1, 30}, {3, 0}, {5, 30}, {6, 30}} {
		if l := at(hm[0], hm[1]); l != 0 {
			t.Errorf("%02d:%02d: the Lobby is %.0f%% lit, want dark", hm[0], hm[1], l*100)
		}
	}
	if l := at(0, 52); l <= 0 || l >= 1 {
		t.Errorf("00:52: the Lobby is %.0f%% lit, want mid-fade", l*100)
	}
	for c := range dayFrames {
		if math.Abs(lobbyLit(c)-lobbyLit(c+minuteFrames)) > 0.1 {
			t.Fatalf("frame %d: the Lobby light jumps", c)
		}
	}
}

// T171: the corridor lamps hang over the Corridors, and at full night they leave a dim pool: lighter than
// the night, darker than the lit Lobby.
func TestCorridorLamps(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	if len(g.hotel.Lights) == 0 {
		t.Fatal("no corridor lamps")
	}
	for _, p := range g.hotel.Lights {
		if p.In(g.hotel.Lobby) {
			t.Errorf("lamp %v is in the Lobby", p)
		}
		for _, r := range g.hotel.Rooms {
			if p.In(r.Shell()) {
				t.Errorf("lamp %v is in Room %s", p, r.Number)
			}
		}
	}
	night := tint(2 * hourFrames)[3]
	if d := night * (1 - lampCut); d < 0.2 || d > 0.5 {
		t.Errorf("a lamp's center keeps %.2f of the darkness, want dim but readable (0.2..0.5)", d)
	}
}

// T172: hall light spills through the Archway into the Corridor, stopped by the walls, and only while the
// Lobby is lit.
func TestSpill(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	h := g.hotel
	mask := spillMask(h)
	at := func(cx, cy int) float64 { // the spill at cell (cx,cy)'s center, 0..1
		p := image.Pt(cx*world.TileSize+world.TileSize/2, cy*world.TileSize+world.TileSize/2).Div(spillRes)
		return float64(mask.AlphaAt(p.X, p.Y).A) / 255
	}
	ax, hall := h.Archway.X/world.TileSize, h.Archway.Y/world.TileSize-1
	r0 := h.Rooms[0].Origin
	behind := image.Pt(r0.X+world.RoomW-2, r0.Y+world.RoomH-3) // Room 101, just inside its south wall, nearest the Archway
	if s := at(ax, hall-1); s < 0.6 {
		t.Errorf("the Corridor cell above the Archway gets %.2f spill, want bright", s)
	}
	if s := at(ax, behind.Y-1); s == 0 { // farther from the Archway than behind, but in line with it
		t.Errorf("the column Corridor cell (%d,%d) gets no spill", ax, behind.Y-1)
	}
	if s := at(behind.X, behind.Y); s != 0 {
		t.Errorf("cell %v, behind Room 101's wall, gets %.2f spill, want none", behind, s)
	}
	if s := at(2, hall-1); s != 0 {
		t.Errorf("the Corridor's far west end, behind the Hall Wall, gets %.2f spill, want none", s)
	}
	for _, hm := range [][2]int{{1, 30}, {3, 0}, {5, 30}} {
		if l := spillLevel(hm[0]*hourFrames + hm[1]*minuteFrames); l != 0 {
			t.Errorf("%02d:%02d: spill at %.2f with the Lobby off, want 0", hm[0], hm[1], l)
		}
	}
	if spillLevel(22*hourFrames) != 1 {
		t.Error("22:00: no spill with the Lobby lit")
	}
}

// F0 W1 leftovers: a lantern's beam stops at the first wall tile, never showing through into a neighbouring Room.
func TestLanternStopsAtWall(t *testing.T) {
	g, _ := headless(t, 2, 2, 1)
	r := g.hotel.Rooms[0]
	const ts = world.TileSize
	at := image.Pt((r.Origin.X+6)*ts+ts/2, (r.Origin.Y+world.RoomH)*ts+ts/2) // the corridor cell below the front wall
	for _, reach := range g.beamReach(at, -math.Pi/2) {                      // facing up, into the wall
		if reach > ts {
			t.Fatalf("a ray facing the wall reaches %.0f px, want at most a tile", reach)
		}
	}
	if reach := g.beamReach(at, 0); slices.Max(reach) != lanternR { // along the corridor: its middle ray runs free
		t.Fatalf("down the open corridor the longest ray reaches %.0f px, want %d", slices.Max(reach), lanternR)
	}
}
