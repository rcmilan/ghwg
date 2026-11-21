package game

// 08 W5-SCENE: the questline's cutscenes (O4, O9 and the finale), each 1 to 3 one-liners, full the first time
// and skipped with SPACE on a replay. Stubs from the W5 seam.

// sceneState: the cutscene playing, if any.
type sceneState struct{}

// playQuestScene starts cutscene id ("O4", "O9", "finale"); it reports evScene with id when it ends.
func (g *Game) playQuestScene(id string) { g.questEvent(evScene, -1, id) }
