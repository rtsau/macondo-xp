// Package finalturns solves the endgame of NYT Crossplay. Once the bag is empty,
// each player gets exactly one more turn, the last drawer going last, and nothing
// else affects the score: neither going out nor the tiles left on a rack. So the
// last turn is just the highest-scoring play, and the turn before it is the play
// that most outscores the reply it leaves.
package finalturns

import (
	"context"
	"errors"
	"fmt"

	"github.com/domino14/macondo/equity"
	"github.com/domino14/macondo/gaddag"
	"github.com/domino14/macondo/game"
	pb "github.com/domino14/macondo/gen/api/proto/macondo"
	"github.com/domino14/macondo/move"
	"github.com/domino14/macondo/movegen"
)

// Solve finds the best line of play for the player on turn. It returns the change
// in that player's spread over the rest of the game, and the moves that make it.
// Since nothing is hidden and nothing is left to chance, the line with the best
// spread is also the one most likely to win. g isn't changed.
func Solve(ctx context.Context, g *game.Game, gd gaddag.WordGraph) (int, []*move.Move, error) {
	if !game.IsCrossplay(g.Rules().Variant()) {
		return 0, nil, errors.New("finalturns only solves Crossplay endgames")
	}
	if g.Playing() != pb.PlayState_PLAYING {
		return 0, nil, errors.New("the game is over")
	}
	gc := g.Copy()
	gc.InferFinalTurns()
	if gc.FinalTurnsLeft() == 0 {
		return 0, nil, fmt.Errorf("the bag isn't empty: it has %d tile(s)", gc.Bag().TilesRemaining())
	}

	ld := gc.Bag().LetterDistribution()
	best := movegen.NewGordonGenerator(gd, gc.Board(), ld)
	best.SetGame(gc)
	best.SetEquityCalculators([]equity.EquityCalculator{equity.NewNoLeaveCalculator()})
	best.SetPlayRecorderTopPlay()

	if gc.FinalTurnsLeft() == 1 {
		m := copyMove(bestPlay(best, gc))
		return m.Score(), []*move.Move{m}, nil
	}

	all := movegen.NewGordonGenerator(gd, gc.Board(), ld)
	all.SetGenPass(true)
	plays := all.GenAll(gc.RackFor(gc.PlayerOnTurn()), false)

	gc.SetBackupMode(game.SimulationMode)
	gc.SetStateStackLength(1)

	var bestMove, bestReply *move.Move
	bestValue := 0
	for _, m := range plays {
		if err := ctx.Err(); err != nil {
			return 0, nil, err
		}
		if err := gc.PlayMove(m, false, 0); err != nil {
			return 0, nil, err
		}
		reply := bestPlay(best, gc)
		value := m.Score() - reply.Score()
		// Between equal lines, score more now. It's only for a consistent answer.
		if bestMove == nil || value > bestValue || (value == bestValue && m.Score() > bestMove.Score()) {
			bestMove, bestValue = m, value
			bestReply = copyMove(reply)
		}
		gc.UnplayLastMove()
	}
	return bestValue, []*move.Move{copyMove(bestMove), bestReply}, nil
}

// bestPlay is the highest-scoring play for the player on turn, or a pass without
// one. It belongs to the generator, so copy it to keep it.
func bestPlay(gen *movegen.GordonGenerator, g *game.Game) *move.Move {
	return gen.GenAll(g.RackFor(g.PlayerOnTurn()), false)[0]
}

func copyMove(m *move.Move) *move.Move {
	c := &move.Move{}
	c.CopyFrom(m)
	return c
}
