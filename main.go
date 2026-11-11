package main

import (
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"log"
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
	grid := flag.String("grid", "2x1", "dev, with -play: the Room grid, colsxrows")
	react := flag.Int("react", 30, "dev, with -play: frames the Pilot waits before each action")
	keyGap := flag.Int("keygap", 6, "dev, with -play: frames between the Pilot's key taps")
	shotPath := flag.String("shot", "", "dev: start a Run, fast-forward to frame -at, save the screen to this PNG and quit")
	at := flag.Int("at", 0, "dev, with -shot: the frame to capture, from 08:00 on Day 1 (600 frames = 1 game hour)")
	ghost := flag.Bool("ghost", false, "dev, with -shot: start the Run in the ghost dimension")
	shop := flag.String("shop", "", "dev: start a Run on the Terminal's shop holding money,ecto (e.g. 30,40), every unlock but the Mirror locked")
	shadow := flag.Float64("shadow", 0, "dev: start a Run where Room 101 gets a Shadow with this much essence, watched from the Corridor beside it")
	haunt := flag.Int("haunt", 0, "dev: start a Run with this many Shadows wandering the walls forever, to watch them move")
	dirt := flag.Int("dirt", 0, "dev: start a Run with every Room at this much dirt (5 a spot)")
	mood := flag.Int("mood", noMood, "dev: start a Run with every Room at this Room Mood (-100..100)")
	ghosts := flag.Int("ghosts", 0, "dev: start a Run with a human and a ghost of this Phantasm (1..12; 4 and up is strong) settled in Room 101, you inside it")
	skipIntro := flag.Bool("skipintro", false, "dev: skip the opening intro (the sim and the other dev starts never play it)")
	intro := flag.Int("intro", -1, "dev: start at the opening: 0 the title, 1 the three lines, 2 the walk in (with -shot -at)")
	key := flag.Int("key", -1, "dev: start a Run that has ordered the key of this Room (0 is the first), you in the Corridor by its door")
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
	if *play == "" {
		g, err = game.New()
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "seed" && err == nil { // -seed alone: the same hotel again, for prints
				g, err = game.NewSeed(*seed)
			}
		})
	} else {
		var cols, rows int
		if _, err := fmt.Sscanf(*grid, "%dx%d", &cols, &rows); err != nil {
			log.Fatalf("-grid %q: want colsxrows, like 2x1", *grid)
		}
		g, err = game.Play(*play, cols, rows, *seed, *react, *keyGap)
	}
	if err != nil {
		log.Fatal(err)
	}
	var run ebiten.Game = g
	dev := *shotPath != "" || *shop != "" || *shadow > 0 || *haunt > 0 || *dirt > 0 || *mood != noMood || *ghosts > 0 || *intro >= 0 || *key >= 0
	if dev {
		if *intro >= 0 {
			g.StartIntro(*intro)
		} else {
			g.Start(*ghost)
		}
		g.DevKey(*key)
		if *shadow > 0 {
			g.DevShadow(*shadow)
		}
		g.DevHaunt(*haunt)
		g.DevDirt(*dirt)
		g.DevGhosts(*ghosts)
		if *mood != noMood {
			g.DevMood(*mood)
		}
	}
	if *shop != "" {
		var money, ecto float64
		if _, err := fmt.Sscanf(*shop, "%g,%g", &money, &ecto); err != nil {
			log.Fatalf("-shop %q: want money,ecto, like 30,40", *shop)
		}
		g.ShopPrint(money, ecto)
	}
	if *shotPath != "" {
		s := &shot{Game: g, path: *shotPath, at: *at}
		run = s
	}
	if *play == "" && !*skipIntro && !dev { // a person starting from the title: the opening
		g.PlayIntro()
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
