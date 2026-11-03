// crossplay_import turns a transcription of an NYT Crossplay game into a GCG
// file. The app has no export, so a game gets transcribed from its coach view;
// see scripts/crossplay/extract_frames.swift. The coach view doesn't show
// per-move scores, so this replays the moves on the engine to derive them, and
// checks the transcription along the way.
//
// Usage:
//
//	go run ./cmd/crossplay_import -turns turns.json -out game.gcg -final 437-434
//
// The final score is the app's, in first-player-first order. It's the only check
// that doesn't rest on this engine, so it's worth providing. Words that aren't in
// the lexicon are warnings, as NYT's word list differs slightly from NWL23.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/domino14/word-golib/tilemapping"
	"github.com/rs/zerolog"

	"github.com/domino14/macondo/board"
	"github.com/domino14/macondo/config"
	"github.com/domino14/macondo/game"
	"github.com/domino14/macondo/gcgio"
	pb "github.com/domino14/macondo/gen/api/proto/macondo"
	"github.com/domino14/macondo/move"
)

type tile struct {
	Sq     string `json:"sq"`
	Letter string `json:"letter"`
	Value  int    `json:"value"`
}

// The values of turn.Action. A play can leave it out.
const (
	actionPlay = "play"
	actionSwap = "swap"
	actionPass = "pass"
)

type turn struct {
	Turn     int    `json:"turn"`
	Player   string `json:"player"`
	Action   string `json:"action"`
	Word     string `json:"word"`
	Rack     string `json:"rack"`
	NewTiles []tile `json:"new_tiles"`
	Start    string `json:"start"`
	Dir      string `json:"dir"`
	// Swapped is the tiles a swap put back, as the header names them.
	Swapped string `json:"swapped"`
}

// importer holds the problems found so far, which are errors in the
// transcription or in our rules data.
type importer struct {
	problems []string
	warnings []string
}

func (im *importer) problemf(format string, args ...any) {
	im.problems = append(im.problems, fmt.Sprintf(format, args...))
}

func main() {
	zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	turnsPath := flag.String("turns", "", "path to the transcription, as JSON (required)")
	outPath := flag.String("out", "", "path of the GCG file to write (required)")
	final := flag.String("final", "", "the app's final score, first player first, like 437-434")
	lex := flag.String("lex", "NWL23", "lexicon")
	flag.Parse()
	if *turnsPath == "" || *outPath == "" {
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*turnsPath, *outPath, *final, *lex); err != nil {
		fmt.Fprintln(os.Stderr, "FAILED:", err)
		os.Exit(1)
	}
}

func run(turnsPath, outPath, final, lex string) error {
	bts, err := os.ReadFile(turnsPath)
	if err != nil {
		return err
	}
	var turns []turn
	if err := json.Unmarshal(bts, &turns); err != nil {
		return err
	}
	if len(turns) == 0 {
		return fmt.Errorf("no turns in %s", turnsPath)
	}

	cfg := config.DefaultConfig()
	if p := os.Getenv("MACONDO_DATA_PATH"); p != "" {
		cfg.Set(config.ConfigDataPath, p)
	}
	rules, err := game.NewBasicGameRules(cfg, lex, board.CrossplayLayout,
		game.CrossplayLetterDistribution, game.CrossScoreAndSet, game.VarCrossplay)
	if err != nil {
		return err
	}
	players := []*pb.PlayerInfo{
		{Nickname: "p1", RealName: "Player One"},
		{Nickname: "p2", RealName: "Player Two"},
	}
	g, err := game.NewGame(rules, players)
	if err != nil {
		return err
	}
	g.StartGame()

	im := &importer{}
	lines, err := im.replay(g, rules, turns)
	if err != nil {
		im.report()
		return err
	}

	fmt.Printf("final %d-%d, bag %d, left on racks: p1 %s, p2 %s\n", g.PointsFor(0), g.PointsFor(1),
		g.Bag().TilesRemaining(), g.RackFor(0).String(), g.RackFor(1).String())
	if g.Playing() != pb.PlayState_GAME_OVER {
		im.problemf("the game isn't over after the last turn (%d final turn(s) left)", g.FinalTurnsLeft())
	}
	if final == "" {
		im.warnings = append(im.warnings, "no -final given, so nothing checked the scores independently")
	} else if want := fmt.Sprintf("%d-%d", g.PointsFor(0), g.PointsFor(1)); want != final {
		im.problemf("derived final score is %s, but the app says %s", want, final)
	}
	if im.report() {
		return fmt.Errorf("%d problem(s), no GCG written", len(im.problems))
	}

	gcg := strings.Join(append(header(lex), lines...), "\n") + "\n"
	if _, err := gcgio.ParseGCGFromReader(cfg, strings.NewReader(gcg)); err != nil {
		return fmt.Errorf("the generated GCG doesn't parse: %w", err)
	}
	if err := os.WriteFile(outPath, []byte(gcg), 0644); err != nil {
		return err
	}
	fmt.Println("wrote", outPath)
	return nil
}

func header(lex string) []string {
	return []string{
		"#character-encoding UTF-8",
		"#description NYT Crossplay game transcribed from the coach view. Racks and moves are from the app; scores are derived by replaying them.",
		"#player1 p1 Player One",
		"#player2 p2 Player Two",
		"#lexicon " + lex,
		"#board-layout " + board.CrossplayLayout,
		"#tile-distribution " + game.CrossplayLetterDistribution,
		"#game-type " + string(game.VarCrossplay),
	}
}

// replay plays the turns on g and returns their GCG lines. An error means that
// the replay can't continue; anything else it finds goes in im.problems.
func (im *importer) replay(g *game.Game, rules *game.GameRules, turns []turn) ([]string, error) {
	ld := rules.LetterDistribution()
	alph := g.Alphabet()
	placed := map[string]string{}
	kept := map[string]map[string]int{}
	firstPlayer := turns[0].Player
	lines := make([]string, 0, len(turns))

	for i, t := range turns {
		label := fmt.Sprintf("turn %d (%s)", t.Turn, t.Word)
		if t.Turn != i+1 {
			im.problemf("%s: expected turn number %d", label, i+1)
		}
		if i > 0 && t.Player == turns[i-1].Player {
			im.problemf("%s: %s moved twice in a row", label, t.Player)
		}
		// Only a turn the app labeled as a pass or swap can place nothing, so that
		// a misread play can't turn into one here.
		switch t.Action {
		case "", actionPlay:
			if len(t.NewTiles) == 0 {
				return nil, fmt.Errorf("%s: no new tiles, but it isn't marked as a pass or swap", label)
			}
		case actionSwap, actionPass:
			if len(t.NewTiles) != 0 {
				im.problemf("%s: a %s can't place tiles", label, t.Action)
			}
		default:
			return nil, fmt.Errorf("%s: bad action %q", label, t.Action)
		}
		rack := tally(t.Rack)
		if len(t.Rack) > game.RackTileLimit {
			im.problemf("%s: rack %s has too many tiles", label, t.Rack)
		}
		for letter, n := range kept[t.Player] {
			if rack[letter] < n {
				im.problemf("%s: rack %s is missing %s, kept from this player's last turn", label, t.Rack, letter)
			}
		}

		if t.Action == actionSwap || t.Action == actionPass {
			line, err := im.playNonScoring(g, firstPlayer, label, t, kept)
			if err != nil {
				return nil, err
			}
			lines = append(lines, line)
			continue
		}

		// Check each new tile, and work out what's left on the rack.
		leave := tally(t.Rack)
		byPos := map[string]tile{}
		for _, nt := range t.NewTiles {
			mls, err := tilemapping.ToMachineLetters(nt.Letter, alph)
			if err != nil {
				return nil, fmt.Errorf("%s: bad letter %q: %w", label, nt.Letter, err)
			}
			if nt.Value != 0 && nt.Value != ld.Score(mls[0]) {
				im.problemf("%s: %s %s reads as worth %d, but our tile data says %d",
					label, nt.Sq, nt.Letter, nt.Value, ld.Score(mls[0]))
			}
			if _, ok := placed[nt.Sq]; ok {
				im.problemf("%s: %s is already occupied", label, nt.Sq)
			}
			from := nt.Letter
			if nt.Value == 0 {
				from = "?"
			}
			if leave[from] == 0 {
				im.problemf("%s: %s %s isn't on rack %s", label, nt.Sq, from, t.Rack)
			}
			leave[from]--
			byPos[nt.Sq] = nt
		}
		kept[t.Player] = leave

		position, played, err := im.playString(label, t, byPos, placed)
		if err != nil {
			return nil, err
		}

		// The player on turn is always 0 for whoever moved first.
		pidx := 1
		if t.Player == firstPlayer {
			pidx = 0
		}
		if pidx != g.PlayerOnTurn() {
			return nil, fmt.Errorf("%s: player %d isn't on turn", label, pidx+1)
		}
		if err := g.SetRackFor(pidx, tilemapping.RackFromString(t.Rack, alph)); err != nil {
			return nil, fmt.Errorf("%s: rack %s uses tiles that are already gone: %w", label, t.Rack, err)
		}
		m, err := g.CreateAndScorePlacementMove(position, played, t.Rack, false)
		if err != nil {
			return nil, fmt.Errorf("%s: %s %s: %w", label, position, played, err)
		}
		words, err := g.ValidateMove(m)
		if err != nil {
			return nil, fmt.Errorf("%s: %s %s is illegal: %w", label, position, played, err)
		}
		visible := make([]string, len(words))
		for j, w := range words {
			visible[j] = w.UserVisible(alph)
			if !g.Lexicon().HasWord(w) {
				im.warnings = append(im.warnings, fmt.Sprintf("%s: %s isn't in the lexicon (misread, or an NYT tweak?)", label, visible[j]))
			}
		}
		if err := g.PlayMove(m, false, 0); err != nil {
			return nil, fmt.Errorf("%s: %w", label, err)
		}
		for sq, nt := range byPos {
			placed[sq] = nt.Letter
		}

		fmt.Printf("%2d p%d %-7s %-4s %-9s +%-3d %3d-%-3d bag %-2d %s\n", t.Turn, pidx+1, t.Rack, position,
			played, m.Score(), g.PointsFor(0), g.PointsFor(1), g.Bag().TilesRemaining(), strings.Join(visible, ","))
		lines = append(lines, fmt.Sprintf(">p%d: %s %s %s +%s %d", pidx+1, t.Rack, position, played,
			strconv.Itoa(m.Score()), g.PointsFor(pidx)))
	}
	return lines, nil
}

// playNonScoring plays a swap or a pass and returns its GCG line.
func (im *importer) playNonScoring(g *game.Game, firstPlayer, label string, t turn, kept map[string]map[string]int) (string, error) {
	alph := g.Alphabet()
	leave := tally(t.Rack)
	if t.Action == actionSwap {
		if t.Swapped == "" {
			return "", fmt.Errorf("%s: a swap needs the tiles that were swapped", label)
		}
		for _, letter := range strings.Split(t.Swapped, "") {
			if leave[letter] == 0 {
				im.problemf("%s: swapped %s isn't on rack %s", label, letter, t.Rack)
			}
			leave[letter]--
		}
	}
	kept[t.Player] = leave

	pidx := 1
	if t.Player == firstPlayer {
		pidx = 0
	}
	if pidx != g.PlayerOnTurn() {
		return "", fmt.Errorf("%s: player %d isn't on turn", label, pidx+1)
	}
	if err := g.SetRackFor(pidx, tilemapping.RackFromString(t.Rack, alph)); err != nil {
		return "", fmt.Errorf("%s: rack %s uses tiles that are already gone: %w", label, t.Rack, err)
	}

	rack, err := tilemapping.ToMachineWord(t.Rack, alph)
	if err != nil {
		return "", fmt.Errorf("%s: bad rack %q: %w", label, t.Rack, err)
	}
	var m *move.Move
	position := "-"
	if t.Action == actionSwap {
		swapped, err := tilemapping.ToMachineWord(t.Swapped, alph)
		if err != nil {
			return "", fmt.Errorf("%s: bad swap %q: %w", label, t.Swapped, err)
		}
		kept, err := tilemapping.Leave(rack, swapped, false)
		if err != nil {
			return "", fmt.Errorf("%s: can't swap %s from %s: %w", label, t.Swapped, t.Rack, err)
		}
		m = move.NewExchangeMove(swapped, kept, alph)
		position = "-" + t.Swapped
	} else {
		m = move.NewPassMove(rack, alph)
	}
	if _, err := g.ValidateMove(m); err != nil {
		return "", fmt.Errorf("%s: %s is illegal: %w", label, t.Action, err)
	}
	if err := g.PlayMove(m, false, 0); err != nil {
		return "", fmt.Errorf("%s: %w", label, err)
	}
	fmt.Printf("%2d p%d %-7s %-14s +0   %3d-%-3d bag %d\n", t.Turn, pidx+1, t.Rack, position,
		g.PointsFor(0), g.PointsFor(1), g.Bag().TilesRemaining())
	return fmt.Sprintf(">p%d: %s %s +0 %d", pidx+1, t.Rack, position, g.PointsFor(pidx)), nil
}

// playString returns the GCG position and tiles of a play: uppercase for tiles,
// lowercase for blanks and "." for tiles already on the board. Spelling it out
// from t.Start checks the main word against the squares that were read.
func (im *importer) playString(label string, t turn, byPos map[string]tile, placed map[string]string) (string, string, error) {
	if len(t.Start) < 2 || t.Start[0] < 'A' || t.Start[0] > 'O' {
		return "", "", fmt.Errorf("%s: bad start square %q", label, t.Start)
	}
	col := int(t.Start[0] - 'A')
	row, err := strconv.Atoi(t.Start[1:])
	if err != nil {
		return "", "", fmt.Errorf("%s: bad start square %q", label, t.Start)
	}
	var position string
	switch t.Dir {
	case "across":
		position = fmt.Sprintf("%d%c", row, 'A'+col)
	case "down":
		position = t.Start
	default:
		return "", "", fmt.Errorf("%s: bad direction %q", label, t.Dir)
	}

	var played strings.Builder
	used := 0
	for i, letter := range strings.Split(t.Word, "") {
		sq := fmt.Sprintf("%c%d", 'A'+col, row+i)
		if t.Dir == "across" {
			sq = fmt.Sprintf("%c%d", 'A'+col+i, row)
		}
		if nt, ok := byPos[sq]; ok {
			used++
			switch {
			case nt.Letter != letter:
				im.problemf("%s: %s reads as %s, but the word needs %s", label, sq, nt.Letter, letter)
			case nt.Value == 0:
				letter = strings.ToLower(letter)
			}
			played.WriteString(letter)
		} else if placed[sq] == letter {
			played.WriteString(".")
		} else {
			return "", "", fmt.Errorf("%s: the word needs %s at %s, which holds %q", label, letter, sq, placed[sq])
		}
	}
	if used != len(byPos) {
		im.problemf("%s: %d new tile(s) aren't on the main word", label, len(byPos)-used)
	}
	return position, played.String(), nil
}

// report prints what was found, and returns whether there are any problems.
func (im *importer) report() bool {
	for _, w := range im.warnings {
		fmt.Println("WARNING:", w)
	}
	for _, p := range im.problems {
		fmt.Println("PROBLEM:", p)
	}
	return len(im.problems) > 0
}

func tally(rack string) map[string]int {
	counts := map[string]int{}
	for _, letter := range strings.Split(rack, "") {
		counts[letter]++
	}
	return counts
}
