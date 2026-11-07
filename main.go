package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"log"
	"os"

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
	flag.Parse()

	ebiten.SetWindowSize(960, 540)
	ebiten.SetWindowTitle("Ghost Hotel")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
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
		run = &shot{Game: g, path: *shotPath, at: *at}
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
