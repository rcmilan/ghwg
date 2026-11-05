package main

import (
	"flag"
	"fmt"
	"log"

	"ghosthotel/game"

	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	play := flag.String("play", "", "dev: a Strategy plays instead of you (watch mode), e.g. bestrate")
	seed := flag.Uint64("seed", 1, "dev, with -play: the Run's seed")
	grid := flag.String("grid", "3x3", "dev, with -play: the Room grid, colsxrows")
	react := flag.Int("react", 30, "dev, with -play: frames the Pilot waits before each action")
	keyGap := flag.Int("keygap", 6, "dev, with -play: frames between the Pilot's key taps")
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
			log.Fatalf("-grid %q: want colsxrows, like 3x3", *grid)
		}
		g, err = game.Play(*play, cols, rows, *seed, *react, *keyGap)
	}
	if err != nil {
		log.Fatal(err)
	}
	if err := ebiten.RunGame(g); err != nil {
		log.Fatal(err)
	}
}
