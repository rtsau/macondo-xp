package equity

import (
	"testing"

	"github.com/domino14/word-golib/tilemapping"
	"github.com/matryer/is"

	"github.com/domino14/macondo/board"
	"github.com/domino14/macondo/config"
	"github.com/domino14/macondo/move"
	"github.com/domino14/macondo/testhelpers"
)

// The squares next to a 2LS, for the boards that once had them hardcoded.
func TestExposes2LS(t *testing.T) {
	for _, tc := range []struct {
		name string
		desc []string
		want map[int]bool
	}{
		{"standard", board.CrosswordGameBoard, map[int]bool{2: true, 6: true, 8: true, 12: true}},
		{"super", board.SuperCrosswordGameBoard, map[int]bool{5: true, 9: true, 11: true, 15: true}},
		// The rows beside the center have only 3Ls.
		{"crossplay", board.CrossplayBoard, map[int]bool{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)
			b := board.MakeBoard(tc.desc)
			for j := range b.Dim() {
				is.Equal(exposes2LS(b, j, false), tc.want[j])
				is.Equal(exposes2LS(b, j, true), tc.want[j])
			}
		})
	}
}

// Going out is worth something only where leftover tiles score.
func TestEndgameAdjustmentNeedsLeftoversToScore(t *testing.T) {
	is := is.New(t)
	alph := testhelpers.EnglishAlphabet()
	dist, err := tilemapping.EnglishLetterDistribution(config.DefaultConfig().WGLConfig())
	is.NoErr(err)
	word := func(s string) tilemapping.MachineWord {
		mw, err := tilemapping.ToMachineWord(s, alph)
		is.NoErr(err)
		return mw
	}
	oppRack := tilemapping.RackFromString("XY", alph)
	goingOut := move.NewScoringMove(10, word("AT"), nil, false, 2, alph, 7, 7)
	keeping := move.NewScoringMove(10, word("AT"), word("Z"), false, 2, alph, 7, 7)

	b := board.MakeBoard(board.CrosswordGameBoard)
	is.Equal(endgameAdjustment(goingOut, b, oppRack, dist), 24.0) // 2 * (8 + 4)
	is.Equal(endgameAdjustment(keeping, b, oppRack, dist), -30.0) // 2 * 10 + 10

	b.SetLeftoverRacksScore(false)
	is.Equal(endgameAdjustment(goingOut, b, oppRack, dist), 0.0)
	is.Equal(endgameAdjustment(keeping, b, oppRack, dist), 0.0)
	is.Equal(b.Copy().LeftoverRacksScore(), false)
}
