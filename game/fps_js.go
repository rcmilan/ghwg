//go:build js

package game

import (
	"fmt"
	"image/color"
	"net/url"
	"strings"
	"syscall/js"

	"github.com/hajimehoshi/ebiten/v2"
)

// fpsOn: the readout shows unless the page URL has ?nofps: on for the live tests of the web build (T381 refine 7).
// ponytail: on by default, flip to opt-in (?fps) before the itch release.
var fpsOn = func() bool {
	loc := js.Global().Get("location")
	if !Dev || loc.IsUndefined() { // not in the production build; the Node test runner has no page
		return false
	}
	js.Global().Get("document").Set("title", Name)
	q, _ := url.ParseQuery(strings.TrimPrefix(loc.Get("search").String(), "?"))
	return !q.Has("nofps")
}()

func drawFPS(screen *ebiten.Image) {
	if fpsOn {
		printAt(screen, fmt.Sprintf("%.0f FPS  %.0f TPS", ebiten.ActualFPS(), ebiten.ActualTPS()), 2, 2, 1, color.White, 1)
	}
}
