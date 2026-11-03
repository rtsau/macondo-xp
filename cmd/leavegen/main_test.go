package main

import (
	"strings"
	"testing"

	"github.com/domino14/word-golib/tilemapping"
	"github.com/matryer/is"

	"github.com/domino14/macondo/config"
)

func crossplayLD(t *testing.T) *tilemapping.LetterDistribution {
	t.Helper()
	ld, err := tilemapping.GetDistribution(config.DefaultConfig().WGLConfig(), "crossplay")
	is.New(t).NoErr(err)
	return ld
}

func TestForEachSubset(t *testing.T) {
	is := is.New(t)
	var got []string
	forEachSubset([]tilemapping.MachineLetter{0, 0, 1}, 6, func(s []tilemapping.MachineLetter) {
		got = append(got, string(tilemapping.MachineWord(s).ToByteArr()))
	})
	// Each of none, one or two blanks, with or without the A: duplicates count once.
	is.Equal(len(got), 6)

	n := 0
	forEachSubset([]tilemapping.MachineLetter{1, 2, 3, 4, 5, 6, 7}, 6, func(s []tilemapping.MachineLetter) { n++ })
	is.Equal(n, 127) // all 128 subsets but the whole rack
}

func TestAllLeavesRespectsTheTiles(t *testing.T) {
	is := is.New(t)
	// 2 blanks, 1 A, 1 B, leaves up to 2: ?, A, B, ??, ?A, ?B, AB.
	is.Equal(len(allLeaves([]uint8{2, 1, 1}, 2)), 7)

	leaves := allLeaves(crossplayLD(t).Distribution(), maxLeaveLength)
	counts := map[int]int{}
	threeBlanks, fourBlanks := false, false
	zz := false
	for i, l := range leaves {
		counts[len(l)]++
		if i > 0 {
			is.True(len(l) >= len(leaves[i-1])) // shortest first
		}
		blanks := 0
		zs := 0
		for _, t := range l {
			if t == 0 {
				blanks++
			}
			if t == 26 {
				zs++
			}
		}
		threeBlanks = threeBlanks || blanks == 3
		fourBlanks = fourBlanks || blanks == 4
		zz = zz || zs > 1
	}
	is.Equal(counts[1], 27)
	is.True(threeBlanks)
	is.True(!fourBlanks)
	is.True(!zz) // there's one Z
}

const header = "playerID,gameID,turn,rack,play,score,totalscore,tilesplayed,leave,equity,tilesremaining,oppscore\n"

// Two rack kinds, with made-up equities that make the arithmetic easy to check.
func TestComputeLeaves(t *testing.T) {
	is := is.New(t)
	ld := crossplayLD(t)
	logText := header +
		"p1,g,1,?AEINST, 8H FOO,10,10,3,AEI,30.000,50,0\n" +
		"p2,g,2,AAAEIOU, 8H FOO,10,10,3,AEI,10.000,50,0\n" +
		"p1,g,3,AEINRST, 8H FOO,10,10,3,AEI,20.000,50,0\n" +
		// With an empty bag, nothing counts, whatever the equity.
		"p2,g,4,QQQQQQQ, 8H FOO,10,10,3,AEI,1000.000,0,0\n"
	stats, err := readLog(strings.NewReader(logText), ld.TileMapping())
	is.NoErr(err)
	is.Equal(stats.rows, int64(3))

	entries := computeLeaves(stats, ld.Distribution(), options{})
	val := func(s string) float32 {
		mls, err := tilemapping.ToMachineLetters(s, ld.TileMapping())
		is.NoErr(err)
		key := keyOf(mls)
		for _, e := range entries {
			if keyOf(e.Leave) == key {
				return e.Value
			}
		}
		t.Fatalf("no leave %s", s)
		return 0
	}
	// The mean is 20.
	is.Equal(val("?"), float32(10))  // only the 30
	is.Equal(val("A"), float32(0))   // all three
	is.Equal(val("O"), float32(-10)) // only the 10
	is.Equal(val("R"), float32(0))   // only the 20
	is.Equal(val("S"), float32(5))   // the 30 and the 20
	// Never seen. Its parts lean good (? is 10, R is 0), so it takes the stronger.
	is.Equal(val("?R"), float32(10))
	// Its parts disagree (? is 10, O is -10), so it takes their average.
	is.Equal(val("?O"), float32(0))
	// Q was only on the empty-bag rack, so it's as unseen as any: 0.
	is.Equal(val("Q"), float32(0))
}

func TestSmoothingBorrowsFromNeighbors(t *testing.T) {
	is := is.New(t)
	ld := crossplayLD(t)
	// AEINR seen once at 50, and AEINS, one letter away, often at 10.
	logText := header + "p1,g,1,AEINRRR, x,1,1,1,A,50.000,50,0\n"
	for i := 0; i < 99; i++ {
		logText += "p1,g,1,AEINSSS, x,1,1,1,A,10.000,50,0\n"
	}
	stats, err := readLog(strings.NewReader(logText), ld.TileMapping())
	is.NoErr(err)
	mean := float32(stats.total / float64(stats.rows))
	mls, err := tilemapping.ToMachineLetters("AEINR", ld.TileMapping())
	is.NoErr(err)

	find := func(opts options) float32 {
		for _, e := range computeLeaves(stats, ld.Distribution(), opts) {
			if keyOf(e.Leave) == keyOf(mls) {
				return e.Value
			}
		}
		t.Fatal("missing")
		return 0
	}
	is.Equal(find(options{}), 50-mean)
	// Rare enough to smooth, it takes the average of the leaves one letter away. At 50:
	// itself, once for each of its five letters as in the old script, and AEIRR, AENRR,
	// AINRR and EINRR. At 10, for its 99 turns: AEINS. So (9*50 + 99*10) / 108, less
	// the mean.
	want := float32((9*50+99*10)/108.0) - mean
	got := find(options{smooth5: 10})
	is.True(got > want-0.001 && got < want+0.001)
}
