package movegen

import (
	"testing"

	"github.com/domino14/word-golib/tilemapping"
	"github.com/matryer/is"

	"github.com/domino14/macondo/board"
	"github.com/domino14/macondo/equity"
	"github.com/domino14/macondo/game"
	pb "github.com/domino14/macondo/gen/api/proto/macondo"
	"github.com/domino14/macondo/move"
)

func newScoringGame(t *testing.T, layout, dist string, variant game.Variant) *game.Game {
	t.Helper()
	is := is.New(t)
	rules, err := game.NewBasicGameRules(DefaultConfig, "NWL23", layout, dist, game.CrossScoreAndSet, variant)
	is.NoErr(err)
	g, err := game.NewGame(rules, []*pb.PlayerInfo{{Nickname: "a"}, {Nickname: "b"}})
	is.NoErr(err)
	g.StartGame()
	return g
}

// The generator keeps its own running score. It has to agree with the board's
// scoring routine on every play, for every kind of rack and position.
func TestGeneratedScoresMatchBoardScoring(t *testing.T) {
	variants := []struct {
		name    string
		layout  string
		dist    string
		variant game.Variant
		rack    string
		// What the rack's best play is worth on an empty board, if we know.
		bestOpening int
	}{
		// As in game.TestCrossplayScoring.
		{"crossplay", "", "", game.VarCrossplay, "JUKEBOX", 132},
		{"crossplay one blank", "", "", game.VarCrossplay, "JUKEBO?", 0},
		{"crossplay three blanks", "", "", game.VarCrossplay, "???ABCD", 0},
		{"classic", board.CrosswordGameLayout, "english", game.VarClassic, "JUKEBOX", 0},
	}
	for _, tc := range variants {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)
			g := newScoringGame(t, tc.layout, tc.dist, tc.variant)
			gd, err := GaddagFromLexicon("NWL23")
			is.NoErr(err)
			alph := gd.GetAlphabet()
			gen := NewGordonGenerator(gd, g.Board(), g.Bag().LetterDistribution())

			check := func(wantBest int) {
				plays := scoringPlays(gen.GenAll(tilemapping.RackFromString(tc.rack, alph), false))
				is.True(len(plays) > 0)
				best := 0
				for _, m := range plays {
					rescored, err := g.CreateAndScorePlacementMove(m.BoardCoords(), m.TilesString(), tc.rack, false)
					is.NoErr(err)
					is.Equal(m.Score(), rescored.Score())
					best = max(best, m.Score())
				}
				if wantBest != 0 {
					is.Equal(best, wantBest)
				}
			}
			check(tc.bestOpening)

			// Now with something on the board, for cross-words and play-through.
			m, err := g.CreateAndScorePlacementMove("8D", "JUKEBOX", "JUKEBOX", false)
			is.NoErr(err)
			is.NoErr(g.PlayMove(m, false, 0))
			check(0)
		})
	}
}

// Shadow play skips anchors that can't beat the best play so far, so a bound
// using the wrong bonus would be free to skip the true best play.
func TestTopPlayMatchesAllPlaysOnCrossplay(t *testing.T) {
	is := is.New(t)
	g := newScoringGame(t, "", "", game.VarCrossplay)
	gd, err := GaddagFromLexicon("NWL23")
	is.NoErr(err)
	alph := gd.GetAlphabet()
	ld := g.Bag().LetterDistribution()

	opening, err := g.CreateAndScorePlacementMove("8D", "JUKEBOX", "JUKEBOX", false)
	is.NoErr(err)
	is.NoErr(g.PlayMove(opening, false, 0))

	for _, rack := range []string{"AEINRST", "SATIRE?", "???ABCD", "QZVVWWK", "DDESW??"} {
		all := NewGordonGenerator(gd, g.Board(), ld)
		best := 0
		for _, m := range all.GenAll(tilemapping.RackFromString(rack, alph), false) {
			if m.Action() == move.MoveTypePlay {
				best = max(best, m.Score())
			}
		}

		top := NewGordonGenerator(gd, g.Board(), ld)
		top.SetGame(g)
		top.SetEquityCalculators([]equity.EquityCalculator{equity.NewNoLeaveCalculator()})
		top.SetPlayRecorderTopPlay()
		plays := top.GenAll(tilemapping.RackFromString(rack, alph), false)
		is.Equal(len(plays), 1)
		is.Equal(plays[0].Score(), best)
	}
}

// Three blanks is one more than any other game has. Small moves, which the
// solvers use, have to represent them all too.
func TestThreeBlankRacks(t *testing.T) {
	is := is.New(t)
	g := newScoringGame(t, "", "", game.VarCrossplay)
	gd, err := GaddagFromLexicon("NWL23")
	is.NoErr(err)
	alph := gd.GetAlphabet()
	ld := g.Bag().LetterDistribution()

	opening, err := g.CreateAndScorePlacementMove("8D", "JUKEBOX", "JUKEBOX", false)
	is.NoErr(err)
	is.NoErr(g.PlayMove(opening, false, 0))

	rack := "???ABCD"
	all := NewGordonGenerator(gd, g.Board(), ld)
	plays := scoringPlays(all.GenAll(tilemapping.RackFromString(rack, alph), false))

	threeBlanks := 0
	wantScore := 0
	for _, m := range plays {
		blanks := 0
		for _, ml := range m.Tiles() {
			if ml != 0 && ml.IsBlanked() {
				blanks++
			}
		}
		if blanks == 3 {
			threeBlanks++
		}
		wantScore += m.Score()
	}
	is.True(threeBlanks > 0)

	small := NewGordonGenerator(gd, g.Board(), ld)
	small.SetPlayRecorder(AllPlaysSmallRecorder)
	small.GenAll(tilemapping.RackFromString(rack, alph), false)
	gotPlays, gotScore := 0, 0
	for _, sm := range small.SmallPlays() {
		if sm.IsPass() {
			continue
		}
		gotPlays++
		gotScore += int(sm.Score())
	}
	is.Equal(gotPlays, len(plays))
	is.Equal(gotScore, wantScore)
}
