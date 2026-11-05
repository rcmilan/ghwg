// Command sim plays many headless Runs in parallel, each by a Strategy through the keys a player has,
// and prints how each Strategy did (simulation mode). A dev tool: never part of the game binary.
//
//	go run ./cmd/sim -strategy bestrate,idle -runs 100 -days 7 -grid 3x3
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

	"shift/game"
)

func main() {
	strategies := flag.String("strategy", "bestrate,idle", "comma-separated Strategies to compare")
	runs := flag.Int("runs", 100, "Runs per Strategy; Run i uses seed+i, the same for every Strategy")
	days := flag.Int("days", 7, "days per Run, 24 h each from the Run start")
	grid := flag.String("grid", "3x3", "the Room grid, colsxrows, 2x1 to 3x3")
	seed := flag.Uint64("seed", 1, "the first Run's seed")
	react := flag.Int("react", 30, "frames the Pilot waits before each action")
	keyGap := flag.Int("keygap", 6, "frames between the Pilot's key taps")
	noCheck := flag.Bool("nocheck", false, "skip the invariant checks")
	csvPath := flag.String("csv", "", "write one row per Run to this file")
	flag.Parse()
	debug.SetGCPercent(400) // ponytail: Runs allocate per frame and the GC is what stops them scaling across cores

	var cols, rows int
	if _, err := fmt.Sscanf(*grid, "%dx%d", &cols, &rows); err != nil {
		log.Fatalf("-grid %q: want colsxrows, like 3x3", *grid)
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
					Name: name, Strategy: game.Strategies[name], React: *react, KeyGap: *keyGap, NoCheck: *noCheck})
			}()
		}
	}
	wg.Wait()

	fmt.Printf("%s, %d days, %d Runs per Strategy (seeds %d..%d), react %d, keygap %d\n\n",
		*grid, *days, *runs, *seed, *seed+uint64(*runs)-1, *react, *keyGap)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(w, "strategy\t$/day mean\tmin\tmax\toccupancy\tlost\trefused\tbookings\tfailures\t")
	var failures []string
	for s, name := range names {
		var perDay []float64
		var sold, avail, lost, refused, bookings, failed int
		cheat := false
		for _, r := range results[s] {
			total := 0
			for _, m := range r.MoneyByDay {
				total += m
			}
			perDay = append(perDay, float64(total)/float64(*days))
			sold, avail, lost, refused, bookings = sold+r.Sold, avail+r.Available, lost+r.Lost, refused+r.Refused, bookings+r.Bookings
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
		fmt.Fprintf(w, "%s\t%.1f\t%.0f\t%.0f\t%.1f%%\t%.1f\t%.1f\t%.1f\t%d\t\n", name, mean(perDay), slices.Min(perDay), slices.Max(perDay),
			100*float64(sold)/float64(max(avail, 1)), float64(lost)/n, float64(refused)/n, float64(bookings)/n, failed)
	}
	w.Flush()
	if len(failures) > 0 {
		fmt.Println("\nfailures:")
		fmt.Println(strings.Join(failures, "\n"))
	}
	if *csvPath != "" {
		if err := writeCSV(*csvPath, results); err != nil {
			log.Fatal(err)
		}
	}
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
	w.Write([]string{"strategy", "seed", "cheat", "money_by_day", "sold", "available", "lost", "refused", "bookings", "paid", "pairs", "outings", "failure"})
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
				strconv.Itoa(r.Sold), strconv.Itoa(r.Available), strconv.Itoa(r.Lost), strconv.Itoa(r.Refused),
				strconv.Itoa(r.Bookings), strconv.Itoa(r.Paid), strconv.Itoa(r.Pairs), strconv.Itoa(r.Outings), failure})
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
