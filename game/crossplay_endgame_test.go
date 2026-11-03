package game

import (
	"testing"

	"github.com/domino14/word-golib/tilemapping"
	"github.com/matryer/is"

	pb "github.com/domino14/macondo/gen/api/proto/macondo"
	"github.com/domino14/macondo/move"
)

// newCrossplayEndgame deals known racks and leaves bagLeft random tiles in the bag.
// These tests lay tiles without checking words, since only the rules are at stake.
func newCrossplayEndgame(t *testing.T, rack0, rack1 string, bagLeft int) *Game {
	t.Helper()
	is := is.New(t)
	g := newCrossplayGame(t)
	is.NoErr(g.SetRacksForBoth([]*tilemapping.Rack{
		tilemapping.RackFromString(rack0, g.Alphabet()),
		tilemapping.RackFromString(rack1, g.Alphabet()),
	}))
	n := g.Bag().TilesRemaining() - bagLeft
	is.NoErr(g.Bag().Draw(n, make([]tilemapping.MachineLetter, n)))
	is.Equal(g.Bag().TilesRemaining(), bagLeft)
	return g
}

func playTiles(t *testing.T, g *Game, coords, tiles string) int {
	t.Helper()
	is := is.New(t)
	m, err := g.CreateAndScorePlacementMove(coords, tiles, g.RackLettersFor(g.PlayerOnTurn()), false)
	is.NoErr(err)
	is.NoErr(g.PlayMove(m, false, 0))
	return m.Score()
}

func pass(t *testing.T, g *Game) {
	t.Helper()
	is.New(t).NoErr(g.PlayMove(move.NewPassMove(g.RackFor(g.PlayerOnTurn()).TilesOn(), g.Alphabet()), false, 0))
}

// An exact emptying, as in both real games: the last drawer keeps seven tiles.
func TestCrossplayEndAfterExactEmptying(t *testing.T) {
	is := is.New(t)
	g := newCrossplayEndgame(t, "ATLNEIR", "OXYGENS", 2)

	s0 := playTiles(t, g, "8G", "AT")
	is.Equal(g.Bag().TilesRemaining(), 0)
	is.Equal(int(g.RackFor(0).NumTiles()), 7)
	is.Equal(g.FinalTurnsLeft(), 2)
	is.Equal(g.Playing(), pb.PlayState_PLAYING)
	is.Equal(g.PlayerOnTurn(), 1)

	s1 := playTiles(t, g, "9G", "OX")
	is.Equal(g.FinalTurnsLeft(), 1)
	is.Equal(g.Playing(), pb.PlayState_PLAYING)
	is.Equal(g.PlayerOnTurn(), 0)

	s0 += playTiles(t, g, "10G", "LE")
	is.Equal(g.FinalTurnsLeft(), 0)
	is.Equal(g.Playing(), pb.PlayState_GAME_OVER)

	// Both still hold tiles, which change nothing.
	is.True(g.RackFor(0).NumTiles() > 0)
	is.True(g.RackFor(1).NumTiles() > 0)
	is.Equal(g.PointsFor(0), s0)
	is.Equal(g.PointsFor(1), s1)

	// And no more moves.
	m, err := g.CreateAndScorePlacementMove("11G", "YG", g.RackLettersFor(1), false)
	is.NoErr(err)
	_, err = g.ValidateMove(m)
	is.True(err != nil)
}

// Overdrawing leaves fewer tiles for the last turn: 7 - (played - in the bag).
func TestCrossplayOverdrawShortensTheLastRack(t *testing.T) {
	for _, tc := range []struct {
		name     string
		coords   string
		tiles    string
		bagLeft  int
		wantRack int
	}{
		{"two tiles into one", "8G", "AT", 1, 6},
		{"five tiles into two", "8E", "ATLNE", 2, 4},
		{"a sweep into one", "8B", "ATLNEIR", 1, 1},
		{"a sweep into seven", "8B", "ATLNEIR", 7, 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			is := is.New(t)
			g := newCrossplayEndgame(t, "ATLNEIR", "OXYGENS", tc.bagLeft)
			playTiles(t, g, tc.coords, tc.tiles)
			is.Equal(g.Bag().TilesRemaining(), 0)
			is.Equal(int(g.RackFor(0).NumTiles()), tc.wantRack)
			is.Equal(int(g.RackFor(1).NumTiles()), 7)
			is.Equal(g.FinalTurnsLeft(), 2)
		})
	}
}

// A play that doesn't empty the bag doesn't start the final turns.
func TestCrossplayFinalTurnsWaitForAnEmptyBag(t *testing.T) {
	is := is.New(t)
	g := newCrossplayEndgame(t, "ATLNEIR", "OXYGENS", 5)
	playTiles(t, g, "8G", "AT")
	is.Equal(g.Bag().TilesRemaining(), 3)
	is.Equal(g.FinalTurnsLeft(), 0)
	// The opponent takes the last three, so they become the last drawer.
	playTiles(t, g, "9F", "OXY")
	is.Equal(g.Bag().TilesRemaining(), 0)
	is.Equal(g.FinalTurnsLeft(), 2)
	is.Equal(g.PlayerOnTurn(), 0)
	pass(t, g)
	is.Equal(g.PlayerOnTurn(), 1)
	is.Equal(g.Playing(), pb.PlayState_PLAYING)
	pass(t, g)
	is.Equal(g.Playing(), pb.PlayState_GAME_OVER)
}

// The opponent going out on their final turn doesn't end the game or earn anything.
func TestCrossplayGoingOutEndsNothing(t *testing.T) {
	is := is.New(t)
	g := newCrossplayEndgame(t, "ATLNEIR", "OXYGENS", 1)
	s0 := playTiles(t, g, "8G", "AT")
	s1 := playTiles(t, g, "9B", "OXYGENS")
	is.Equal(int(g.RackFor(1).NumTiles()), 0)
	is.Equal(g.Playing(), pb.PlayState_PLAYING)
	is.Equal(g.FinalTurnsLeft(), 1)
	is.Equal(g.PointsFor(1), s1) // no bonus for the opponent's rack

	s0 += playTiles(t, g, "10G", "LE")
	is.Equal(g.Playing(), pb.PlayState_GAME_OVER)
	is.Equal(g.PointsFor(0), s0)
	is.Equal(g.PointsFor(1), s1) // no penalty for going out first, either
}

// The last drawer going out earns nothing for the opponent's tiles.
func TestCrossplayLastDrawerGoingOutEarnsNothing(t *testing.T) {
	is := is.New(t)
	g := newCrossplayEndgame(t, "ATLNEIR", "OXYGENS", 0)
	g.SetFinalTurnsLeft(1)
	// A short rack to go out with.
	is.NoErr(g.SetRacksForBoth([]*tilemapping.Rack{
		tilemapping.RackFromString("AT", g.Alphabet()),
		tilemapping.RackFromString("OXYGENS", g.Alphabet()),
	}))
	s0 := playTiles(t, g, "8G", "AT")
	is.Equal(g.Playing(), pb.PlayState_GAME_OVER)
	is.Equal(g.PointsFor(0), s0)
	is.Equal(g.PointsFor(1), 0)
}

// A pass uses up a final turn like any other move.
func TestCrossplayPassesOnFinalTurns(t *testing.T) {
	t.Run("opponent passes", func(t *testing.T) {
		is := is.New(t)
		g := newCrossplayEndgame(t, "ATLNEIR", "OXYGENS", 2)
		playTiles(t, g, "8G", "AT")
		pass(t, g)
		is.Equal(g.Playing(), pb.PlayState_PLAYING)
		is.Equal(g.FinalTurnsLeft(), 1)
		s := g.PointsFor(0)
		s += playTiles(t, g, "9G", "LE")
		is.Equal(g.Playing(), pb.PlayState_GAME_OVER)
		is.Equal(g.PointsFor(0), s)
	})
	t.Run("last drawer passes", func(t *testing.T) {
		is := is.New(t)
		g := newCrossplayEndgame(t, "ATLNEIR", "OXYGENS", 2)
		s0 := playTiles(t, g, "8G", "AT")
		s1 := playTiles(t, g, "9G", "OX")
		pass(t, g)
		is.Equal(g.Playing(), pb.PlayState_GAME_OVER)
		is.Equal(g.PointsFor(0), s0)
		is.Equal(g.PointsFor(1), s1)
	})
	t.Run("two passes after earlier passes", func(t *testing.T) {
		// A run of passes before the bag empties can't end the final turns early.
		is := is.New(t)
		g := newCrossplayEndgame(t, "ATLNEIR", "OXYGENS", 2)
		g.SetMaxScorelessTurns(2)
		pass(t, g)
		playTiles(t, g, "8G", "OX")
		is.Equal(g.FinalTurnsLeft(), 2)
		pass(t, g)
		is.Equal(g.Playing(), pb.PlayState_PLAYING)
		pass(t, g)
		is.Equal(g.Playing(), pb.PlayState_GAME_OVER)
	})
}

// There's no limit on passes and swaps while the bag has tiles.
func TestCrossplayScorelessTurnsAreUnlimited(t *testing.T) {
	is := is.New(t)
	g := newCrossplayEndgame(t, "ATLNEIR", "OXYGENS", 30)
	for i := 0; i < 50; i++ {
		pass(t, g)
		is.Equal(g.Playing(), pb.PlayState_PLAYING)
	}
	for i := 0; i < 50; i++ {
		rack := g.RackFor(g.PlayerOnTurn()).TilesOn()
		is.NoErr(g.PlayMove(move.NewExchangeMove(rack[:1], rack[1:], g.Alphabet()), false, 0))
		is.Equal(g.Playing(), pb.PlayState_PLAYING)
		is.Equal(g.Bag().TilesRemaining(), 30)
	}
	is.Equal(g.FinalTurnsLeft(), 0)
	is.Equal(g.PointsFor(0), 0)
	is.Equal(g.PointsFor(1), 0)
}

// Bots get a limit so that they can't go on forever, without any rack penalty.
func TestCrossplayScorelessLimitForBots(t *testing.T) {
	is := is.New(t)
	g := newCrossplayEndgame(t, "ATLNEIR", "OXYGENS", 30)
	g.SetMaxScorelessTurns(DefaultMaxScorelessTurns)
	for i := 0; i < DefaultMaxScorelessTurns-1; i++ {
		pass(t, g)
		is.Equal(g.Playing(), pb.PlayState_PLAYING)
	}
	pass(t, g)
	is.Equal(g.Playing(), pb.PlayState_GAME_OVER)
	is.Equal(g.PointsFor(0), 0)
	is.Equal(g.PointsFor(1), 0)
}

// Solvers undo moves constantly, so undoing has to restore all of this exactly.
func TestCrossplayUndoRestoresFinalTurns(t *testing.T) {
	is := is.New(t)
	g := newCrossplayEndgame(t, "ATLNEIR", "OXYGENS", 2)
	g.SetBackupMode(SimulationMode)
	g.SetStateStackLength(5)

	type state struct {
		final, onturn, p0, p1, bag int
		playing                    pb.PlayState
		r0, r1                     string
	}
	snap := func() state {
		return state{g.FinalTurnsLeft(), g.PlayerOnTurn(), g.PointsFor(0), g.PointsFor(1),
			g.Bag().TilesRemaining(), g.Playing(), g.RackLettersFor(0), g.RackLettersFor(1)}
	}
	states := []state{snap()}
	playTiles(t, g, "8G", "AT")
	states = append(states, snap())
	playTiles(t, g, "9G", "OX")
	states = append(states, snap())
	playTiles(t, g, "10G", "LE")
	is.Equal(g.Playing(), pb.PlayState_GAME_OVER)

	for i := len(states) - 1; i >= 0; i-- {
		g.UnplayLastMove()
		is.Equal(snap(), states[i])
	}

	// All the way back, in one step.
	playTiles(t, g, "8G", "AT")
	playTiles(t, g, "9G", "OX")
	playTiles(t, g, "10G", "LE")
	g.ResetToFirstState()
	is.Equal(snap(), states[0])
}

func TestCrossplayCopyKeepsFinalTurns(t *testing.T) {
	is := is.New(t)
	g := newCrossplayEndgame(t, "ATLNEIR", "OXYGENS", 2)
	playTiles(t, g, "8G", "AT")

	c := g.Copy()
	is.Equal(c.FinalTurnsLeft(), 2)
	pass(t, c)
	is.Equal(c.FinalTurnsLeft(), 1)
	pass(t, c)
	is.Equal(c.Playing(), pb.PlayState_GAME_OVER)
	// The copy's game doesn't touch this one.
	is.Equal(g.FinalTurnsLeft(), 2)
	is.Equal(g.Playing(), pb.PlayState_PLAYING)
}

// A game played to its end records the result, and nothing for the tiles left over.
// Replaying a history is tested on whole games, in endgame/finalturns.
func TestCrossplayHistory(t *testing.T) {
	is := is.New(t)
	g := newCrossplayGame(t)
	g.SetChallengeRule(pb.ChallengeRule_DOUBLE) // don't check words
	is.NoErr(g.SetRacksForBoth([]*tilemapping.Rack{
		tilemapping.RackFromString("ATLNEIR", g.Alphabet()),
		tilemapping.RackFromString("OXYGENS", g.Alphabet()),
	}))
	n := g.Bag().TilesRemaining() - 2
	is.NoErr(g.Bag().Draw(n, make([]tilemapping.MachineLetter, n)))

	playInHistory := func(coords, tiles string) {
		m, err := g.CreateAndScorePlacementMove(coords, tiles, g.RackLettersFor(g.PlayerOnTurn()), false)
		is.NoErr(err)
		is.NoErr(g.PlayMove(m, true, 0))
	}
	playInHistory("8G", "AT")
	is.Equal(len(g.History().FinalScores), 0)
	playInHistory("9G", "OX")
	is.Equal(g.History().PlayState, pb.PlayState_PLAYING)
	playInHistory("10G", "LE")

	is.Equal(g.Playing(), pb.PlayState_GAME_OVER)
	is.Equal(g.History().PlayState, pb.PlayState_GAME_OVER)
	is.Equal(int(g.History().FinalScores[0]), g.PointsFor(0))
	is.Equal(int(g.History().FinalScores[1]), g.PointsFor(1))
	is.Equal(len(g.History().Events), 3) // nothing added for the tiles left over
	p0, p1 := g.PointsFor(0), g.PointsFor(1)
	is.Equal(p0+p1 > 0, true)
}

func TestCrossplayEndgameModeRestoresLimits(t *testing.T) {
	is := is.New(t)
	g := newCrossplayEndgame(t, "ATLNEIR", "OXYGENS", 30)
	g.SetScorelessTurns(4)

	g.SetEndgameMode(true)
	is.Equal(g.ScorelessTurns(), 1)
	pass(t, g)
	is.Equal(g.Playing(), pb.PlayState_GAME_OVER) // the second pass in a row
	is.Equal(g.PointsFor(0), 0)
	g.SetPlaying(pb.PlayState_PLAYING)

	g.SetEndgameMode(false)
	is.Equal(g.ScorelessTurns(), 4)
	for i := 0; i < 20; i++ {
		pass(t, g)
	}
	is.Equal(g.Playing(), pb.PlayState_PLAYING)
}

// None of this leaks into other games.
func TestClassicEndingIsUnchanged(t *testing.T) {
	is := is.New(t)
	rules, err := NewBasicGameRules(DefaultConfig, "NWL23", "CrosswordGame", "english", CrossScoreAndSet, VarClassic)
	is.NoErr(err)

	newGame := func(r0, r1 string) *Game {
		g, err := NewGame(rules, crossplayPlayers)
		is.NoErr(err)
		g.StartGame()
		is.NoErr(g.SetRacksForBoth([]*tilemapping.Rack{
			tilemapping.RackFromString(r0, g.Alphabet()),
			tilemapping.RackFromString(r1, g.Alphabet()),
		}))
		n := g.Bag().TilesRemaining()
		is.NoErr(g.Bag().Draw(n, make([]tilemapping.MachineLetter, n)))
		return g
	}

	// Going out ends the game, and earns twice the other rack: O=1, X=8.
	g := newGame("AT", "OX")
	g.history.ChallengeRule = pb.ChallengeRule_VOID
	s := playTiles(t, g, "8G", "AT")
	is.Equal(g.Playing(), pb.PlayState_GAME_OVER)
	is.Equal(g.PointsFor(0), s+2*(1+8))

	// Six scoreless turns end the game, with both racks counted against their
	// owners: A=1, T=1; O=1, X=8.
	g = newGame("AT", "OX")
	for i := 0; i < DefaultMaxScorelessTurns; i++ {
		is.Equal(g.Playing(), pb.PlayState_PLAYING)
		pass(t, g)
	}
	is.Equal(g.Playing(), pb.PlayState_GAME_OVER)
	is.Equal(g.PointsFor(0), -2)
	is.Equal(g.PointsFor(1), -9)
}
