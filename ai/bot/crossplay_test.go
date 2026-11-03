package bot

import (
	"testing"

	"github.com/domino14/word-golib/tilemapping"
	"github.com/matryer/is"

	"github.com/domino14/macondo/game"
	pb "github.com/domino14/macondo/gen/api/proto/macondo"
)

// A bot plays under Crossplay's scoring. With no leave values yet, its top play
// is simply the one that scores the most.
func TestHastyBotOnCrossplay(t *testing.T) {
	is := is.New(t)
	rules, err := game.NewBasicGameRules(DefaultConfig, "NWL23", "", "", game.CrossScoreAndSet, game.VarCrossplay)
	is.NoErr(err)
	g, err := game.NewGame(rules, []*pb.PlayerInfo{{Nickname: "a"}, {Nickname: "b"}})
	is.NoErr(err)
	g.StartGame()
	is.NoErr(g.SetRackFor(g.PlayerOnTurn(), tilemapping.RackFromString("JUKEBOX", g.Alphabet())))

	btp, err := NewBotTurnPlayerFromGame(g, &BotConfig{Config: *DefaultConfig}, pb.BotRequest_HASTY_BOT)
	is.NoErr(err)
	moves := btp.GenerateMoves(1)
	is.Equal(len(moves), 1)
	is.Equal(moves[0].Score(), 132)
	is.Equal(moves[0].Equity(), 132.0)
}
