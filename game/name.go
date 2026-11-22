package game

import "strings"

// Name is the game's name: the title, the window, the web page's tab. Change it here only.
const Name = "WEIRD GUESTS"

// spacedName: the title's look, a space between letters and three between words ("W E I R D   G U E S T S").
func spacedName() string {
	w := strings.Fields(Name)
	for i, v := range w {
		w[i] = strings.Join(strings.Split(v, ""), " ")
	}
	return strings.Join(w, "   ")
}
