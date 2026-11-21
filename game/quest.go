package game

import "github.com/hajimehoshi/ebiten/v2"

// 08 W5: Morwenna's circle, the questline's state (W5-SEAM). The data is circle.go. W5-QUEST fills the stubs below;
// every verb (verbs.go) reports what it did through questEvent, the one call the engine listens to.

// questRun: this Run's questline, in the dawn checkpoint (runState.Quest).
type questRun struct {
	Beat  int            // the beat under way, 1..beatCount; 0 before the first Growth
	Done  []bool         // per objective of Beat, in beats[Beat-1].Objs order
	Rolls map[string]int // this Run's rolled parts, by name ("room", "porter", ...): the engine's to name
}

// questRecord: the questline across every Run, saved with the Level and the Bonus (saveData.Quest).
type questRecord struct {
	Times     [beatCount]int  // times each beat was done, ever
	Items     [itemCount]bool // the items owned (given the first time ever a beat is done)
	MacGuffin bool            // the finale was played: the MacGuffin is summoned, once ever
}

// questEv is a quest-worthy thing a verb did.
type questEv int

const (
	evGrow    questEv = iota // a Growth landed (beginGrow): a beat starts
	evDawn                   // 08:00 (the checkpoint): overnight objectives check
	evTell                   // TELL on a Card: room, the guest's key
	evGive                   // GIVE on a Card (the Concierge gives): room, key
	evTake                   // TAKE on a Card (the Concierge takes): room, key
	evReveal                 // a REVEAL: room, key
	evMove                   // a talk MOVE landed: room is the new Room, key
	evServe                  // a ghost Request delivered: room, key
	evDeliver                // a quest row delivered (the wreath, the crate): room (-1: the Mirror), key the row's
	evLit                    // the spirit caught in the Lantern beam: room -1, key ""
	evDoor                   // SPACE on the spirit's handprint door in ghost: room
	evRitual                 // a Priest's Ritual landed: room, key "k" or "m"
	evBook                   // a quest entry read in the guest book: room -1, key the entry's
	evScene                  // a cutscene ended: room -1, key its id
)

// questEvent: a verb did something quest-worthy (room -1 when none). W5-QUEST ticks objectives from it.
func (g *Game) questEvent(ev questEv, room int, key string) {}

// questStart: a Run starts (startRun): the questline back to before beat 1, rolled again.
func (g *Game) questStart() { g.quest = questRun{} }

// questStage: how far this Run's questline has got: the beats done this Run (0..beatCount).
func (g *Game) questStage() int { return 0 }

// questPinned: the Party is a quest guest whose objective is not done: no checkout, flee, passing on or Father M.
func (g *Game) questPinned(p *Party) bool { return false }

// questGate: why the next Growth must wait ("SHIFT'S NOT OVER"), "" if it needn't.
func (g *Game) questGate() string { return "" }

// questArrivals runs each game minute, before the walk-ins: the beat's quest guests check in ahead of them.
func (g *Game) questArrivals() {}

// questHUD draws the checklist: real rows on the left in real, ghost rows on the right in ghost.
func (g *Game) questHUD(dst *ebiten.Image) {}

// itemOn: the item is owned (for good, across prestiges).
func (g *Game) itemOn(id itemID) bool { return id >= 0 && id < itemCount && g.questRec.Items[id] }

// timesDone: how many times beat b (1..beatCount) has been done, ever.
func (g *Game) timesDone(b int) int { return g.questRec.Times[b-1] }

// byTimes: the line for the n-th time (0: the 1st), the last non-empty one before it if that one is empty.
func byTimes(lines [3]string, n int) string {
	for i := min(n, 2); i >= 0; i-- {
		if lines[i] != "" {
			return lines[i]
		}
	}
	return ""
}
