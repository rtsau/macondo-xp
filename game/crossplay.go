package game

import (
	pb "github.com/domino14/macondo/gen/api/proto/macondo"
)

// FinalTurnsAfterBagEmptied is how many turns are left in a Crossplay game after
// a play empties the bag: the opponent's, and then the last drawer's.
const FinalTurnsAfterBagEmptied = 2

// FinalTurnsLeft is how many turns are left in a Crossplay game whose bag is
// empty. It's 0 before the bag is empty, and once the game is over.
func (g *Game) FinalTurnsLeft() int {
	return g.finalTurns
}

// SetFinalTurnsLeft sets how many turns are left in a Crossplay game, for a
// position that didn't get there by being played. With none left, the game is over.
func (g *Game) SetFinalTurnsLeft(n int) {
	g.finalTurns = n
	if n == 0 {
		g.playing = pb.PlayState_GAME_OVER
		if g.history != nil {
			g.history.PlayState = g.playing
		}
	}
}

// InferFinalTurns works out the final turns left for a Crossplay position with
// an empty bag and no record of who emptied it. The opponent of the last drawer
// always holds seven tiles, as they refilled from a bag that still had tiles
// after that. So a shorter rack on turn is the last drawer's. When both racks are
// full, either could be, and this assumes the bag was just emptied; a position
// that knows better should say so with SetFinalTurnsLeft.
func (g *Game) InferFinalTurns() {
	if !g.crossplay || g.finalTurns > 0 || g.bag.TilesRemaining() > 0 || g.playing != pb.PlayState_PLAYING {
		return
	}
	g.finalTurns = FinalTurnsAfterBagEmptied
	if int(g.RackFor(g.onturn).NumTiles()) < RackTileLimit {
		g.finalTurns = 1
	}
}

// updateFinalTurns counts the turn just played, and reports whether it was the
// last one. It's the whole of how a Crossplay game ends: once the bag is empty
// each player gets one more turn, the last drawer going last, whatever they do
// with it. Neither an empty rack nor a pass ends the game any sooner.
func (g *Game) updateFinalTurns(bagBefore int) bool {
	switch {
	case g.finalTurns > 0:
		g.finalTurns--
		return g.finalTurns == 0
	case g.bag.TilesRemaining() > 0:
		// Only bots and searches have a limit here. In the app, it's unlimited.
		return g.maxScorelessTurns != NoScorelessLimit && g.scorelessTurns >= g.maxScorelessTurns
	case bagBefore > 0:
		g.finalTurns = FinalTurnsAfterBagEmptied
		return false
	default:
		// The bag was already empty, with no record of it, so assume as
		// InferFinalTurns does for full racks: that was the opponent's turn.
		g.finalTurns = 1
		return false
	}
}

func (g *Game) finishCrossplayTurn(bagBefore int, addToHistory bool) {
	if g.updateFinalTurns(bagBefore) {
		g.playing = pb.PlayState_GAME_OVER
		if addToHistory {
			g.history.PlayState = g.playing
			g.AddFinalScoresToHistory()
		}
	}
	g.onturn = (g.onturn + 1) % len(g.players)
}

// replayCrossplayTurn is finishCrossplayTurn for PlayTurn, which replays history
// and doesn't move on to the next player itself.
func (g *Game) replayCrossplayTurn(bagBefore int) {
	if g.updateFinalTurns(bagBefore) {
		g.playing = pb.PlayState_GAME_OVER
		g.history.PlayState = g.playing
		g.AddFinalScoresToHistory()
	}
}

// setCrossplayEndgameMode makes two passes in a row end the game while searching,
// as there's nothing to find in a longer run of them, without any rack penalty.
// In a real game, passes while the bag has tiles are unlimited.
func (g *Game) setCrossplayEndgameMode(m bool) {
	if m {
		g.sturnsBackup = g.scorelessTurns
		g.maxScorelessBackup = g.maxScorelessTurns
		g.scorelessTurns = min(g.scorelessTurns, 1)
		g.maxScorelessTurns = 2
	} else {
		g.scorelessTurns = g.sturnsBackup
		g.maxScorelessTurns = g.maxScorelessBackup
	}
}
