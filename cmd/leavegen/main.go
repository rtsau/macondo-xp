// leavegen turns the per-turn log of an autoplay run into leave values, in the KLV2
// format that bots load. A leave's value is how much better than average the turns
// were whose racks contained it. Since a turn's equity includes the leave values the
// bots were playing with, repeating this from the new values refines them.
//
// This is notebooks/superleaves/autogenerate_superleaves.py in Go, for any tile set.
//
// Usage:
//
//	go run ./cmd/leavegen -log run.txt -ld crossplay -out candidate.klv2
package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"

	"github.com/domino14/word-golib/tilemapping"
	"github.com/rs/zerolog"

	"github.com/domino14/macondo/config"
	"github.com/domino14/macondo/equity"
)

const maxLeaveLength = 6

// The columns of the per-turn log that this reads.
const (
	colRack   = 3
	colEquity = 9
	colBag    = 10
)

// leaveKey packs a sorted leave of up to six tiles into a number. Tiles are stored
// plus one, so that a leave of blanks differs from a shorter one.
type leaveKey uint64

func keyOf(tiles []tilemapping.MachineLetter) leaveKey {
	var k leaveKey
	for _, t := range tiles {
		k = k<<6 | leaveKey(t+1)
	}
	return k
}

type tally struct {
	count  int64
	equity float64
}

// rackStats totals the equity of the turns on each rack that had tiles left to draw.
type rackStats struct {
	racks map[string]*tally
	rows  int64
	total float64
}

func readLog(r io.Reader, tm *tilemapping.TileMapping) (*rackStats, error) {
	cr := csv.NewReader(bufio.NewReaderSize(r, 1<<20))
	cr.FieldsPerRecord = -1
	cr.ReuseRecord = true
	if _, err := cr.Read(); err != nil {
		return nil, fmt.Errorf("reading header: %w", err)
	}
	stats := &rackStats{racks: map[string]*tally{}}
	for {
		row, err := cr.Read()
		if err == io.EOF {
			return stats, nil
		}
		if err != nil {
			return nil, err
		}
		// With an empty bag, no tiles are coming and a leave means nothing.
		bag, err := strconv.Atoi(row[colBag])
		if err != nil {
			return nil, fmt.Errorf("bad bag count in %v: %w", row, err)
		}
		if bag < 1 {
			continue
		}
		eq, err := strconv.ParseFloat(row[colEquity], 64)
		if err != nil {
			return nil, fmt.Errorf("bad equity in %v: %w", row, err)
		}
		mls, err := tilemapping.ToMachineLetters(row[colRack], tm)
		if err != nil {
			return nil, fmt.Errorf("bad rack in %v: %w", row, err)
		}
		slices.Sort(mls)
		key := string(tilemapping.MachineWord(mls).ToByteArr())
		t, ok := stats.racks[key]
		if !ok {
			t = &tally{}
			stats.racks[key] = t
		}
		t.count++
		t.equity += eq
		stats.rows++
		stats.total += eq
	}
}

// forEachSubset calls f with every distinct sub-multiset of the sorted tiles, up to
// maxLen long, including the empty one.
func forEachSubset(tiles []tilemapping.MachineLetter, maxLen int, f func([]tilemapping.MachineLetter)) {
	buf := make([]tilemapping.MachineLetter, 0, maxLen)
	var rec func(i int)
	rec = func(i int) {
		if i == len(tiles) {
			f(buf)
			return
		}
		next := i
		for next < len(tiles) && tiles[next] == tiles[i] {
			next++
		}
		start := len(buf)
		rec(next)
		for c := i; c < next && len(buf) < maxLen; c++ {
			buf = append(buf, tiles[i])
			rec(next)
		}
		buf = buf[:start]
	}
	rec(0)
}

// allLeaves lists every leave a player could hold under a tile distribution, shortest
// first.
func allLeaves(dist []uint8, maxLen int) [][]tilemapping.MachineLetter {
	byLen := make([][][]tilemapping.MachineLetter, maxLen+1)
	buf := make([]tilemapping.MachineLetter, 0, maxLen)
	var rec func(ml int)
	rec = func(ml int) {
		if ml == len(dist) {
			if n := len(buf); n > 0 {
				byLen[n] = append(byLen[n], slices.Clone(buf))
			}
			return
		}
		start := len(buf)
		rec(ml + 1)
		for c := 0; c < int(dist[ml]) && len(buf) < maxLen; c++ {
			buf = append(buf, tilemapping.MachineLetter(ml))
			rec(ml + 1)
		}
		buf = buf[:start]
	}
	rec(0)
	var out [][]tilemapping.MachineLetter
	for _, ls := range byLen {
		out = append(out, ls...)
	}
	return out
}

// options are the thresholds below which a long leave is too rare to trust alone, so
// it takes its value from similar leaves. The defaults are the old script's, for runs
// of 200,000 games.
type options struct {
	smooth5, smooth6 int64
}

// computeLeaves gives each leave a value from the log's totals.
func computeLeaves(stats *rackStats, dist []uint8, opts options) []equity.LeaveEntry {
	seen := map[leaveKey]*tally{}
	for rack, t := range stats.racks {
		tiles := make([]tilemapping.MachineLetter, len(rack))
		for i := range rack {
			tiles[i] = tilemapping.MachineLetter(rack[i])
		}
		forEachSubset(tiles, maxLeaveLength, func(sub []tilemapping.MachineLetter) {
			if len(sub) == 0 {
				return
			}
			k := keyOf(sub)
			s, ok := seen[k]
			if !ok {
				s = &tally{}
				seen[k] = s
			}
			s.count += t.count
			s.equity += t.equity
		})
	}
	mean := stats.total / float64(stats.rows)

	leaves := allLeaves(dist, maxLeaveLength)
	values := make(map[leaveKey]float64, len(leaves))
	// Shortest first, so that a leave never seen can lean on its shorter parts.
	for _, l := range leaves {
		k := keyOf(l)
		if s, ok := seen[k]; ok && s.count > 0 {
			values[k] = s.equity/float64(s.count) - mean
			continue
		}
		values[k] = fromSubleaves(l, values)
	}

	entries := make([]equity.LeaveEntry, len(leaves))
	for i, l := range leaves {
		v := values[keyOf(l)]
		var count int64
		if s, ok := seen[keyOf(l)]; ok {
			count = s.count
		}
		if (len(l) == 5 && count < opts.smooth5) || (len(l) == 6 && count < opts.smooth6) {
			if nv, ok := fromNeighbors(l, seen, dist); ok {
				v = nv - mean
			}
		}
		entries[i] = equity.LeaveEntry{Leave: tilemapping.MachineWord(l), Value: float32(v)}
	}
	return entries
}

// fromSubleaves values a leave never seen from those with one tile fewer: the
// strongest if they mostly agree that the leave is good, the weakest if they mostly
// agree that it's bad, or else their average.
func fromSubleaves(l []tilemapping.MachineLetter, values map[leaveKey]float64) float64 {
	if len(l) == 1 {
		return 0
	}
	sub := make([]tilemapping.MachineLetter, 0, len(l)-1)
	signs, total := 0, 0.0
	best, worst := -1e9, 1e9
	for i := range l {
		sub = append(append(sub[:0], l[:i]...), l[i+1:]...)
		v := values[keyOf(sub)]
		total += v
		best, worst = max(best, v), min(worst, v)
		if v > 0 {
			signs++
		} else if v < 0 {
			signs--
		}
	}
	switch {
	case signs > 0:
		return best
	case signs < 0:
		return worst
	}
	return total / float64(len(l))
}

// fromNeighbors is the average equity of the turns with a leave one letter away. It
// never changes a blank, whose value is too large to borrow from or lend.
func fromNeighbors(l []tilemapping.MachineLetter, seen map[leaveKey]*tally, dist []uint8) (float64, bool) {
	var count int64
	var eq float64
	n := make([]tilemapping.MachineLetter, len(l))
	for i, t := range l {
		if t == 0 {
			continue
		}
		for ml := 1; ml < len(dist); ml++ {
			copy(n, l)
			n[i] = tilemapping.MachineLetter(ml)
			slices.Sort(n)
			if s, ok := seen[keyOf(n)]; ok {
				count += s.count
				eq += s.equity
			}
		}
	}
	if count == 0 {
		return 0, false
	}
	return eq / float64(count), true
}

func run() error {
	logPath := flag.String("log", "", "autoplay's per-turn log (required)")
	ldName := flag.String("ld", "crossplay", "letter distribution")
	out := flag.String("out", "", "the .klv2 file to write (required)")
	smooth5 := flag.Int64("smooth5", 828, "5-tile leaves seen less than this borrow from similar leaves")
	smooth6 := flag.Int64("smooth6", 234, "6-tile leaves seen less than this borrow from similar leaves")
	flag.Parse()
	if *logPath == "" || *out == "" {
		flag.Usage()
		os.Exit(2)
	}
	cfg := config.DefaultConfig()
	if p := os.Getenv("MACONDO_DATA_PATH"); p != "" {
		cfg.Set(config.ConfigDataPath, p)
	}
	ld, err := tilemapping.GetDistribution(cfg.WGLConfig(), *ldName)
	if err != nil {
		return err
	}
	f, err := os.Open(*logPath)
	if err != nil {
		return err
	}
	defer f.Close()
	stats, err := readLog(f, ld.TileMapping())
	if err != nil {
		return err
	}
	if stats.rows == 0 {
		return fmt.Errorf("no turns with tiles in the bag in %s", *logPath)
	}
	entries := computeLeaves(stats, ld.Distribution(), options{*smooth5, *smooth6})

	o, err := os.Create(*out)
	if err != nil {
		return err
	}
	defer o.Close()
	if err := equity.WriteKLV(o, entries); err != nil {
		return err
	}
	fmt.Printf("%d turns, mean equity %.2f, %d leaves written to %s\n",
		stats.rows, stats.total/float64(stats.rows), len(entries), *out)
	for _, e := range entries {
		if len(e.Leave) == 1 {
			fmt.Printf(" %s %+.1f", e.Leave.UserVisible(ld.TileMapping()), e.Value)
		}
	}
	fmt.Println()
	return nil
}

func main() {
	zerolog.SetGlobalLevel(zerolog.WarnLevel)
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "FAILED:", err)
		os.Exit(1)
	}
}
