package finalturns

import (
	"context"
	"testing"
	"time"

	"github.com/domino14/word-golib/kwg"
	"github.com/matryer/is"

	"github.com/domino14/macondo/board"
	"github.com/domino14/macondo/cgp"
	"github.com/domino14/macondo/config"
	"github.com/domino14/macondo/endgame/negamax"
	"github.com/domino14/macondo/game"
	pb "github.com/domino14/macondo/gen/api/proto/macondo"
	"github.com/domino14/macondo/move"
	"github.com/domino14/macondo/movegen"
	"github.com/domino14/macondo/preendgame"
	"github.com/domino14/macondo/tinymove"
)

type tinySmallMove = tinymove.SmallMove

var DefaultConfig = config.DefaultConfig()

func nwl23(t *testing.T) *kwg.KWG {
	t.Helper()
	gd, err := kwg.GetKWG(DefaultConfig.WGLConfig(), "NWL23")
	is.New(t).NoErr(err)
	return gd
}

func newGame(t *testing.T, seed byte) *game.Game {
	t.Helper()
	is := is.New(t)
	rules, err := game.NewBasicGameRules(DefaultConfig, "NWL23", "", "", game.CrossScoreAndSet, game.VarCrossplay)
	is.NoErr(err)
	g, err := game.NewGame(rules, []*pb.PlayerInfo{{Nickname: "a"}, {Nickname: "b"}})
	is.NoErr(err)
	g.SeedBag([32]byte{seed})
	g.StartGame()
	return g
}

func allPlays(gd *kwg.KWG, g *game.Game) []*move.Move {
	gen := movegen.NewGordonGenerator(gd, g.Board(), g.Bag().LetterDistribution())
	gen.SetGenPass(true)
	return gen.GenAll(g.RackFor(g.PlayerOnTurn()), false)
}

func maxScore(plays []*move.Move) int {
	best := 0
	for _, m := range plays {
		best = max(best, m.Score())
	}
	return best
}

// bruteForce finds what Solve should, the slow way: generating every reply in full
// rather than using shadow play's top-play search, and playing on separate copies.
func bruteForce(t *testing.T, gd *kwg.KWG, g *game.Game) int {
	t.Helper()
	if g.FinalTurnsLeft() == 1 {
		return maxScore(allPlays(gd, g))
	}
	best := -100000
	for _, m := range allPlays(gd, g) {
		c := g.Copy()
		is.New(t).NoErr(c.PlayMove(m, false, 0))
		best = max(best, m.Score()-maxScore(allPlays(gd, c)))
	}
	return best
}

// playToEmptyBag plays a game by the highest score until its bag is empty, leaving
// it at the first final turn. It returns how many tiles the last play used, and how
// many the bag held before it.
func playToEmptyBag(t *testing.T, gd *kwg.KWG, g *game.Game) (lastPlayed, bagBefore int) {
	t.Helper()
	is := is.New(t)
	for g.Bag().TilesRemaining() > 0 {
		is.Equal(g.FinalTurnsLeft(), 0)
		is.Equal(g.Playing(), pb.PlayState_PLAYING)
		var best *move.Move
		for _, m := range allPlays(gd, g) {
			if best == nil || m.Score() > best.Score() {
				best = m
			}
		}
		lastPlayed, bagBefore = best.TilesPlayed(), g.Bag().TilesRemaining()
		is.NoErr(g.PlayMove(best, false, 0))
	}
	return lastPlayed, bagBefore
}

func TestSolveAgainstBruteForce(t *testing.T) {
	is := is.New(t)
	gd := nwl23(t)
	ctx := context.Background()

	var beatGreedy, n int
	var slowest time.Duration
	for seed := byte(1); seed <= 25; seed++ {
		g := newGame(t, seed)
		lastPlayed, bagBefore := playToEmptyBag(t, gd, g)

		// What the rules say about how the racks stand, in games nobody arranged.
		is.Equal(g.FinalTurnsLeft(), game.FinalTurnsAfterBagEmptied)
		onturn := g.PlayerOnTurn()
		is.Equal(int(g.RackFor(onturn).NumTiles()), game.RackTileLimit)
		is.Equal(int(g.RackFor(1-onturn).NumTiles()), game.RackTileLimit-(lastPlayed-bagBefore))

		for _, turns := range []int{2, 1} {
			before := g.ToCGP(false)
			start := time.Now()
			val, seq, err := Solve(ctx, g, gd)
			is.NoErr(err)
			slowest = max(slowest, time.Since(start))
			n++
			is.Equal(g.ToCGP(false), before) // the game isn't touched
			is.Equal(len(seq), turns)
			is.Equal(val, bruteForce(t, gd, g))

			// Playing the line out gives the value claimed, and ends the game.
			c := g.Copy()
			spread := c.SpreadFor(onturn)
			for _, m := range seq {
				is.NoErr(c.PlayMove(m, false, 0))
			}
			if turns == 2 {
				is.Equal(c.Playing(), pb.PlayState_GAME_OVER)
				is.Equal(c.SpreadFor(onturn)-spread, val)

				// Does looking at the reply matter? Compare the highest-scoring play.
				var greedy *move.Move
				for _, m := range allPlays(gd, g) {
					if greedy == nil || m.Score() > greedy.Score() {
						greedy = m
					}
				}
				gc := g.Copy()
				is.NoErr(gc.PlayMove(greedy, false, 0))
				greedyVal := greedy.Score() - maxScore(allPlays(gd, gc))
				is.True(val >= greedyVal)
				if val > greedyVal {
					beatGreedy++
				}
			}
			// Now on to the last turn: the opponent's highest score.
			if turns == 2 {
				var greedy *move.Move
				for _, m := range allPlays(gd, g) {
					if greedy == nil || m.Score() > greedy.Score() {
						greedy = m
					}
				}
				is.NoErr(g.PlayMove(greedy, false, 0))
				is.Equal(g.FinalTurnsLeft(), 1)
			}
		}
	}
	t.Logf("%d solves, slowest %v; better than the highest score in %d of 25 positions", n, slowest, beatGreedy)
	// If this never happened, this test wouldn't show that the search works.
	is.True(beatGreedy > 0)
}

// Whole games, with the score checked against the moves alone. The solver isn't
// involved, so this is about the game engine under real play.
func TestSelfPlayedGamesEndByTheRules(t *testing.T) {
	is := is.New(t)
	gd := nwl23(t)
	for seed := byte(101); seed <= 130; seed++ {
		g := newGame(t, seed)
		scores := [2]int{}
		turnsAfterEmpty := 0
		for g.Playing() == pb.PlayState_PLAYING {
			var best *move.Move
			for _, m := range allPlays(gd, g) {
				if best == nil || m.Score() > best.Score() {
					best = m
				}
			}
			if g.Bag().TilesRemaining() == 0 {
				turnsAfterEmpty++
			}
			scores[g.PlayerOnTurn()] += best.Score()
			is.NoErr(g.PlayMove(best, false, 0))
		}
		is.Equal(turnsAfterEmpty, game.FinalTurnsAfterBagEmptied)
		is.Equal(g.PointsFor(0), scores[0])
		is.Equal(g.PointsFor(1), scores[1])
	}
}

// Replaying a game's history has to arrive at the same place as playing it did,
// turn by turn, with nothing arranged by hand.
func TestReplayMatchesPlay(t *testing.T) {
	is := is.New(t)
	gd := nwl23(t)
	for seed := byte(201); seed <= 215; seed++ {
		g := newGame(t, seed)
		g.SetChallengeRule(pb.ChallengeRule_VOID)
		type state struct {
			final, p0, p1 int
			playing       pb.PlayState
		}
		states := []state{{0, 0, 0, pb.PlayState_PLAYING}}
		for g.Playing() == pb.PlayState_PLAYING {
			var best *move.Move
			for _, m := range allPlays(gd, g) {
				if best == nil || m.Score() > best.Score() {
					best = m
				}
			}
			is.NoErr(g.PlayMove(best, true, 0))
			states = append(states, state{g.FinalTurnsLeft(), g.PointsFor(0), g.PointsFor(1), g.Playing()})
		}
		is.Equal(g.History().PlayState, pb.PlayState_GAME_OVER)
		is.Equal(len(g.History().Events), len(states)-1) // no events for leftover tiles
		is.Equal(int(g.History().FinalScores[0]), g.PointsFor(0))
		is.Equal(int(g.History().FinalScores[1]), g.PointsFor(1))

		for turn, want := range states {
			r, err := game.NewFromHistory(g.History(), g.Rules(), turn)
			is.NoErr(err)
			is.Equal(state{r.FinalTurnsLeft(), r.PointsFor(0), r.PointsFor(1), r.Playing()}, want)
		}
		// Going back from the end works too.
		is.NoErr(g.PlayToTurn(len(states) - 2))
		is.Equal(g.Playing(), pb.PlayState_PLAYING)
		is.Equal(g.FinalTurnsLeft(), 1)
	}
}

// The moves that solvers use: small ones, with and without drawing tiles.
func TestSmallMovesFollowTheRules(t *testing.T) {
	is := is.New(t)
	gd := nwl23(t)
	g := newGame(t, 7)
	// Play until a single move can take what's in the bag.
	for g.Bag().TilesRemaining() > 2 {
		best := allPlays(gd, g)[0]
		is.NoErr(g.PlayMove(best, false, 0))
	}
	is.Equal(g.Playing(), pb.PlayState_PLAYING)
	g.SetBackupMode(game.SimulationMode)
	g.SetStateStackLength(5)

	small := func() []tinySmallMove {
		gen := movegen.NewGordonGenerator(gd, g.Board(), g.Bag().LetterDistribution())
		gen.SetPlayRecorder(movegen.AllPlaysSmallRecorder)
		gen.SetGenPass(true)
		gen.GenAll(g.RackFor(g.PlayerOnTurn()), false)
		return gen.SmallPlays()
	}
	var emptying *tinySmallMove
	plays := small()
	for i := range plays {
		if !plays[i].IsPass() && int(plays[i].TilesPlayed()) >= g.Bag().TilesRemaining() {
			emptying = &plays[i]
			break
		}
	}
	is.True(emptying != nil)
	is.True(int(emptying.TilesPlayed()) < game.RackTileLimit)

	before := g.ToCGP(false)
	_, err := g.PlaySmallMoveWithDraw(emptying)
	is.NoErr(err)
	is.Equal(g.Bag().TilesRemaining(), 0)
	is.Equal(g.FinalTurnsLeft(), game.FinalTurnsAfterBagEmptied)

	first := small()[0]
	_, err = g.PlaySmallMove(&first)
	is.NoErr(err)
	is.Equal(g.FinalTurnsLeft(), 1)
	is.Equal(g.Playing(), pb.PlayState_PLAYING)

	second := small()[0]
	_, err = g.PlaySmallMove(&second)
	is.NoErr(err)
	is.Equal(g.FinalTurnsLeft(), 0)
	is.Equal(g.Playing(), pb.PlayState_GAME_OVER)

	g.UnplayLastMove()
	is.Equal(g.FinalTurnsLeft(), 1)
	is.Equal(g.Playing(), pb.PlayState_PLAYING)
	g.UnplayLastMove()
	is.Equal(g.FinalTurnsLeft(), game.FinalTurnsAfterBagEmptied)
	g.UnplayLastMove()
	is.Equal(g.FinalTurnsLeft(), 0)
	is.Equal(g.ToCGP(false), before)
}

// JUKEBOX is on the board and the last drawer has one tile with nowhere to go.
const stuckCGP = "15/15/15/15/15/15/15/3JUKEBOX5/15/15/15/15/15/15/15 Q/AEINRST 40/0 0 lex NWL23; var crossplay; ftl 1;"

func TestLastTurnWithNoPlayIsAPass(t *testing.T) {
	is := is.New(t)
	gd := nwl23(t)
	parsed, err := cgp.ParseCGP(DefaultConfig, stuckCGP)
	is.NoErr(err)
	val, seq, err := Solve(context.Background(), parsed.Game, gd)
	is.NoErr(err)
	is.Equal(val, 0)
	is.Equal(len(seq), 1)
	is.Equal(seq[0].Action(), move.MoveTypePass)
}

func TestSolveRefusesWhatItCannotSolve(t *testing.T) {
	is := is.New(t)
	gd := nwl23(t)
	ctx := context.Background()

	_, _, err := Solve(ctx, newGame(t, 3), gd)
	is.True(err != nil) // the bag has tiles

	rules, err := game.NewBasicGameRules(DefaultConfig, "NWL23", board.CrosswordGameLayout, "english", game.CrossScoreAndSet, game.VarClassic)
	is.NoErr(err)
	classic, err := game.NewGame(rules, []*pb.PlayerInfo{{Nickname: "a"}, {Nickname: "b"}})
	is.NoErr(err)
	classic.StartGame()
	_, _, err = Solve(ctx, classic, gd)
	is.True(err != nil)

	over, err := cgp.ParseCGP(DefaultConfig, "15/15/15/15/15/15/15/3JUKEBOX5/15/15/15/15/15/15/15 AE/IN 40/0 0 lex NWL23; var crossplay; ftl 0;")
	is.NoErr(err)
	_, _, err = Solve(ctx, over.Game, gd)
	is.True(err != nil)

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	g := newGame(t, 4)
	playToEmptyBag(t, gd, g)
	_, _, err = Solve(cancelled, g, gd)
	is.Equal(err, context.Canceled)
}

// The solvers for other rules would give confident, wrong answers here.
func TestOtherSolversRefuseCrossplay(t *testing.T) {
	is := is.New(t)
	gd := nwl23(t)
	g := newGame(t, 5)
	playToEmptyBag(t, gd, g)

	s := &negamax.Solver{}
	g.SetBackupMode(game.SimulationMode)
	g.SetStateStackLength(10)
	gen := movegen.NewGordonGenerator(gd, g.Board(), g.Bag().LetterDistribution())
	is.NoErr(s.Init(gen, g))
	_, _, err := s.Solve(context.Background(), 2)
	is.Equal(err, negamax.ErrCrossplay)
	_, _, err = s.QuickAndDirtySolve(context.Background(), 2, 0)
	is.Equal(err, negamax.ErrCrossplay)

	peg := &preendgame.Solver{}
	is.NoErr(peg.Init(g, gd))
	_, err = peg.Solve(context.Background())
	is.Equal(err, preendgame.ErrCrossplay)
}
