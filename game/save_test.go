package game

import (
	"errors"
	"reflect"
	"testing"

	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// memStore swaps the byte store for a variable; the Game under test never touches a real file or localStorage.
func memStore(t *testing.T) (data *[]byte, reads, writes *int) {
	t.Helper()
	var b []byte
	r, w := 0, 0
	oldR, oldW := readSave, writeSave
	readSave = func() ([]byte, error) { r++; return b, nil }
	writeSave = func(p []byte) error { w++; b = p; return nil }
	t.Cleanup(func() { readSave, writeSave = oldR, oldW })
	return &b, &r, &w
}

// dressedRun: a grown, dirtied, staffed Run with its Rooms bought, a Mirror, a retained Father K, a deal on the way.
func dressedRun(t *testing.T, seed uint64) *Game {
	t.Helper()
	g, _ := headless(t, 2, 1, seed)
	g.startRun()
	g.persist = true
	if _, err := g.grow(true); err != nil {
		t.Fatal(err)
	}
	if _, err := g.grow(false); err != nil {
		t.Fatal(err)
	}
	g.hotel.SetCorridorMirrors(g.corridorMirrors(g.hotel, g.hotel))
	g.hotel.Corridors[0].Open = true
	twin, _ := world.TypeIn(g.types, "Standard", "Twin")
	g.openRoom(0)
	g.openRoom(1)
	g.hotel.Land(1, twin, true)
	g.DevDirt(10)
	g.mood[0], g.mood[3] = -20, 40
	g.addDirtLevel(1, 7)
	g.taps[1][TaskServe] = 12
	g.clock = 2*dayFrames + runStart
	g.wallet, g.earned, g.runEcto = [2]Num{123, 45}, [2]Num{900, 80}, 33
	g.own(Mirror)
	g.own(shoeIDs[0])
	g.see()
	g.stock = map[Unlock]int{SupplyWater: 4}
	g.hire()
	g.hire()
	g.keepers[1].unpaid = true
	g.keepers[1].due = g.clock + 100
	g.retainK()
	g.priests[0].due = g.clock + 700
	return g
}

func TestSaveRoundTrip(t *testing.T) {
	g1 := dressedRun(t, 1)
	g1.level, g1.bonus = 2, 1.5
	data, _, writes := memStore(t)
	g1.saveDawn()
	if *writes != 1 {
		t.Fatalf("writes %d", *writes)
	}
	s := g1.loadSave()
	if s == nil || s.Level != 2 || s.Bonus != 1.5 || s.Run == nil {
		t.Fatalf("loaded %+v from %s", s, *data)
	}
	g2, _ := headless(t, 2, 1, 99)
	g2.startRun()
	g2.persist = true
	if !g2.resume(s) {
		t.Fatal("the checkpoint did not apply")
	}
	if g2.state != StatePlay || g2.intro != nil || g2.clock != 2*dayFrames+runStart || g2.level != 2 || g2.bonus != 1.5 {
		t.Errorf("state %d clock %d level %d bonus %g", g2.state, g2.clock, g2.level, g2.bonus)
	}
	if len(g2.parties) != 0 {
		t.Error("guests in flight")
	}
	a, b := g1.checkpoint(), g2.checkpoint()
	a.Seed, b.Seed = 0, 0
	if !reflect.DeepEqual(a, b) {
		t.Errorf("the restored Run differs:\n got %+v\nwant %+v", b, a)
	}
	if c, r := g2.hotel.Grid(); c != 3 || r != 2 || len(g2.keepers) != 2 || !g2.keepers[1].unpaid || !g2.fatherIn(false) {
		t.Errorf("grid %dx%d keepers %d", c, r, len(g2.keepers))
	}
	for range 3 * tps { // and it runs
		g2.step(0, 0)
	}
}

func TestSaveTolerantLoad(t *testing.T) {
	g := dressedRun(t, 3)
	data, _, _ := memStore(t)
	*data = []byte(`{"level":3,"extra":{"x":1},"run":{"Cols":2,"Rows":1,"Rooms":[{"Tier":"Standard","Beds":"Single","Future":9},{}],"New":true}}`)
	s := g.loadSave()
	if s == nil || s.Level != 3 || s.Bonus != 0 || s.Run == nil || len(s.Run.Rooms) != 2 {
		t.Fatalf("unknown or missing fields: %+v", s)
	}
	g2, _ := headless(t, 2, 1, 4)
	g2.startRun()
	g2.persist = true
	if !g2.resume(s) || g2.bonus != 1 || g2.state != StatePlay || g2.clock != runStart {
		t.Errorf("defaults: bonus %g clock %d", g2.bonus, g2.clock)
	}
	if g2.locked == nil || !g2.locked[Mirror] { // a missing Locked keeps a fresh Run's
		t.Error("missing Locked should keep the start's")
	}
	for _, bad := range []string{`not json`, `{"level":`, `[1,2]`, `{"level":"x"}`, "\x00\xff\x01"} {
		g3, _ := headless(t, 2, 1, 5)
		g3.startRun()
		g3.persist = true
		*data = []byte(bad)
		if g3.loadSave() != nil || len(g3.note.items) != 1 {
			t.Errorf("%q: want no save and a notice, got %d notices", bad, len(g3.note.items))
		}
	}
	// a Run that does not fit the hotel: fresh Run, notice, no panic
	g4, _ := headless(t, 2, 1, 6)
	g4.startRun()
	g4.persist = true
	if g4.resume(&saveData{Level: 1, Run: &runState{Cols: 9, Rows: 9}}) || len(g4.note.items) != 1 || g4.state != StatePlay {
		t.Error("a Run that does not fit should start fresh with a notice")
	}
	// storage that fails
	readSave = func() ([]byte, error) { return nil, errors.New("blocked") }
	g5, _ := headless(t, 2, 1, 7)
	g5.persist = true
	if g5.loadSave() != nil || len(g5.note.items) != 1 {
		t.Error("an unreadable store should give a notice")
	}
}

func TestSavePrestigeHasNoRun(t *testing.T) {
	g := dressedRun(t, 8)
	g.level, g.bonus = 1, 1.25
	data, _, _ := memStore(t)
	g.saveGame()
	s, err := decodeSave(*data)
	if err != nil || s.Level != 1 || s.Bonus != 1.25 || s.Run != nil {
		t.Fatalf("%+v %v", s, err)
	}
}

func TestSaveNotPersistNeverTouchesStore(t *testing.T) {
	_, reads, writes := memStore(t)
	g := dressedRun(t, 9)
	g.persist = false
	g.saveDawn()
	g.saveGame()
	if g.loadSave() != nil {
		t.Error("loaded")
	}
	for range dayFrames + 100 { // a whole dawn passes
		g.advance(0, 0, 1)
	}
	if *reads != 0 || *writes != 0 {
		t.Errorf("reads %d writes %d", *reads, *writes)
	}
}

func TestSaveDawnWrites(t *testing.T) {
	g := dressedRun(t, 10)
	_, _, writes := memStore(t)
	g.clock = 3*dayFrames + runStart - 5
	for range 10 {
		g.advance(0, 0, 1)
	}
	if *writes != 1 {
		t.Errorf("a dawn writes once, got %d", *writes)
	}
}

// the title: with a save SPACE continues into the checkpoint; without, the opening plays.
func TestSaveTitleContinue(t *testing.T) {
	src := dressedRun(t, 11)
	data, _, _ := memStore(t)
	src.saveDawn()
	g, s := headless(t, 2, 1, 12)
	g.persist = true
	g.PlayIntro()
	if g.intro == nil || g.intro.save == nil {
		t.Fatal("the title should hold the save")
	}
	s.hold(ebiten.KeySpace)
	g.Update()
	if g.intro != nil || g.state != StatePlay || g.clock != 2*dayFrames+runStart {
		t.Errorf("CONTINUE: intro %v state %d clock %d", g.intro, g.state, g.clock)
	}
	*data = nil
	g2, s2 := headless(t, 2, 1, 13)
	g2.persist = true
	g2.PlayIntro()
	s2.hold(ebiten.KeySpace)
	g2.Update()
	if g2.intro == nil || g2.intro.phase != introText {
		t.Error("no save: the opening plays as before")
	}
}

// A resumed Run with the pantry bought stands its shelves again (W3 merge: the growths ran before the locks).
func TestSaveResumesPantry(t *testing.T) {
	g1 := dressedRun(t, 1)
	g1.level = 1
	g1.boughtPantry()
	if len(g1.hotel.Pantries) == 0 {
		t.Fatal("no pantry on the grown hotel")
	}
	g1.pantryStock[0][SupplyFood] = 2
	g1.pantryStock[len(g1.pantryStock)-1][SupplyTowel] = 1
	memStore(t)
	g1.saveDawn()
	g2, _ := headless(t, 2, 1, 99)
	g2.startRun()
	g2.persist = true
	if !g2.resume(g2.loadSave()) || len(g2.hotel.Pantries) != len(g1.hotel.Pantries) || len(g2.pantryStock) != len(g2.hotel.Pantries) {
		t.Fatalf("pantries %d, stocks %d; want %d", len(g2.hotel.Pantries), len(g2.pantryStock), len(g1.hotel.Pantries))
	}
	if !reflect.DeepEqual(g2.pantryStock, g1.pantryStock) {
		t.Errorf("pantry stock %v after the round trip, want %v", g2.pantryStock, g1.pantryStock)
	}
	// an old save, with no PantryStock: empty shelves
	s := g1.loadSave()
	s.Run.PantryStock = nil
	g3, _ := headless(t, 2, 1, 99)
	g3.startRun()
	if !g3.resume(s) || len(g3.pantryStock) != len(g3.hotel.Pantries) || g3.pantryStock[0][SupplyFood] != 0 {
		t.Errorf("an old save should load empty pantries: %v", g3.pantryStock)
	}
}
