package bot

import (
	"context"
	"testing"
	"time"

	"github.com/domino14/word-golib/kwg"
	"github.com/domino14/word-golib/tilemapping"
	"github.com/matryer/is"

	"github.com/domino14/macondo/endgame/finalturns"
	"github.com/domino14/macondo/game"
	"github.com/domino14/macondo/gcgio"
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

// The last turns of a real game. The GCG's 27th event emptied the bag.
func crossplayEndgame(t *testing.T, turn int) *game.Game {
	t.Helper()
	is := is.New(t)
	hist, err := gcgio.ParseGCG(DefaultConfig, "../../gcgio/testdata/crossplay_game2.gcg")
	is.NoErr(err)
	rules, err := game.NewBasicGameRules(DefaultConfig, "NWL23", "", "", game.CrossScoreAndSet, game.VarCrossplay)
	is.NoErr(err)
	g, err := game.NewFromHistory(hist, rules, turn)
	is.NoErr(err)
	return g
}

// Bots with an endgame engine use the one for these rules, at both final turns.
func TestEndgameBotsSolveCrossplay(t *testing.T) {
	is := is.New(t)
	gd, err := kwg.GetKWG(DefaultConfig.WGLConfig(), "NWL23")
	is.NoErr(err)
	for _, tc := range []struct {
		turn, finalTurns int
	}{{27, 2}, {28, 1}} {
		for _, bt := range []pb.BotRequest_BotCode{pb.BotRequest_SIMMING_BOT, pb.BotRequest_SIMMING_INFER_BOT} {
			g := crossplayEndgame(t, tc.turn)
			is.Equal(g.FinalTurnsLeft(), tc.finalTurns)
			_, seq, err := finalturns.Solve(context.Background(), g, gd)
			is.NoErr(err)

			btp, err := NewBotTurnPlayerFromGame(g, &BotConfig{Config: *DefaultConfig}, bt)
			is.NoErr(err)
			m, err := btp.BestPlay(context.Background())
			is.NoErr(err)
			is.Equal(m.ShortDescription(), seq[0].ShortDescription())
		}
	}
}

// A bot without an endgame engine, and with its quick search configured, doesn't
// run a search made for other rules: it plays the highest-scoring play.
func TestQuickEndgameStaysOutOfCrossplay(t *testing.T) {
	is := is.New(t)
	g := crossplayEndgame(t, 27)
	conf := &BotConfig{Config: *DefaultConfig, QuickEndgamePlies: 2}
	btp, err := NewBotTurnPlayerFromGame(g, conf, pb.BotRequest_HASTY_BOT)
	is.NoErr(err)
	m, err := btp.BestPlay(context.Background())
	is.NoErr(err)
	is.Equal(btp.QuickEndgameSolves, 0)
	is.Equal(m.Score(), btp.GenerateMoves(1)[0].Score())
}

// One tile in the bag is when other games would use the pre-endgame solver, which
// refuses Crossplay. The bot has to fall back on simming, not stop with an error.
func TestSimmingBotPlaysWithOneTileInTheBag(t *testing.T) {
	is := is.New(t)
	g := crossplayEndgame(t, 26)
	is.Equal(g.Bag().TilesRemaining(), 2)
	is.NoErr(g.Bag().Draw(1, make([]tilemapping.MachineLetter, 1)))

	btp, err := NewBotTurnPlayerFromGame(g, &BotConfig{Config: *DefaultConfig}, pb.BotRequest_SIMMING_BOT)
	is.NoErr(err)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	m, err := btp.BestPlay(ctx)
	is.NoErr(err)
	is.True(m != nil)
}
