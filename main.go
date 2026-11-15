package main

import (
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
	"math/rand/v2"
	"os"

	"ghosthotel/assets"
	"ghosthotel/game"

	"github.com/hajimehoshi/ebiten/v2"
)

// noMood: the -mood flag is not set.
const noMood = 1000

func main() {
	play := flag.String("play", "", "dev: a Strategy plays instead of you (watch mode), e.g. smart")
	seed := flag.Uint64("seed", 1, "dev: the Run's seed (with -play, or alone for the same hotel in a print)")
	grid := flag.String("grid", "2x1", "dev: the Room grid, colsxrows (e.g. 3x3; with -play too)")
	react := flag.Int("react", 30, "dev, with -play: frames the Pilot waits before each action")
	keyGap := flag.Int("keygap", 6, "dev, with -play: frames between the Pilot's key taps")
	shotPath := flag.String("shot", "", "dev: start a Run, fast-forward to frame -at, save the screen to this PNG and quit")
	at := flag.Int("at", 0, "dev, with -shot: the frame to capture, from 08:00 on Day 1 (600 frames = 1 game hour)")
	ghost := flag.Bool("ghost", false, "dev: start the Run in the ghost dimension")
	shop := flag.String("shop", "", "dev: start a Run on the Terminal's shop holding money,ecto (e.g. 30,40), every unlock but the Mirror locked")
	shadow := flag.Float64("shadow", 0, "dev: start a Run where Room 101 gets a Shadow with this much essence, watched from the Corridor beside it")
	haunt := flag.Int("haunt", 0, "dev: start a Run with this many Shadows wandering the walls forever, to watch them move")
	dirt := flag.Int("dirt", 0, "dev: start a Run with every Room at this much dirt (5 a spot)")
	mood := flag.Int("mood", noMood, "dev: start a Run with every Room at this Room Mood (-100..100)")
	notices := flag.Bool("notices", false, "dev: start a Run with three notices stacked in the right-side feed")
	ghosts := flag.Int("ghosts", 0, "dev: start a Run with a human and a ghost of this Phantasm (1..12; 4 and up is strong) settled in Room 101, you inside it")
	skipIntro := flag.Bool("skipintro", false, "dev: skip the opening intro (the sim and the other dev starts never play it)")
	intro := flag.Int("intro", -1, "dev: start at the opening: 0 the title, 1 the three lines, 2 the walk in (with -shot -at)")
	change := flag.String("change", "", "dev: start a Run with Room 101 changing to this bed layout (single, twin, double), mirrored, and Room 102's change waiting on its guests; you in the Corridor (with -shot -at; -term 2,1,1 shows Room 102's actions)")
	term := flag.String("term", "", "dev, with -change or -shop: open the Terminal on tab,row (from 0), a third number 1 pressing SPACE on it (e.g. 1,0: the HOTEL tab's first row); see -view for a Room's actions")
	key := flag.Int("key", -1, "dev: start a Run that has ordered the key of this Room (0 is the first), you in the Corridor by its door")
	hotelView := flag.String("view", "", "dev: open the hotel view on this Room (0 is the first) picked; a second number -1 picks none, 1 opens its ring, 2 its Change type's beds, a third focusing that action (clockwise from the top) or bed (e.g. 1,1,2), with -shot -at")
	grow := flag.String("grow", "", "dev: start a Run whose hotel grows by each of these in turn (col, floor), the last one's cinematic playing (e.g. col,floor; with -shot -at: 10 pulled back, 50 sliding in, 120 done)")
	staff := flag.String("staff", "", "dev: start a Run with a full stock, housekeepers in the kitchen and Money in the wallet, you beside them: n,money (e.g. 1,500; with less Money than their wages, as 1,0, they are owed them already)")
	ready := flag.Int("prestige-ready", -1, "dev: a full level-1 hotel, every key bought, the Terminal on Father P's row; 1 with his confirm card open")
	busy := flag.Bool("busy", false, "dev: start a stress Run: a full 3x3 hotel, Parties and ghosts arriving minutes apart, three housekeepers, a courier restocking")
	match := flag.Int("match", -1, "dev: start a Run with a bad pair in Room 101 and the other way round in 102, you in 101: 0 as it is (-view 0: the hotel view's tints), 1 the talk open, 2 the swap taken, both under way (with -shot -at)")
	scale := flag.Float64("scale", 1, "dev: the world's time scale, e.g. 0.25 for slow motion (holding T still runs it x4)")
	father := flag.String("father", "", "dev: start a Run with Rooms 101 and 102 open and guests in, Money for both Fathers, Father k (or m) already walking to Room 101, you in the Lobby Archway: SPACE, then Room 102's ring has no call for him until he walks out")
	kitchen := flag.Bool("kitchen", false, "dev: start a Run in the kitchen, in front of the oven, the Water full, the Food low, the Towels out (with -shot -at, -ghost)")
	mirror := flag.Bool("mirror", false, "dev: Room 101 has a Room mirror and two ghosts (the second came through it), Room 102 none: its ring offers Add mirror (-view 1,1,0); you in 101; use with -ghost")
	corridors := flag.Int("corridors", -1, "dev, with -grow floor: stand beside the first Corridor Mirror's cell; this many (0, 1) are uncovered already; the Concierge has the Money and Ectoplasm to buy the next one at the Terminal")
	prestige := flag.Bool("prestige", false, "dev: start a busy Run a few hours in with Father P's prestige cutscene playing (with -shot -at: 300 gathering, 400 he appears, 600 lines, 850 flip, 1100 race, 1300 white)")
	flag.Parse()

	ebiten.SetWindowSize(960, 540)
	ebiten.SetWindowTitle("Ghost Hotel")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if f, err := assets.FS.Open("characters/Adam_idle_anim_16x16.png"); err == nil { // the window's icon: a Shadow Person's black head (D169), as icon.ico
		if sheet, err := png.Decode(f); err == nil {
			icon := image.NewNRGBA(image.Rect(0, 0, 16, 16))
			draw.DrawMask(icon, image.Rect(0, 1, 16, 14), image.Black, image.Point{}, sheet, image.Pt(18*16, 9), draw.Src)
			ebiten.SetWindowIcon([]image.Image{icon})
		}
	}
	var g *game.Game
	var err error
	var cols, rows int
	if _, err := fmt.Sscanf(*grid, "%dx%d", &cols, &rows); err != nil {
		log.Fatalf("-grid %q: want colsxrows, like 2x1", *grid)
	}
	if *play == "" {
		seeded := false
		flag.Visit(func(f *flag.Flag) { seeded = seeded || f.Name == "seed" })
		if !seeded { // -seed alone: the same hotel again, for prints
			*seed = rand.Uint64()
		}
		g, err = game.NewGrid(cols, rows, *seed)
	} else {
		g, err = game.Play(*play, cols, rows, *seed, *react, *keyGap)
	}
	if err != nil {
		log.Fatal(err)
	}
	g.DevScale(*scale)
	var run ebiten.Game = g
	// Dev starts combine (08 F0): one Run, started once, each start adding to it. Where two set the same thing
	// (the Concierge's spot, the wallet, every Room's Mood), the later one here wins.
	dev := *shotPath != "" || *shop != "" || *shadow > 0 || *haunt > 0 || *dirt > 0 || *mood != noMood || *ghosts > 0 || *intro >= 0 || *key >= 0 ||
		*change != "" || *kitchen || *grow != "" || *staff != "" || *mirror || *corridors >= 0 || *busy || *ready >= 0 || *match >= 0 || *ghost || *hotelView != "" || *term != "" || *father != "" || *notices || *prestige
	if dev {
		switch {
		case *intro >= 0:
			g.StartIntro(*intro)
		case *busy || *prestige: // it starts its own Run: first, so nothing after it is lost
			g.DevBusy()
		case *ready >= 0:
			g.Start(false)
		default:
			g.Start(false)
		}
		g.DevGhost(*ghost)
		if *prestige {
			g.DevPrestige()
		}
		// The Room's Mood is one value, set by -dirt and -mood: the later of the two wins it. -mood also stands the
		// Concierge in Room 101, so both go first: any start after them that places him takes him from there.
		g.DevDirt(*dirt)
		if *mood != noMood {
			g.DevMood(*mood)
		}
		g.DevKey(*key)
		g.DevChange(*change)
		if *staff != "" {
			n, money := 1, 0.0
			fmt.Sscanf(*staff, "%d,%g", &n, &money)
			g.DevStaff(n, money)
		}
		if *kitchen {
			g.DevKitchen()
		}
		if *shadow > 0 {
			g.DevShadow(*shadow)
		}
		g.DevHaunt(*haunt)
		g.DevGhosts(*ghosts)
		if *mirror {
			g.DevMirror()
		}
		if *father != "" {
			g.DevFather(*father == "m")
		}
		if *notices {
			g.DevNotices()
		}
		g.DevGrow(*grow)
		if *corridors >= 0 { // after the growth that makes them
			g.DevCorridors(*corridors)
		}
		g.DevMatch(*match) // after DevGrow, which puts the game back in play: the talk stays open
	}
	if *ready >= 0 {
		g.DevPrestigeReady(*ready == 1)
	}
	if *shop != "" {
		var money, ecto float64
		if _, err := fmt.Sscanf(*shop, "%g,%g", &money, &ecto); err != nil {
			log.Fatalf("-shop %q: want money,ecto, like 30,40", *shop)
		}
		g.ShopPrint(money, ecto)
	}
	if *hotelView != "" {
		var room, open, pick int
		fmt.Sscanf(*hotelView, "%d,%d,%d", &room, &open, &pick)
		g.DevView(room, open, pick)
	}
	if *term != "" {
		var tab, row, place int
		fmt.Sscanf(*term, "%d,%d,%d", &tab, &row, &place)
		g.DevTerm(tab, row, place == 1)
	}
	if *shotPath != "" {
		s := &shot{Game: g, path: *shotPath, at: *at}
		run = s
	}
	if *play == "" && !*skipIntro && !dev { // a person starting from the title: the opening
		g.PlayIntro()
	}
	if *shotPath == "" { // a print runs thousands of Updates a tick on purpose
		w := &guard{Game: run}
		go w.watch()
		run = w
	}
	if err := ebiten.RunGame(run); err != nil {
		log.Fatal(err)
	}
}

// shot runs the game unthrottled up to frame at, then saves one screen to path and quits: a dev print.
type shot struct {
	*game.Game
	path      string
	at, frame int
	saved     bool
}

func (s *shot) Update() error {
	if s.saved {
		return ebiten.Termination
	}
	for n := 0; s.frame < s.at && n < 2000; n++ { // ponytail: 2000 Updates per tick keeps the window responsive
		if err := s.Game.Update(); err != nil {
			return err
		}
		s.frame++
	}
	return nil
}

func (s *shot) Draw(screen *ebiten.Image) {
	s.Game.Draw(screen)
	if s.frame < s.at || s.saved {
		return
	}
	img := image.NewRGBA(screen.Bounds())
	screen.ReadPixels(img.Pix)
	f, err := os.Create(s.path)
	if err == nil {
		err = png.Encode(f, img)
		f.Close()
	}
	if err != nil {
		log.Fatal(err)
	}
	s.saved = true
}
