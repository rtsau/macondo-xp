package cgp

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/matryer/is"

	"github.com/domino14/macondo/board"
	"github.com/domino14/macondo/config"
	"github.com/domino14/macondo/game"
	pb "github.com/domino14/macondo/gen/api/proto/macondo"
	"github.com/domino14/macondo/move"
	"github.com/domino14/macondo/testhelpers"
	"github.com/domino14/word-golib/tilemapping"
)

var DefaultConfig = config.DefaultConfig()

func TestRowToLetters(t *testing.T) {
	is := is.New(t)
	testcases := []struct {
		row    string
		parsed []tilemapping.MachineLetter
	}{
		{"15", []tilemapping.MachineLetter{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		{"10AB3", []tilemapping.MachineLetter{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 0, 0, 0}},
		{"A3B10", []tilemapping.MachineLetter{1, 0, 0, 0, 2, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		{"1A1B2C3D4", []tilemapping.MachineLetter{0, 1, 0, 2, 0, 0, 3, 0, 0, 0, 4, 0, 0, 0, 0}},
	}
	for _, tc := range testcases {
		parsed, err := rowToLetters(tc.row, testhelpers.EnglishAlphabet())
		is.NoErr(err)
		is.Equal(parsed, tc.parsed)
	}
}

func TestRowToLettersMultichar(t *testing.T) {
	is := is.New(t)
	catalan, err := tilemapping.NamedLetterDistribution(DefaultConfig.WGLConfig(), "catalan")
	is.NoErr(err)
	testcases := []struct {
		row    string
		parsed []tilemapping.MachineLetter
	}{
		{"15", []tilemapping.MachineLetter{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		{"10AB3", []tilemapping.MachineLetter{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 2, 0, 0, 0}},
		{"A3b10", []tilemapping.MachineLetter{1, 0, 0, 0, 2 | 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0}},
		{"1A1B2C3D4", []tilemapping.MachineLetter{0, 1, 0, 2, 0, 0, 3, 0, 0, 0, 5, 0, 0, 0, 0}},
		{"[qu]3[L·L]5MA3", []tilemapping.MachineLetter{19 | 0x80, 0, 0, 0, 13, 0, 0, 0, 0, 0, 14, 1, 0, 0, 0}},
		{"[qu]3[L·L]5MA2Ç", []tilemapping.MachineLetter{19 | 0x80, 0, 0, 0, 13, 0, 0, 0, 0, 0, 14, 1, 0, 0, 4}},
		{"[QU]3[l·l]9[NY]", []tilemapping.MachineLetter{19, 0, 0, 0, 13 | 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 16}},
		{"QU3l·l9NY", []tilemapping.MachineLetter{19, 0, 0, 0, 13 | 0x80, 0, 0, 0, 0, 0, 0, 0, 0, 0, 16}},
	}
	for _, tc := range testcases {
		parsed, err := rowToLetters(tc.row, catalan.TileMapping())
		is.NoErr(err)
		is.Equal(parsed, tc.parsed)
	}
}

func TestCrossplayCGP(t *testing.T) {
	is := is.New(t)
	const empty = "15/15/15/15/15/15/15/15/15/15/15/15/15/15/15 JUKEBOX/ 0/0 0 lex NWL23;"

	// The variant alone selects the board and tiles.
	parsed, err := ParseCGP(DefaultConfig, empty+" var crossplay;")
	is.NoErr(err)
	is.Equal(parsed.Rules().Variant(), game.VarCrossplay)
	is.Equal(parsed.Rules().BoardName(), board.CrossplayLayout)
	is.Equal(parsed.Rules().LetterDistributionName(), game.CrossplayLetterDistribution)
	m, err := parsed.CreateAndScorePlacementMove("8D", "JUKEBOX", "JUKEBOX", false)
	is.NoErr(err)
	is.Equal(m.Score(), 132)

	// And it survives a trip through CGP.
	again, err := ParseCGP(DefaultConfig, parsed.ToCGP(false))
	is.NoErr(err)
	is.Equal(again.Rules().Variant(), game.VarCrossplay)
	is.Equal(again.Board().BingoBonus(), game.CrossplayBingoBonus)

	// Asking for a combination that can't work is an error, rather than a game
	// that plays with some of the rules.
	_, err = ParseCGP(DefaultConfig, empty+" var crossplay; ld english;")
	is.True(err != nil)
	_, err = ParseCGP(DefaultConfig, empty+" var crossplay; bdn CrosswordGame;")
	is.True(err != nil)
	_, err = ParseCGP(DefaultConfig, empty+" ld crossplay;")
	is.True(err != nil)

	// A CGP without the variant is still the classic game.
	classic, err := ParseCGP(DefaultConfig, empty)
	is.NoErr(err)
	is.Equal(classic.Rules().BoardName(), board.CrosswordGameLayout)
	is.Equal(classic.Board().BingoBonus(), 50)
}

func TestCrossplayFinalTurnsInCGP(t *testing.T) {
	is := is.New(t)
	// Every tile placed but what's on the racks, leaving the bag empty, in the
	// layout of the first real game's last turns would be long; so a full board
	// isn't used. ftl sets what the tile count can't show here.
	const board = "15/15/15/15/15/15/15/3JUKEBOX5/15/15/15/15/15/15/15"

	for _, ftl := range []int{1, 2} {
		s := fmt.Sprintf("%s AE/IN 40/0 0 lex NWL23; var crossplay; ftl %d;", board, ftl)
		parsed, err := ParseCGP(DefaultConfig, s)
		is.NoErr(err)
		is.Equal(parsed.FinalTurnsLeft(), ftl)
		is.Equal(parsed.Playing(), pb.PlayState_PLAYING)

		// It survives the trip, and tells apart what the racks can't.
		again, err := ParseCGP(DefaultConfig, parsed.ToCGP(false))
		is.NoErr(err)
		is.Equal(again.FinalTurnsLeft(), ftl)
	}

	over, err := ParseCGP(DefaultConfig, board+" AE/IN 40/0 0 lex NWL23; var crossplay; ftl 0;")
	is.NoErr(err)
	is.Equal(over.Playing(), pb.PlayState_GAME_OVER)
	again, err := ParseCGP(DefaultConfig, over.ToCGP(false))
	is.NoErr(err)
	is.Equal(again.Playing(), pb.PlayState_GAME_OVER)

	// A bag with tiles has no final turns yet, and a position that doesn't say
	// isn't given any.
	mid, err := ParseCGP(DefaultConfig, board+" AE/IN 40/0 0 lex NWL23; var crossplay;")
	is.NoErr(err)
	is.Equal(mid.FinalTurnsLeft(), 0)
	is.True(!strings.Contains(mid.ToCGP(false), "ftl"))

	// Without a limit from the position, Crossplay has none.
	for i := 0; i < 20; i++ {
		is.NoErr(mid.PlayMove(move.NewPassMove(nil, mid.Alphabet()), false, 0))
	}
	is.Equal(mid.Playing(), pb.PlayState_PLAYING)
	// With one, it does.
	limited, err := ParseCGP(DefaultConfig, board+" AE/IN 40/0 0 lex NWL23; var crossplay; mcnz 2;")
	is.NoErr(err)
	is.NoErr(limited.PlayMove(move.NewPassMove(nil, limited.Alphabet()), false, 0))
	is.NoErr(limited.PlayMove(move.NewPassMove(nil, limited.Alphabet()), false, 0))
	is.Equal(limited.Playing(), pb.PlayState_GAME_OVER)

	_, err = ParseCGP(DefaultConfig, board+" AE/IN 40/0 0 lex NWL23; ftl 1;")
	is.True(err != nil) // only for Crossplay
	_, err = ParseCGP(DefaultConfig, board+" AE/IN 40/0 0 lex NWL23; var crossplay; ftl 3;")
	is.True(err != nil)
}

// With the racks holding every unplayed tile, the bag is empty, and a position that
// doesn't say how many final turns are left gets them from the racks.
func TestCrossplayInfersFinalTurnsFromRacks(t *testing.T) {
	rules, err := game.NewBasicGameRules(DefaultConfig, "NWL23", "", "", game.CrossScoreAndSet, game.VarCrossplay)
	if err != nil {
		t.Fatal(err)
	}
	players := []*pb.PlayerInfo{{Nickname: "a"}, {Nickname: "b"}}

	for _, tc := range []struct {
		name       string
		onTurnRack int
		wantFinal  int
	}{
		{"short rack on turn is the last drawer", 3, 1},
		{"full racks assume the bag was just emptied", 7, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)
			g, err := game.NewGame(rules, players)
			is.NoErr(err)
			g.StartGame()
			// Everything but two racks goes on the board, off to the side.
			tiles := g.Bag().Peek()
			tiles = append(tiles, g.RackFor(0).TilesOn()...)
			tiles = append(tiles, g.RackFor(1).TilesOn()...)
			// Blanks first, so they go on the racks: 0 on a board row means empty.
			slices.Sort(tiles)
			racks := [2]tilemapping.MachineWord{}
			racks[0] = tilemapping.MachineWord(tiles[:tc.onTurnRack])
			racks[1] = tilemapping.MachineWord(tiles[tc.onTurnRack : tc.onTurnRack+7])
			rest := tiles[tc.onTurnRack+7:]
			rows := make([][]tilemapping.MachineLetter, 15)
			for r := range rows {
				rows[r] = make([]tilemapping.MachineLetter, 15)
				for c := 0; c < 15 && len(rest) > 0; c++ {
					rows[r][c] = rest[0]
					rest = rest[1:]
				}
			}
			alph := g.Alphabet()
			s, err := game.NewFromSnapshot(rules, players,
				[]string{racks[0].UserVisible(alph), racks[1].UserVisible(alph)}, []int{0, 0}, rows)
			is.NoErr(err)
			is.Equal(s.Bag().TilesRemaining(), 0)
			is.Equal(s.FinalTurnsLeft(), tc.wantFinal)
			is.Equal(s.Playing(), pb.PlayState_PLAYING)
		})
	}
}
