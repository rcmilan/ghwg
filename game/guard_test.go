package game

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// U2: the game has no message line. All it has to say to the player goes through g.notice and shows on the HUD
// (hud.go); nothing else may hold or print a notification. This reads the package's sources, so a change that
// brings one back fails here, whoever writes it:
//   - Game has no message field, and nothing reads or writes one;
//   - the notice is touched only in hud.go, through g.notice, the one call any file may make;
//   - drawFrame prints no text of its own: what it draws last is the HUD;
//   - the debug font is printed by printAt alone, and the white-only text() is gone;
//   - the HUD's words stand alone, on no background, and the feed carries events (a Room at -100, a Party fleeing), no system text, neither a
//     confirmation nor a refusal (show, don't tell); why a SPACE did nothing is kept for the View alone.
func TestNoMessageLine(t *testing.T) {
	if runtime.GOOS == "js" {
		t.Skip("reads the source files, which Node on Windows cannot list")
	}
	files, _ := filepath.Glob("*.go")
	var all strings.Builder
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src := string(b)
		all.WriteString(src)
		for _, bad := range []struct{ re, why string }{
			{`\bg\.message\b|^\s*message\s+string`, "a message field: say it with g.notice"},
			{`(?m)^func text\(|\btext\(s(creen)?,`, "text(): print through the HUD's notice, or printAt in a screen of its own"},
		} {
			if m := regexp.MustCompile("(?m)" + bad.re).FindString(src); m != "" {
				t.Errorf("%s: %q: %s", f, strings.TrimSpace(m), bad.why)
			}
		}
		if f != "hud.go" && regexp.MustCompile(`\bg\.note\.`).MatchString(src) {
			t.Errorf("%s touches g.note: only hud.go does; say it with g.notice, read it with g.said", f)
		}
		if f == "hud.go" && strings.Contains(src, "rect(") {
			t.Errorf("hud.go draws a rectangle: the HUD's words stand alone, never on a background")
		}
		if strings.Contains(src, "g.notify(") || strings.Contains(src, "g.said(") {
			t.Errorf("%s: notify and said are gone: the one path is g.notice(text, ink), the HUD's right-side feed", f)
		}
		if f != "view.go" && strings.Contains(src, "g.refused") {
			t.Errorf("%s reads g.refused: it is the View's, set by g.refuse and never drawn", f)
		}
		if f != "draw.go" && strings.Contains(src, "DebugPrint") {
			t.Errorf("%s prints the debug font itself: only printAt does", f)
		}
	}
	src := all.String()
	a := strings.Index(src, "func (g *Game) drawFrame(")
	if a < 0 {
		t.Fatal("no drawFrame")
	}
	body := src[a:]
	body = body[:strings.Index(body, "\n}\n")]
	if m := regexp.MustCompile(`printAt\(|tinted(At)?\(|DebugPrint`).FindString(body); m != "" {
		t.Errorf("drawFrame prints text itself (%s): the HUD is the only place for a notification", m)
	}
	if last := strings.TrimSpace(body[strings.LastIndex(body, "\n"):]); !strings.HasPrefix(last, "g.drawHUD(screen)") {
		t.Errorf("drawFrame ends with %q: nothing is drawn after the HUD", last)
	}
}

// B1-N: the feed holds a notice ~4 s in its ink, shows at most 3 (the oldest leaves early), and empties.
func TestNotice(t *testing.T) {
	g, _ := headless(t, 2, 1, 1)
	g.startRun()
	for _, w := range []string{"ONE", "TWO", "THREE", "FOUR"} {
		g.notice(w)
	}
	live := func() (n int) {
		for _, it := range g.note.items {
			if it.age < noticeFrames {
				n++
			}
		}
		return
	}
	if live() != noticeMax {
		t.Fatalf("%d live after four notices, want %d", live(), noticeMax)
	}
	if g.note.items[0].age < noticeFrames || g.note.items[3].age != 0 {
		t.Fatalf("the oldest should leave early, the newest stay")
	}
	for range slideFrames {
		g.updateHUD()
	}
	if g.note.items[len(g.note.items)-1].in != 1 {
		t.Fatalf("the newest slid in %.2f, want 1", g.note.items[len(g.note.items)-1].in)
	}
	for _, it := range g.note.items {
		if w := len(it.text) * glyphW * hudScale; w > screenW/2 {
			t.Errorf("%q is %d px wide: one line, at most half the screen", it.text, w)
		}
	}
	if g.noticesHidden() {
		t.Fatal("notices hidden in play")
	}
	g.state = StateTerminal
	if !g.noticesHidden() {
		t.Fatal("notices show over the open Terminal")
	}
	g.state = StatePlay
	for range noticeFrames + 2*slideFrames {
		g.updateHUD()
	}
	if len(g.note.items) != 0 {
		t.Fatalf("%d notices left after their hold", len(g.note.items))
	}
}
