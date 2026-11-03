package equity

import (
	"bytes"
	"testing"

	"github.com/domino14/word-golib/tilemapping"
	"github.com/matryer/is"

	"github.com/domino14/macondo/config"
	"github.com/domino14/macondo/dataloaders"
)

// allLeaves walks a table's graph, which gives its leaves in the order of its values.
func allLeaves(k *KLV) []tilemapping.MachineWord {
	var out []tilemapping.MachineWord
	var walk func(node uint32, prefix tilemapping.MachineWord)
	walk = func(node uint32, prefix tilemapping.MachineWord) {
		for node != 0 {
			word := append(prefix[:len(prefix):len(prefix)], tilemapping.MachineLetter(k.kwg.Tile(node)))
			if k.kwg.Accepts(node) {
				out = append(out, word)
			}
			walk(k.kwg.ArcIndex(node), word)
			if k.kwg.IsEnd(node) {
				return
			}
			node++
		}
	}
	walk(k.kwg.ArcIndex(0), nil)
	return out
}

// The real test: rebuild a table made by wolges, and get the same answer for every
// leave, by every way the engine reads one.
func TestWriteKLVReproducesRealTable(t *testing.T) {
	is := is.New(t)
	cfg := config.DefaultConfig().WGLConfig()
	file, err := dataloaders.LeavesFileForLexicon(cfg, "", "NWL23")
	is.NoErr(err)
	defer file.Close()
	orig, err := ReadKLV(file)
	is.NoErr(err)

	leaves := allLeaves(orig)
	is.Equal(len(leaves), len(orig.leaveValues))
	is.True(len(leaves) > 900000)
	entries := make([]LeaveEntry, len(leaves))
	for i, l := range leaves {
		entries[i] = LeaveEntry{Leave: l, Value: float32(orig.leaveValues[i])}
	}

	var buf bytes.Buffer
	is.NoErr(WriteKLV(&buf, entries))
	rebuilt, err := ReadKLV(bytes.NewReader(buf.Bytes()))
	is.NoErr(err)
	t.Logf("%d leaves: %d nodes in the original, %d rebuilt, %d bytes",
		len(leaves), len(orig.kwg.Nodes()), len(rebuilt.kwg.Nodes()), buf.Len())

	is.Equal(len(rebuilt.leaveValues), len(orig.leaveValues))
	for i, l := range leaves {
		is.Equal(rebuilt.LeaveValue(append(tilemapping.MachineWord{}, l...)), orig.leaveValues[i])
		is.Equal(rebuilt.kwg.GetWordIndexOf(rebuilt.kwg.ArcIndex(0), l), int32(i))
	}
	// Leaves that aren't in the table are still not in it.
	is.Equal(rebuilt.LeaveValue(tilemapping.MachineWord{0, 0, 0}), 0.0)
}

func TestWriteKLVSmall(t *testing.T) {
	is := is.New(t)
	entries := []LeaveEntry{
		{tilemapping.MachineWord{1, 2}, 1.5},
		{tilemapping.MachineWord{0, 0, 0}, 40},
		{tilemapping.MachineWord{1}, -2},
		{tilemapping.MachineWord{0}, 25},
		{tilemapping.MachineWord{2}, 3},
	}
	var buf bytes.Buffer
	is.NoErr(WriteKLV(&buf, entries))
	k, err := ReadKLV(bytes.NewReader(buf.Bytes()))
	is.NoErr(err)
	for _, tc := range []struct {
		leave tilemapping.MachineWord
		want  float64
	}{
		{tilemapping.MachineWord{0}, 25},
		{tilemapping.MachineWord{0, 0, 0}, 40},
		{tilemapping.MachineWord{1}, -2},
		{tilemapping.MachineWord{2, 1}, 1.5}, // looking one up sorts it
		{tilemapping.MachineWord{2}, 3},
		{tilemapping.MachineWord{0, 0}, 0}, // a prefix of a leave isn't one
		{tilemapping.MachineWord{3}, 0},
	} {
		is.Equal(k.LeaveValue(tc.leave), tc.want)
	}

	is.True(WriteKLV(&buf, nil) != nil)
	is.True(WriteKLV(&buf, []LeaveEntry{{tilemapping.MachineWord{2, 1}, 1}}) != nil)
	is.True(WriteKLV(&buf, []LeaveEntry{{tilemapping.MachineWord{1}, 1}, {tilemapping.MachineWord{1}, 2}}) != nil)
}
