package automatic

import (
	"testing"

	"github.com/matryer/is"

	"github.com/domino14/macondo/config"
	"github.com/domino14/macondo/game"
	pb "github.com/domino14/macondo/gen/api/proto/macondo"
	"github.com/domino14/macondo/move"
)

// Asking for Crossplay's tiles plays Crossplay, for a whole game with its ending.
func TestRunnerPlaysCrossplay(t *testing.T) {
	is := is.New(t)
	cfg := config.DefaultConfig()
	r := &GameRunner{config: cfg, lexicon: "NWL23", letterDistribution: "crossplay"}
	players := []AutomaticRunnerPlayer{{BotCode: pb.BotRequest_HASTY_BOT}, {BotCode: pb.BotRequest_HASTY_BOT}}
	is.NoErr(r.Init(players))
	is.Equal(r.game.Rules().Variant(), game.VarCrossplay)
	is.Equal(r.game.Board().BingoBonus(), game.CrossplayBingoBonus)

	for gidx := 0; gidx < 20; gidx++ {
		is.NoErr(r.playGame(true, gidx, [32]byte{byte(gidx + 1)}))
		is.Equal(r.game.Playing(), pb.PlayState_GAME_OVER)
		is.Equal(r.game.Bag().TilesRemaining(), 0)
		is.Equal(r.game.FinalTurnsLeft(), 0)
		// The score is what was scored, and nothing else.
		scores := [2]int{}
		for _, e := range r.game.History().Events {
			scores[e.PlayerIndex] += int(e.Score)
		}
		is.Equal(r.game.PointsFor(0), scores[0])
		is.Equal(r.game.PointsFor(1), scores[1])
	}

	// The limit for bots is there, so that a pair that won't play can't go forever.
	r.game.StartGame()
	for i := 0; i < game.DefaultMaxScorelessTurns; i++ {
		is.Equal(r.game.Playing(), pb.PlayState_PLAYING)
		is.NoErr(r.game.PlayMove(passMove(r.game), false, 0))
	}
	is.Equal(r.game.Playing(), pb.PlayState_GAME_OVER)
}

func TestRunnerStaysClassicForOtherTiles(t *testing.T) {
	is := is.New(t)
	r := &GameRunner{config: config.DefaultConfig(), lexicon: "NWL23", letterDistribution: "English"}
	is.NoErr(r.Init([]AutomaticRunnerPlayer{{BotCode: pb.BotRequest_HASTY_BOT}, {BotCode: pb.BotRequest_HASTY_BOT}}))
	is.Equal(r.game.Rules().Variant(), game.VarClassic)
	is.Equal(r.game.Board().BingoBonus(), 50)
}

func passMove(g *game.Game) *move.Move {
	return move.NewPassMove(g.RackFor(g.PlayerOnTurn()).TilesOn(), g.Alphabet())
}
