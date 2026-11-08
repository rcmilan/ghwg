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

func main() {
	play := flag.String("play", "", "dev: a Strategy plays instead of you (watch mode), e.g. haunter")
	seed := flag.Uint64("seed", 1, "dev, with -play: the Run's seed")
	grid := flag.String("grid", "2x1", "dev, with -play: the Room grid, colsxrows")
	react := flag.Int("react", 30, "dev, with -play: frames the Pilot waits before each action")
	keyGap := flag.Int("keygap", 6, "dev, with -play: frames between the Pilot's key taps")
	shotPath := flag.String("shot", "", "dev: start a Run, fast-forward to frame -at, save the screen to this PNG and quit")
	at := flag.Int("at", 0, "dev, with -shot: the frame to capture, from 08:00 on Day 1 (600 frames = 1 game hour)")
	ghost := flag.Bool("ghost", false, "dev, with -shot: start the Run in the ghost dimension")
	hexAt := flag.String("hex", "", "dev, with -shot: 8 frames before -at, the Concierge stands on map cell x,y and hexes the Room beside it")
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
	if *shotPath != "" {
		g.Start(*ghost)
		s := &shot{Game: g, path: *shotPath, at: *at}
		if *hexAt != "" {
			s.hex = new(image.Point)
			if _, err := fmt.Sscanf(*hexAt, "%d,%d", &s.hex.X, &s.hex.Y); err != nil {
				log.Fatalf("-hex %q: want x,y", *hexAt)
			}
		}
		run = s
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
	hex       *image.Point // -hex
}

func (s *shot) Update() error {
	if s.saved {
		return ebiten.Termination
	}
	for n := 0; s.frame < s.at && n < 2000; n++ { // ponytail: 2000 Updates per tick keeps the window responsive
		if s.hex != nil && s.frame == s.at-8 {
			s.Game.HexFrom(*s.hex)
		}
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
