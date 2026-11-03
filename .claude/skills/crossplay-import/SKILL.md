---
name: crossplay-import
description: Import a real NYT Crossplay game into a GCG file from a phone screen recording of the app's coach view. Use when the user has a Crossplay recording to transcribe, or wants a new real game as a test fixture.
---

# Import a Crossplay game from a recording

NYT Crossplay has no export. The coach view shows each turn's rack and move, so a
game gets transcribed from a recording of it. Reading frames is cheap, careful
work, so a Haiku subagent does it; the importer then checks what it read.

You need two things from the user:

- **The recording:** made on the phone (not the mirrored window), stepping from
  turn 1 to the Conclusion screen and pausing about a second on each turn.
- **The final score:** first player first. The coach view doesn't show it, and
  it's the only check that doesn't rest on our engine.

Macondo needs `MACONDO_DATA_PATH=$PWD/data` in tool shells. Put frames and the
transcription in the scratchpad, not the repo: frames show the opponent's name.

## 1. Extract the frames

```
swift -O scripts/crossplay/extract_frames.swift <video> <frames dir>
```

Read just one frame yourself. Check that the pink grid lines up with the tiles and
that a turn chip shows at the bottom. If the script warned about the video size
and the grid is off, fix the crop positions in `annotate` before going on.

Expect one image per turn, plus any screens without a board.

## 2. Have Haiku read them

Spawn one Agent with `model: "haiku"` and this prompt, with the paths filled in:

> Transcribe a game of the crossword game Crossplay from images. This is careful
> reading work: report what the images show, and don't guess or "correct" anything.
>
> Read every image in <frames dir> in order with the Read tool. Skip any without a
> board. Each of the others shows one turn:
> - Header: "You played X" means player "me", and "They played X" means player
>   "opp". The line above it varies (key turns say "KEY TURN 2 OF 4"), so ignore it.
> - Board: 15x15, with pink axis labels added: columns A-O across the top, rows 1-15
>   down the left, and a faint pink grid. Read tile squares using the grid lines,
>   not by estimating.
> - This turn's new tiles are dark blue with a green outline. The outline can also
>   enclose lighter blue tiles that were already on the board; those are NOT new.
> - Tray below the board: the tiles held BEFORE this move, in the displayed order. A
>   tile with no letter is a blank: write it as "?".
> - Each tile has a small value at its top right. A letter whose value is 0 is a
>   blank played as that letter.
> - Bottom strip: the selected chip gives the turn number and word.
>
> For each turn, record:
> - turn (number), player ("me" or "opp"), word (the full main word, including tiles
>   already on the board)
> - rack: the tray, left to right, as one string
> - new_tiles: every new tile, as {"sq":"H3","letter":"R","value":0}
> - start: the square of the first letter of the full main word, and dir: "across"
>   or "down"
> - note: anything you couldn't read confidently, otherwise ""
>
> If a turn is a pass or swap, leave new_tiles empty and describe what the screen
> shows in note.
>
> Write a JSON array to <turns.json>. Touch nothing else, and don't touch the
> repository. In your final message, confirm the file was written and list any turns
> with a non-empty note.

## 3. Check it and write the GCG

```
go run ./cmd/crossplay_import -turns <turns.json> -out <game.gcg> -final <first>-<second>
```

This replays the moves on the engine, derives the scores, and writes the GCG only
if nothing's wrong. It checks turn order, tile values, racks against the tiles kept
from the previous turn, each move against the board, and the final score.

If it reports a problem, read only the frames for the turns it names, fix
`turns.json` yourself, and rerun. A mismatch that survives a correct reading is the
interesting case: it may mean that our board or tile data is wrong, so report it
rather than adjusting anything to make it pass.

- Words not in NWL23 are warnings, as they may be NYT's tweaks. Report them.
- Passes and swaps stop the import. We haven't seen how the app shows them, so ask
  the user for that frame and extend the importer.

## 4. Report

Give the user the derived and actual final scores, the warnings, and the GCG's path.
To use it as a fixture, copy it to `gcgio/testdata/`. `*.gcg` is gitignored, so it
needs `git add -f` when the user commits it. Don't stage or commit anything yourself.
