// Command sim plays many headless Runs in parallel, each by a Strategy through the keys a player has,
// and prints how each Strategy did (simulation mode). A dev tool: never part of the game binary.
//
//	go run ./cmd/sim -strategy never,server,keeper,smart,shopper -runs 64 -days 7
//	go run ./cmd/sim -strategy goals -days 4 -runs 16   (the pacing: when a Run reaches each milestone, and its longest waits)
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"text/tabwriter"

	"ghosthotel/game"
)

func main() {
	strategies := flag.String("strategy", "never", "comma-separated Strategies to compare")
	runs := flag.Int("runs", 100, "Runs per Strategy; Run i uses seed+i, the same for every Strategy")
	days := flag.Int("days", 7, "days per Run, 24 h each from the Run start")
	grid := flag.String("grid", "2x1", "the Room grid, colsxrows, 2x1 to 3x3")
	seed := flag.Uint64("seed", 1, "the first Run's seed")
	react := flag.Int("react", 30, "frames the Pilot waits before each action")
	keyGap := flag.Int("keygap", 6, "frames between the Pilot's key taps")
	open := flag.Bool("open", false, "start each Run with every Room owned (a normal Run starts with them boarded up and Money for one key)")
	mirror := flag.Bool("mirror", false, "start each Run with the Mirror unlocked, so ghosts book and haunt (a normal Run starts with it locked: no ghosts)")
	beds := flag.String("beds", "", "start each Run with every Room of this bed layout: Single, Twin or Double (a normal Run starts all Single; Twin and Double bring pairs)")
	noCheck := flag.Bool("nocheck", false, "skip the invariant checks")
	csvPath := flag.String("csv", "", "write one row per Run to this file")
	level := flag.Int("level", 0, "prestige level: the grid's limit is (3+level) a side (08 P1)")
	bonus := flag.Float64("bonus", 1, "the income bonus a prestige left (08 P1)")
	ectoRate := flag.Float64("ectorate", 0.5, "$ one Ectoplasm is worth in the score column: an assumed shop price (T112)")
	noUp := flag.Bool("noup", false, "the Strategies buy no upgrade levels (T374: for a before/after)")
	flag.Parse()
	game.NoUpgrades = *noUp
	debug.SetGCPercent(400) // ponytail: Runs allocate per frame and the GC is what stops them scaling across cores

	var cols, rows int
	if _, err := fmt.Sscanf(*grid, "%dx%d", &cols, &rows); err != nil {
		log.Fatalf("-grid %q: want colsxrows, like 2x1", *grid)
	}
	names := strings.Split(*strategies, ",")
	for _, n := range names {
		if game.Strategies[n] == nil {
			log.Fatalf("unknown Strategy %q", n)
		}
	}

	results := make([][]game.Result, len(names))
	var wg sync.WaitGroup
	sem := make(chan struct{}, runtime.NumCPU())
	for s, name := range names {
		results[s] = make([]game.Result, *runs)
		for i := range *runs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()
				results[s][i] = game.RunSim(game.Config{Cols: cols, Rows: rows, Days: *days, Seed: *seed + uint64(i),
					Name: name, Strategy: game.Strategies[name], React: *react, KeyGap: *keyGap, NoCheck: *noCheck, Mirror: *mirror, Open: *open, Beds: *beds, Level: *level, Bonus: *bonus})
			}()
		}
	}
	wg.Wait()

	fmt.Printf("%s, %d days, %d Runs per Strategy (seeds %d..%d), react %d, keygap %d, score = $ + Ectoplasm × %g\n\n",
		*grid, *days, *runs, *seed, *seed+uint64(*runs)-1, *react, *keyGap, *ectoRate)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "strategy\tscore/day\t$/day mean\tmin\tmax\toccupancy\tbookings\tghosts\tfled\twalked\tcleans\tectoplasm\tserves\texpired\tpriests\tspent\tM calls\tretainer days\tpriest ecto\tclosed h\tmirror at (h)\troom mirror at (h)\tfailures\t")
	var failures, growth, staff, pacing, prestige, ups []string
	for s, name := range names {
		var perDay []float64
		var sold, avail, bookings, ghosts, fled, walked, cleans, ecto, serves, expired, priests, spent, closed, failed, priestsM, retainDays, priestEcto int
		cheat := false
		mirrorSum, mirrorN := 0.0, 0
		var roomMirror []float64
		var marks [5][]float64 // real minutes: the Mirror, the first Growth, the first housekeeper, the in-Room Mirror, the first Deluxe Room
		var waits [3]float64
		earned := 0
		var grown [][]float64 // per expansion, first to last: the game hour of each Run that made it (H1)
		rooms := 0
		hired, quit, staffSpend := 0, 0, 0
		upLevels, upSpend, wallet := 0, 0, make([]int, *days)
		var ready, gains []float64
		longWait := 0.0
		for _, r := range results[s] {
			hired, quit, staffSpend = hired+r.Hired, quit+r.Quit, staffSpend+r.StaffSpend
			upLevels, upSpend = upLevels+r.UpLevels, upSpend+r.UpSpend
			for d, m := range r.WalletByDay {
				wallet[d] += m
			}
			if longWait = max(longWait, r.LongWait); r.PrestigeAt >= 0 {
				ready, gains = append(ready, r.PrestigeAt/6), append(gains, r.Gain)
			}
			first := -1.0
			if len(r.Grown) > 0 {
				first = r.Grown[0]
			}
			for k, h := range []float64{r.MirrorAt, first, r.HiredAt, r.RoomMirror, r.DeluxeAt} {
				if h >= 0 {
					marks[k] = append(marks[k], h/6) // a game hour is 10 real seconds
				}
			}
			for k, w := range r.Waits {
				waits[k] = max(waits[k], w)
			}
			earned += r.EctoEarned
			for k, h := range r.Grown {
				if k == len(grown) {
					grown = append(grown, nil)
				}
				grown[k] = append(grown[k], h)
			}
			rooms += r.Rooms
			if r.RoomMirror >= 0 {
				roomMirror = append(roomMirror, r.RoomMirror)
			}
			if r.MirrorAt >= 0 {
				mirrorSum, mirrorN = mirrorSum+r.MirrorAt, mirrorN+1
			}
			total := 0
			for _, m := range r.MoneyByDay {
				total += m
			}
			perDay = append(perDay, float64(total)/float64(*days))
			sold, avail, bookings = sold+r.Sold, avail+r.Available, bookings+r.Bookings
			ghosts += r.Ghosts
			fled, ecto, serves, expired = fled+r.Fled, ecto+r.Ecto, serves+r.Serves, expired+r.Expired
			walked, cleans = walked+r.Walked, cleans+r.Cleans
			priests, spent, closed = priests+r.Priests, spent+r.PriestSpend, closed+r.ClosedHours
			priestsM, retainDays, priestEcto = priestsM+r.PriestsM, retainDays+r.RetainDays, priestEcto+r.PriestEcto
			cheat = cheat || r.Cheat
			if len(r.Failures) > 0 {
				failed++
				f := r.Failures[0]
				failures = append(failures, fmt.Sprintf("  %s seed %d  %s  %s\n    replay: go run . -play %s -seed %d -grid %s -react %d -keygap %d",
					name, r.Seed, f.Clock, f.Reason, name, r.Seed, *grid, *react, *keyGap))
			}
		}
		if cheat {
			name += " (cheat)"
		}
		n := float64(*runs)
		mirrorAt := "-" // mean over the Runs that got the Mirror; "-" if none did
		if mirrorN > 0 {
			mirrorAt = fmt.Sprintf("%.1f", mirrorSum/float64(mirrorN))
			if mirrorN < *runs {
				mirrorAt += fmt.Sprintf(" (%d/%d)", mirrorN, *runs)
			}
		}
		roomMirrorAt := spread(roomMirror, *runs) // the first in-Room Mirror (K1's pacing goal)
		if len(marks[1])+len(marks[2])+len(marks[3])+len(marks[4]) > 0 {
			pacing = append(pacing, fmt.Sprintf("  %s: Mirror %s; Growth %s; housekeeper %s; Room mirror %s; Deluxe %s\n    longest wait: %.0f s before the Growth, %.0f s before the housekeeper, %.0f s before the Room mirror; %.0f Ectoplasm earned a Run",
				name, spread(marks[0], *runs), spread(marks[1], *runs), spread(marks[2], *runs), spread(marks[3], *runs), spread(marks[4], *runs), waits[0], waits[1], waits[2], float64(earned)/n))
		}
		fmt.Fprintf(w, "%s\t%.1f\t%.1f\t%.0f\t%.0f\t%.1f%%\t%.1f\t%.1f\t%.1f\t%.1f\t%.1f\t%.0f\t%.1f\t%.1f\t%.1f\t%.0f\t%.1f\t%.1f\t%.0f\t%.0f\t%s\t%s\t%d\t\n", name,
			mean(perDay)+float64(ecto)/n**ectoRate/float64(*days), mean(perDay), slices.Min(perDay), slices.Max(perDay),
			100*float64(sold)/float64(max(avail, 1)), float64(bookings)/n, float64(ghosts)/n, float64(fled)/n, float64(walked)/n, float64(cleans)/n, float64(ecto)/n,
			float64(serves)/n, float64(expired)/n, float64(priests)/n, float64(spent)/n, float64(priestsM)/n, float64(retainDays)/n, float64(priestEcto)/n, float64(closed)/n, mirrorAt, roomMirrorAt, failed)
		g := "-"
		if len(gains) > 0 {
			g = fmt.Sprintf("%.2f", mean(gains))
		}
		prestige = append(prestige, fmt.Sprintf("  %s: Father P ready at %s real min; gain %s; longest idle %.0f s", name, spread(ready, *runs), g, longWait))
		line := fmt.Sprintf("  %s: %.1f levels a Run for $%.0f; unspent $ at each day's end:", name, float64(upLevels)/n, float64(upSpend)/n)
		for _, m := range wallet {
			line += fmt.Sprintf(" %.0f", float64(m)/n)
		}
		ups = append(ups, line)
		if hired > 0 {
			staff = append(staff, fmt.Sprintf("  %s hired %.1f a Run, %d walked out unpaid in %d Runs, $%.0f a day on fees and wages", name, float64(hired)/n, quit, *runs, float64(staffSpend)/n/float64(*days)))
		}
		if len(grown) > 0 {
			line := fmt.Sprintf("  %s grew, at game hour (mean, Runs):", name)
			for _, hs := range grown {
				line += fmt.Sprintf(" %.1f (%d/%d)", mean(hs), len(hs), *runs)
			}
			growth = append(growth, fmt.Sprintf("%s; %.1f Rooms at the end", line, float64(rooms)/n))
		}
	}
	w.Flush()
	if len(growth) > 0 {
		fmt.Println("\ngrowth:")
		fmt.Println(strings.Join(growth, "\n"))
	}
	if len(pacing) > 0 {
		fmt.Println("\npacing, real minutes from the Run start (median, min..max; the goal: Growth 4, Room mirror 6, housekeeper 7, Deluxe 9.5; waits 30, 60, 90 s):")
		fmt.Println(strings.Join(pacing, "\n"))
	}
	fmt.Printf("\nprestige (level %d, bonus x%.2f; the goal: ~30 real min to Father P):\n%s\n", *level, max(1, *bonus), strings.Join(prestige, "\n"))
	if len(staff) > 0 {
		fmt.Println("\nstaff:")
		fmt.Println(strings.Join(staff, "\n"))
	}
	if len(failures) > 0 {
		fmt.Println("\nfailures:")
		fmt.Println(strings.Join(failures, "\n"))
	}
	fmt.Println("\nupgrades:\n" + strings.Join(ups, "\n"))
	if *csvPath != "" {
		if err := writeCSV(*csvPath, results); err != nil {
			log.Fatal(err)
		}
	}
}

// spread: "median, min..max" of xs, with how many of the Runs it covers when not all; "-" for none.
func spread(xs []float64, runs int) string {
	if slices.Sort(xs); len(xs) == 0 {
		return "-"
	}
	s := fmt.Sprintf("%.1f, %.1f..%.1f", xs[len(xs)/2], xs[0], xs[len(xs)-1])
	if len(xs) < runs {
		s += fmt.Sprintf(" (%d/%d)", len(xs), runs)
	}
	return s
}

func mean(xs []float64) float64 {
	t := 0.0
	for _, x := range xs {
		t += x
	}
	return t / float64(len(xs))
}

// writeCSV writes one row per Run with every Result field; money by day is ;-separated.
func writeCSV(path string, results [][]game.Result) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	w := csv.NewWriter(f)
	w.Write([]string{"strategy", "seed", "cheat", "money_by_day", "sold", "available", "bookings", "ghosts", "paid", "pairs", "outings", "fled", "ectoplasm", "serves", "expired", "priests", "priest_spend", "m_calls", "retainer_days", "priest_ecto", "closed_hours", "mirror_at_h", "room_mirror_at_h", "failure"})
	for _, rs := range results {
		for _, r := range rs {
			var money []string
			for _, m := range r.MoneyByDay {
				money = append(money, strconv.Itoa(m))
			}
			failure := ""
			if len(r.Failures) > 0 {
				failure = r.Failures[0].Clock + " " + r.Failures[0].Reason
			}
			w.Write([]string{r.Strategy, strconv.FormatUint(r.Seed, 10), strconv.FormatBool(r.Cheat), strings.Join(money, ";"),
				strconv.Itoa(r.Sold), strconv.Itoa(r.Available),
				strconv.Itoa(r.Bookings), strconv.Itoa(r.Ghosts), strconv.Itoa(r.Paid), strconv.Itoa(r.Pairs), strconv.Itoa(r.Outings),
				strconv.Itoa(r.Fled), strconv.Itoa(r.Ecto), strconv.Itoa(r.Serves), strconv.Itoa(r.Expired),
				strconv.Itoa(r.Priests), strconv.Itoa(r.PriestSpend), strconv.Itoa(r.PriestsM), strconv.Itoa(r.RetainDays), strconv.Itoa(r.PriestEcto), strconv.Itoa(r.ClosedHours), strconv.FormatFloat(r.MirrorAt, 'f', 1, 64), strconv.FormatFloat(r.RoomMirror, 'f', 1, 64), failure})
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
