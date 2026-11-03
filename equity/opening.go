package equity

import (
	"github.com/domino14/word-golib/tilemapping"

	"github.com/domino14/macondo/board"
	"github.com/domino14/macondo/move"
)

// OpeningAdjustmentCalculator returns an equity adjustment for an opening play.
// It should only be used if the board is empty.
type OpeningAdjustmentCalculator struct{}

func (oac OpeningAdjustmentCalculator) Equity(play *move.Move, board *board.GameBoard,
	bag *tilemapping.Bag, oppRack *tilemapping.Rack) float64 {

	if !board.IsEmpty() {
		return 0.0
	}
	return placementAdjustment(play, board, bag.LetterDistribution())
}

func (oac OpeningAdjustmentCalculator) Type() string {
	return "OpeningAdjustmentCalculator"
}

// exposes2LS reports whether a tile at position j along the center line of an
// empty board sits right next to a 2LS, on the row or column beside that line.
func exposes2LS(b *board.GameBoard, j int, vertical bool) bool {
	center := b.Dim() / 2
	if vertical {
		return b.GetBonus(j, center-1) == board.Bonus2LS || b.GetBonus(j, center+1) == board.Bonus2LS
	}
	return b.GetBonus(center-1, j) == board.Bonus2LS || b.GetBonus(center+1, j) == board.Bonus2LS
}

func placementAdjustment(play *move.Move, board *board.GameBoard, ld *tilemapping.LetterDistribution) float64 {
	// Very simply just checks how many vowels are overlapping bonus squares.
	// This only gets considered when the board is empty.
	if play.Action() != move.MoveTypePlay {
		return 0
	}
	row, col, vertical := play.CoordsAndVertical()
	var start, end int
	start = col
	if vertical {
		start = row
	}
	end = start + play.TilesPlayed()

	j := start
	penalty := 0.0
	vPenalty := -0.7 // VERY ROUGH approximation from Maven paper.
	for j < end {
		if play.Tiles()[j-start].IsVowel(ld) && exposes2LS(board, j, vertical) {
			penalty += vPenalty
		}
		j++
	}
	return penalty
}
