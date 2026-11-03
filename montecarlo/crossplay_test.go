package montecarlo

import (
	"testing"

	"github.com/domino14/word-golib/tilemapping"
	"github.com/matryer/is"

	"github.com/domino14/macondo/board"
	"github.com/domino14/macondo/equity"
	"github.com/domino14/macondo/game"
	pb "github.com/domino14/macondo/gen/api/proto/macondo"
)

// NWL23's word map exists but only knows racks of up to two blanks, which isn't
// enough for Crossplay.
func TestWMPIsOnlyUsedWhenItFitsTheTiles(t *testing.T) {
	is := is.New(t)
	for _, tc := range []struct {
		layout, dist string
		variant      game.Variant
		wantWMP      bool
	}{
		{"", "", game.VarCrossplay, false},
		{board.CrosswordGameLayout, "english", game.VarClassic, true},
	} {
		rules, err := game.NewBasicGameRules(DefaultConfig, "NWL23", tc.layout, tc.dist, game.CrossScoreAndSet, tc.variant)
		is.NoErr(err)
		g, err := game.NewGame(rules, []*pb.PlayerInfo{{Nickname: "a"}, {Nickname: "b"}})
		is.NoErr(err)
		g.StartGame()
		s := &Simmer{origGame: g}
		s.TryLoadWMP(DefaultConfig.WGLConfig(), "NWL23")
		is.Equal(s.WMP() != nil, tc.wantWMP)
	}
}

// Crossplay has no leave values yet. Until it does, a calculator must use none
// at all, and not NWL23's, which are for other tiles and other rules.
func TestCrossplayDoesNotBorrowLeaves(t *testing.T) {
	is := is.New(t)
	blank := tilemapping.MachineWord{0} // a lone blank is valuable in any set of leaves

	classic, err := equity.NewExhaustiveLeaveCalculator("NWL23", DefaultConfig, "")
	is.NoErr(err)
	is.True(classic.LeaveValue(blank) > 0)

	cp, err := equity.NewExhaustiveLeaveCalculator("NWL23", DefaultConfig, game.CrossplayLeavesFilename)
	is.NoErr(err)
	is.Equal(cp.LeaveValue(blank), 0.0)
}
