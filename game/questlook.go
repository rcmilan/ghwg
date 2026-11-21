package game

import (
	"image"
	"slices"

	"ghosthotel/entities"
	"ghosthotel/world"

	"github.com/hajimehoshi/ebiten/v2"
)

// 08 W5 R2: every quest character wears sheets of its own (tools/questlooks): a LimeZu base sheet in colors of
// its own, under one accessory. They are no one's base: not guestCast's nor shadowCast's, so never a random
// walk-in, passer-by, ghost or Shadow, and a named guest's sheets take no random look.

// questLooks: each quest character's sheets, by its cast Key.
var questLooks = map[string]string{"magenta": nameMagenta, "celestine": nameCelestine, "magda": nameMagda, "tim": nameTim, "teo": nameTeo, "osvaldo": nameOsvaldo}

// questBases: the sheets each look was made from, worn in its place if its own are missing.
var questBases = map[string]string{nameMagenta: "Amelia", nameCelestine: "Amelia", nameMagda: "Amelia", nameTim: "Alex", nameTeo: "Alex", nameOsvaldo: "Adam"}

// questHues: a quest ghost's flat color in the ghost dimension, in place of its kind's (ghostLook): the palette
// shows only as its two tones there, so its hue is its own too: Madame rose, Tim violet, Osvaldo pale blue.
var questHues = map[string][3]float32{"celestine": {1, .42, .62}, "tim": {.68, .46, 1}, "osvaldo": {.6, .8, 1}}

func init() {
	for name, base := range questBases {
		ghostTints[name] = ghostTints[base]
	}
}

// questSheets: the quest characters' sheets as pixels, by name; one whose files are missing wears its base's.
func questSheets(bases map[string][2]*image.RGBA) map[string][2]*image.RGBA {
	out := map[string][2]*image.RGBA{}
	for name, base := range questBases {
		out[name] = bases[base]
		if own, err := loadCharacters(name); err == nil {
			out[name] = own[name]
		}
	}
	return out
}

// loadWitch loads the quest characters' sheets, as drawn: no look ever recolors them.
func (g *Game) loadWitch() error {
	for name, sheets := range questSheets(g.bases) {
		g.bases[name] = sheets
		g.characters[name] = dress(sheets, nil, look{})
	}
	return nil
}

// wear: a guest just named puts on its quest look, if its Key has one.
func wear(guest *entities.Guest) {
	if name, ok := questLooks[guest.Who.Key]; ok {
		guest.Character = name
	}
}

// questPairs: the ordinary ramps' hair and top that together would read as a quest character: Magenta's red
// over green or violet, Magda's grey over blue, Madame's and Tim's grey over rose or violet, Teo's brown over
// green. randomLook rolls again on one, so an ordinary guest never wears them.
var questPairs = [...][2]uint32{{0xb04a2a, 0x3ca05a}, {0xb04a2a, 0x8a4ab4}, {0xa8a8b0, 0x3c6ec8}, {0xa8a8b0, 0xe070a0}, {0xa8a8b0, 0x8a4ab4}, {0x5a3a28, 0x3ca05a}}

func questPair(l look) bool { return slices.Contains(questPairs[:], [2]uint32{l[hair], l[top]}) }

// fearless: Magenta is never scared: a Shadow gives her no Fear (and so no Ecto), no hop and no line, a ghost
// beyond her Nerve does not terrify her, and she never steps out nor flees.
func fearless(guest *entities.Guest) bool { return guest.Who.Key == "magenta" }

// The dev line-up (-witch): the quest characters side by side in the Lobby, then the three ordinary sheets to
// tell them from, all turning together (down, right, up, left), a turn standing then a turn
// running in place; in ghost, as that Dimension draws them (a human a white silhouette, a quest ghost in its
// hue), with the Cat's frames on the Street: walking right and up and the nap, then walking left and down.
const (
	lineupGap  = 17  // world px between two of them
	lineupTurn = 120 // frames a facing is held (prints: -at 60 down, 180 right, 300 up, 420 left, then running)
)

var lineupOrder = [...]string{"magenta", "magda", "teo", "celestine", "tim", "osvaldo"}

// lineup adds the line-up's actors to as.
func (g *Game) lineup(as []actor) []actor {
	at := g.street.lineup
	if at == (image.Point{}) {
		return as
	}
	turn := g.tick / lineupTurn
	anim := entities.Posed([...]entities.Facing{entities.FaceDown, entities.FaceRight, entities.FaceUp, entities.FaceLeft}[turn%4], turn/4%2 == 1, float64(g.tick))
	for i := range len(lineupOrder) + len(guestCast) {
		a := actor{x: float64(at.X + i*lineupGap), y: float64(at.Y), anim: anim}
		var hue [3]float32
		ghost := false
		if i < len(lineupOrder) {
			a.name = questLooks[lineupOrder[i]]
			hue, ghost = questHues[lineupOrder[i]]
		} else {
			a.x += lineupGap / 2
			a.name = guestCast[i-len(lineupOrder)]
		}
		switch {
		case g.ghost && ghost:
			a.tint, a.a, a.mode, a.up = hue[:], ghostAlpha, modeFlat, true
		case g.ghost:
			a.tint, a.a, a.mode, a.sil = silhouetteTint, silhouetteAlpha, modeFlat, true
		}
		as = append(as, a)
	}
	if !g.ghost { // the Cat is the ghost dimension's
		return as
	}
	for i := range 18 { // the sheet's frames in two rows: its rows 0, 1 and the nap's two, then its rows 2 and 3
		row, f := i/4, i%4
		x, y := at.X+(i%8+row/4*8)*lineupGap, g.hotel.Spawn.Y-world.TileSize/2+row/2%2*(catFrame+4)
		as = append(as, actor{x: float64(x), y: float64(y), draw: func(s *ebiten.Image, x, y int) { g.drawCatFrame(s, x, y, row, f) }})
	}
	return as
}

// DevWitch is main's -witch: the line-up in the Lobby, the Concierge out on the Street.
func (g *Game) DevWitch() {
	l := g.hotel.Lobby
	at := image.Pt(l.Min.X+3*world.TileSize-2, l.Max.Y-world.TileSize)
	g.street.lineup = at
	g.player.X, g.player.Y = float64(at.X+8*lineupGap), float64(g.hotel.Spawn.Y+world.TileSize) // on the Street, past the Cat's last frame
	w, h := g.mapSize()
	g.cam.Snap(g.player.X, g.player.Y, w, h)
}
