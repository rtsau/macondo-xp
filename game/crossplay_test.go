package game

import (
	"testing"

	"github.com/domino14/word-golib/tilemapping"
	"github.com/matryer/is"

	"github.com/domino14/macondo/board"
	pb "github.com/domino14/macondo/gen/api/proto/macondo"
)

var crossplayPlayers = []*pb.PlayerInfo{
	{Nickname: "JD", RealName: "Jesse"},
	{Nickname: "cesar", RealName: "César"},
}

func TestCrossplayTiles(t *testing.T) {
	is := is.New(t)
	ld, err := tilemapping.GetDistribution(DefaultConfig.WGLConfig(), CrossplayLetterDistribution)
	is.NoErr(err)

	expected := map[string]struct{ count, score int }{
		"?": {3, 0}, "A": {9, 1}, "B": {2, 4}, "C": {2, 3}, "D": {4, 2}, "E": {12, 1},
		"F": {2, 4}, "G": {3, 4}, "H": {3, 3}, "I": {8, 1}, "J": {1, 10}, "K": {1, 6},
		"L": {4, 2}, "M": {2, 3}, "N": {5, 1}, "O": {8, 1}, "P": {2, 3}, "Q": {1, 10},
		"R": {6, 1}, "S": {5, 1}, "T": {6, 1}, "U": {3, 2}, "V": {2, 6}, "W": {2, 5},
		"X": {1, 8}, "Y": {2, 4}, "Z": {1, 10},
	}
	is.Equal(len(expected), 27)
	tiles, points := 0, 0
	for letter, want := range expected {
		mls, err := tilemapping.ToMachineLetters(letter, ld.TileMapping())
		is.NoErr(err)
		is.Equal(int(ld.Distribution()[mls[0]]), want.count)
		is.Equal(ld.Score(mls[0]), want.score)
		tiles += want.count
		points += want.count * want.score
	}
	// The totals in the published comparison with Scrabble.
	is.Equal(tiles, 100)
	is.Equal(points, 210)
	is.Equal(ld.MakeBag().TilesRemaining(), 100)
}

func TestCrossplayVariantSettings(t *testing.T) {
	is := is.New(t)
	for _, tc := range []struct {
		name          string
		layout, dist  string
		variant       Variant
		ok            bool
		wantLayout    string
		wantDist      string
		wantBingo     int
		wantExchLimit int
	}{
		{"variant alone", "", "", VarCrossplay, true, board.CrossplayLayout, "crossplay", 40, 7},
		{"spelled out", board.CrossplayLayout, "crossplay", VarCrossplay, true, board.CrossplayLayout, "crossplay", 40, 7},
		{"case of tiles", board.CrossplayLayout, "Crossplay", VarCrossplay, true, board.CrossplayLayout, "crossplay", 40, 7},
		{"wrong board", board.CrosswordGameLayout, "crossplay", VarCrossplay, false, "", "", 0, 0},
		{"wrong tiles", board.CrossplayLayout, "english", VarCrossplay, false, "", "", 0, 0},
		{"board without variant", board.CrossplayLayout, "english", VarClassic, false, "", "", 0, 0},
		{"tiles without variant", board.CrosswordGameLayout, "crossplay", VarClassic, false, "", "", 0, 0},
		{"both without variant", board.CrossplayLayout, "crossplay", "", false, "", "", 0, 0},
		{"classic is untouched", board.CrosswordGameLayout, "english", VarClassic, true, board.CrosswordGameLayout, "english", 50, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)
			rules, err := NewBasicGameRules(DefaultConfig, "NWL23", tc.layout, tc.dist, CrossScoreAndSet, tc.variant)
			if !tc.ok {
				is.True(err != nil)
				return
			}
			is.NoErr(err)
			is.Equal(rules.BoardName(), tc.wantLayout)
			is.Equal(rules.LetterDistributionName(), tc.wantDist)
			is.Equal(rules.Board().BingoBonus(), tc.wantBingo)
			is.Equal(rules.ExchangeLimit(), tc.wantExchLimit)
		})
	}
	_ = is
}

func newCrossplayGame(t *testing.T) *Game {
	t.Helper()
	is := is.New(t)
	rules, err := NewBasicGameRules(DefaultConfig, "NWL23", "", "", CrossScoreAndSet, VarCrossplay)
	is.NoErr(err)
	g, err := NewGame(rules, crossplayPlayers)
	is.NoErr(err)
	g.StartGame()
	return g
}

func TestCrossplayNewGame(t *testing.T) {
	is := is.New(t)
	g := newCrossplayGame(t)
	is.Equal(g.Bag().TilesRemaining(), 86)
	is.Equal(g.History().Variant, string(VarCrossplay))
	is.Equal(g.History().BoardLayout, board.CrossplayLayout)
	is.Equal(g.History().LetterDistribution, CrossplayLetterDistribution)
	is.Equal(g.Board().BingoBonus(), 40)
	is.Equal(g.Copy().Board().BingoBonus(), 40)
}

// JUKEBOX on the center row, with its face value of 32 (27 in Scrabble). The
// expected scores are worked out by hand from the board and the tile values.
func TestCrossplayScoring(t *testing.T) {
	is := is.New(t)
	g := newCrossplayGame(t)
	for _, tc := range []struct {
		coords string
		score  int
	}{
		// U and O on 2L: 32 + 2 + 1, + 40.
		{"8E", 75},
		// J on 2W; K and X on 2L: (32 + 6 + 8) * 2, + 40.
		{"8D", 132},
		// J and B on 2L; X on 2W: (32 + 10 + 4) * 2, + 40.
		{"8F", 132},
		// K on 2L; B on 2W: (32 + 6) * 2, + 40.
		{"8H", 116},
		// The same on the column through the center.
		{"H4", 132},
	} {
		m, err := g.CreateAndScorePlacementMove(tc.coords, "JUKEBOX", "JUKEBOX", false)
		is.NoErr(err)
		is.Equal(m.Score(), tc.score)
	}

	// A blank scores nothing, but still counts toward a sweep. As 8E above, less
	// the 8 for the X.
	m, err := g.CreateAndScorePlacementMove("8E", "JUKEBOx", "JUKEBO?", false)
	is.NoErr(err)
	is.Equal(m.Score(), 75-8)

	// Fewer than seven tiles get no bonus. J and B on 2L: 20 + 2 + 6 + 1 + 8.
	m, err = g.CreateAndScorePlacementMove("8F", "JUKEB", "JUKEBOX", false)
	is.NoErr(err)
	is.Equal(m.Score(), 37)
}

func TestClassicScoringIsUnchanged(t *testing.T) {
	is := is.New(t)
	rules, err := NewBasicGameRules(DefaultConfig, "NWL23", board.CrosswordGameLayout, "english", CrossScoreAndSet, VarClassic)
	is.NoErr(err)
	g, err := NewGame(rules, crossplayPlayers)
	is.NoErr(err)
	g.StartGame()
	// J on 2L and B on the center 2W: the face value of 27, + 8 for the J,
	// doubled, + 50.
	m, err := g.CreateAndScorePlacementMove("8D", "JUKEBOX", "JUKEBOX", false)
	is.NoErr(err)
	is.Equal(m.Score(), (27+8)*2+50)
}

func TestCrossplayHistoryToVariant(t *testing.T) {
	is := is.New(t)
	// A history saying only the variant, as a GCG file does. The lexicon on its
	// own would suggest English tiles.
	layout, dist, variant := HistoryToVariant(&pb.GameHistory{Lexicon: "NWL23", Variant: "crossplay"})
	is.Equal(variant, VarCrossplay)
	rules, err := NewBasicGameRules(DefaultConfig, "NWL23", layout, dist, CrossScoreAndSet, variant)
	is.NoErr(err)
	is.Equal(rules.BoardName(), board.CrossplayLayout)
	is.Equal(rules.LetterDistributionName(), CrossplayLetterDistribution)

	// Classic histories still work as they did.
	layout, dist, variant = HistoryToVariant(&pb.GameHistory{Lexicon: "NWL23", BoardLayout: board.CrosswordGameLayout})
	is.Equal(layout, board.CrosswordGameLayout)
	is.Equal(dist, "english")
	is.Equal(variant, Variant(""))
}

func TestCrossplayLeavesFile(t *testing.T) {
	is := is.New(t)
	cp, err := NewBasicGameRules(DefaultConfig, "NWL23", "", "", CrossScoreAndSet, VarCrossplay)
	is.NoErr(err)
	is.Equal(cp.LeavesFileOr(""), CrossplayLeavesFilename)
	is.Equal(cp.LeavesFileOr("leaves.klv2"), CrossplayLeavesFilename)
	is.Equal(cp.LeavesFileOr("super-leaves.klv2"), CrossplayLeavesFilename)
	is.Equal(cp.LeavesFileOr("candidate.klv2"), "candidate.klv2") // for training

	classic, err := NewBasicGameRules(DefaultConfig, "NWL23", board.CrosswordGameLayout, "english", CrossScoreAndSet, VarClassic)
	is.NoErr(err)
	is.Equal(classic.LeavesFileOr(""), "")
	is.Equal(classic.LeavesFileOr("super-leaves.klv2"), "super-leaves.klv2")
}
