package board

import (
	"testing"

	"github.com/matryer/is"
)

func TestCrossplayBoardLayout(t *testing.T) {
	is := is.New(t)
	is.Equal(len(CrossplayBoard), 15)

	counts := map[BonusSquare]int{}
	for r, row := range CrossplayBoard {
		is.Equal(len(row), 15)
		for c := range row {
			counts[BonusSquare(row[c])]++
			// The board looks the same when mirrored or transposed.
			is.Equal(row[c], CrossplayBoard[c][r])
			is.Equal(row[c], row[14-c])
			is.Equal(row[c], CrossplayBoard[14-r][c])
		}
	}
	is.Equal(counts[Bonus2LS], 20)
	is.Equal(counts[Bonus3LS], 20)
	is.Equal(counts[Bonus2WS], 8)
	is.Equal(counts[Bonus3WS], 8)

	b := MakeBoard(CrossplayBoard)
	is.Equal(b.GetBonus(7, 7), NoBonus) // no bonus for the first play
	is.Equal(b.GetBonus(0, 0), Bonus3LS)
	is.Equal(b.GetBonus(0, 3), Bonus3WS)
	is.Equal(b.GetBonus(3, 0), Bonus3WS)
}

// Every game gets its board by copying the one in its rules, and solvers restore
// from copies, so a copy that lost the bonus would quietly score 50 again.
func TestBingoBonusSurvivesCopies(t *testing.T) {
	is := is.New(t)
	b := MakeBoard(CrossplayBoard)
	is.Equal(b.BingoBonus(), DefaultBingoBonus)
	is.Equal(MakeBoard(CrosswordGameBoard).BingoBonus(), 50)

	b.SetBingoBonus(40)
	is.Equal(b.Copy().BingoBonus(), 40)

	other := MakeBoard(CrossplayBoard)
	other.CopyFrom(b)
	is.Equal(other.BingoBonus(), 40)
}

// Test both values, since false is what a copy would get by forgetting it.
func TestLeftoverRacksScoreSurvivesCopies(t *testing.T) {
	is := is.New(t)
	for _, scores := range []bool{true, false} {
		b := MakeBoard(CrosswordGameBoard)
		is.Equal(b.LeftoverRacksScore(), true)
		b.SetLeftoverRacksScore(scores)
		is.Equal(b.Copy().LeftoverRacksScore(), scores)

		other := MakeBoard(CrosswordGameBoard)
		other.SetLeftoverRacksScore(!scores)
		other.CopyFrom(b)
		is.Equal(other.LeftoverRacksScore(), scores)
	}
}
