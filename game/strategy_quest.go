package game

func init() { Strategies["quester"] = quester{} }

// questDelay: game hours a beat's objectives take the quester, one after the other, from the beat's start.
const questDelay = 3

// quester plays the Run as goals does and completes the questline's objectives too (08 W5).
// ponytail: VERBS is not merged on this branch, so each objective is ticked with questEvent after questDelay hours
// apiece; once VERBS lands, this becomes real play through the Pilot (a Cheat row until then).
type quester struct{ goals }

// Cheat ticks the beat's next open objective once questDelay hours of the beat have passed for each one before it.
func (quester) Cheat(g *Game) {
	q := g.quest
	if q.Beat < 1 {
		return
	}
	for i, o := range beats[q.Beat-1].Objs {
		if !q.Done[i] && g.clock-q.At >= (i+1)*questDelay*hourFrames {
			g.questEvent(evDone, -1, o.ID)
			return
		}
	}
}
