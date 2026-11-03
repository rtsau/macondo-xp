package gcgio

import (
	"path/filepath"
	"testing"

	"github.com/domino14/word-golib/tilemapping"
	"github.com/matryer/is"

	"github.com/domino14/macondo/board"
	"github.com/domino14/macondo/game"
	pb "github.com/domino14/macondo/gen/api/proto/macondo"
	"github.com/domino14/macondo/move"
)

// TestCrossplayRealGame replays a real game of NYT Crossplay, transcribed from
// the app's coach view, and recomputes every score with the engine.
//
// The racks and moves came from the app, but the app doesn't show per-move
// scores there, so the scores in the file were derived by this engine. Asserting
// them only guards against regressions. The independent evidence that our board,
// tile values and sweep bonus are right is the final score, 437-434, which the
// app reported.
//
// TODO: once the Crossplay endgame is implemented, assert that the game is over
// after the last event, with the leftover tiles below carrying no penalty.
func TestCrossplayRealGame(t *testing.T) {
	is := is.New(t)

	history, err := ParseGCG(DefaultConfig, filepath.Join("testdata", "crossplay_game1.gcg"))
	is.NoErr(err)
	is.Equal(history.Variant, string(game.VarCrossplay))
	is.Equal(history.BoardLayout, board.CrossplayLayout)
	is.Equal(history.LetterDistribution, game.CrossplayLetterDistribution)
	is.Equal(len(history.Events), 21)

	rules, err := game.NewBasicGameRules(DefaultConfig, "NWL23", board.CrossplayLayout,
		game.CrossplayLetterDistribution, game.CrossScoreAndSet, game.VarCrossplay)
	is.NoErr(err)
	g, err := game.NewGame(rules, history.Players)
	is.NoErr(err)
	g.StartGame()
	// VOID makes ValidateMove check every word formed against the lexicon.
	g.SetChallengeRule(pb.ChallengeRule_VOID)
	alph := g.Alphabet()

	// Make sure that the word check is really on: every play below relies on it.
	const controlRack = "ZQXAEIO"
	is.NoErr(g.SetRackFor(g.PlayerOnTurn(), tilemapping.RackFromString(controlRack, alph)))
	phony, err := g.CreateAndScorePlacementMove("8F", "QZX", controlRack, false)
	is.NoErr(err)
	_, err = g.ValidateMove(phony)
	is.True(err != nil)
	g.ThrowRacksIn()

	sweeps := 0
	for i, evt := range history.Events {
		is.Equal(int(evt.PlayerIndex), g.PlayerOnTurn())
		// SetRackFor fails if a tile isn't left to draw, so this also checks that
		// the game stays within the 100-tile bag.
		is.NoErr(g.SetRackFor(int(evt.PlayerIndex), tilemapping.RackFromString(evt.Rack, alph)))

		m, err := g.CreateAndScorePlacementMove(evt.Position, evt.PlayedTiles, evt.Rack, false)
		is.NoErr(err)
		_, err = g.ValidateMove(m)
		is.NoErr(err)
		if m.Score() != int(evt.Score) {
			t.Errorf("event %d (%s %s): engine scored %d, file has %d",
				i+1, evt.Position, evt.PlayedTiles, m.Score(), evt.Score)
		}
		if m.BingoPlayed() {
			sweeps++
		}
		is.NoErr(g.PlayMove(m, false, 0))
		if g.PointsFor(int(evt.PlayerIndex)) != int(evt.Cumulative) {
			t.Errorf("event %d: engine total %d, file has %d",
				i+1, g.PointsFor(int(evt.PlayerIndex)), evt.Cumulative)
		}

		// GREYEST, the 19th play, draws the last six tiles.
		switch i + 1 {
		case 18:
			is.Equal(g.Bag().TilesRemaining(), 6)
		case 19:
			is.Equal(g.Bag().TilesRemaining(), 0)
		}
	}

	is.Equal(sweeps, 5)
	// The final score that the app reported.
	is.Equal(g.PointsFor(0), 437)
	is.Equal(g.PointsFor(1), 434)
	// With these, the 94 tiles on the board account for all 100.
	is.Equal(g.RackFor(0).String(), "AIU")
	is.Equal(g.RackFor(1).String(), "RSS")

	// The parsed history should load to the same score.
	loaded, err := game.NewFromHistory(history, rules, len(history.Events))
	is.NoErr(err)
	is.Equal(loaded.PointsFor(0), 437)
	is.Equal(loaded.PointsFor(1), 434)
}

// TestCrossplayRealGameWithSwapsAndPass replays a second real game, which has two
// swaps and a pass. As in TestCrossplayRealGame, the final score is the app's and
// the only independent check. It's also real evidence for how the app treats these
// turns: each one counts as a turn, and neither changes the score.
//
// TODO: once the Crossplay endgame is implemented, assert that the game is over
// after the last event, with the leftover tiles below carrying no penalty.
func TestCrossplayRealGameWithSwapsAndPass(t *testing.T) {
	is := is.New(t)

	history, err := ParseGCG(DefaultConfig, filepath.Join("testdata", "crossplay_game2.gcg"))
	is.NoErr(err)
	is.Equal(len(history.Events), 29)

	rules, err := game.NewBasicGameRules(DefaultConfig, "NWL23", board.CrossplayLayout,
		game.CrossplayLetterDistribution, game.CrossScoreAndSet, game.VarCrossplay)
	is.NoErr(err)
	g, err := game.NewGame(rules, history.Players)
	is.NoErr(err)
	g.StartGame()
	g.SetChallengeRule(pb.ChallengeRule_VOID)
	alph := g.Alphabet()

	swaps, passes := 0, 0
	for i, evt := range history.Events {
		is.Equal(int(evt.PlayerIndex), g.PlayerOnTurn())
		is.NoErr(g.SetRackFor(int(evt.PlayerIndex), tilemapping.RackFromString(evt.Rack, alph)))
		rack, err := tilemapping.ToMachineWord(evt.Rack, alph)
		is.NoErr(err)

		var m *move.Move
		switch evt.Type {
		case pb.GameEvent_EXCHANGE:
			swaps++
			swapped, err := tilemapping.ToMachineWord(evt.Exchanged, alph)
			is.NoErr(err)
			leave, err := tilemapping.Leave(rack, swapped, false)
			is.NoErr(err)
			m = move.NewExchangeMove(swapped, leave, alph)
		case pb.GameEvent_PASS:
			passes++
			m = move.NewPassMove(rack, alph)
		default:
			m, err = g.CreateAndScorePlacementMove(evt.Position, evt.PlayedTiles, evt.Rack, false)
			is.NoErr(err)
			if m.Score() != int(evt.Score) {
				t.Errorf("event %d (%s %s): engine scored %d, file has %d",
					i+1, evt.Position, evt.PlayedTiles, m.Score(), evt.Score)
			}
		}
		_, err = g.ValidateMove(m)
		is.NoErr(err)

		before := g.PointsFor(int(evt.PlayerIndex))
		bag := g.Bag().TilesRemaining()
		is.NoErr(g.PlayMove(m, false, 0))
		if m.Action() != move.MoveTypePlay {
			is.Equal(g.PointsFor(int(evt.PlayerIndex)), before)
			is.Equal(g.Bag().TilesRemaining(), bag)
		}

		// RUG, the 27th play, plays two tiles and draws the last two. Its player
		// moved first, and moves last, with one more turn than their opponent.
		switch i + 1 {
		case 26:
			is.Equal(g.Bag().TilesRemaining(), 2)
		case 27:
			is.Equal(g.Bag().TilesRemaining(), 0)
			is.Equal(g.RackFor(0).NumTiles(), uint8(7))
		}
	}

	is.Equal(swaps, 2)
	is.Equal(passes, 1)
	// The final score that the app reported.
	is.Equal(g.PointsFor(0), 372)
	is.Equal(g.PointsFor(1), 409)
	// With these, the 95 tiles on the board account for all 100, including the
	// third blank, which was still on a rack at the end.
	is.Equal(g.RackFor(0).String(), "?LN")
	is.Equal(g.RackFor(1).String(), "DS")

	loaded, err := game.NewFromHistory(history, rules, len(history.Events))
	is.NoErr(err)
	is.Equal(loaded.PointsFor(0), 372)
	is.Equal(loaded.PointsFor(1), 409)
}
