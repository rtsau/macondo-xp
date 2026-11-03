package equity

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"slices"

	"github.com/domino14/word-golib/tilemapping"
)

// LeaveEntry is one leave and its value, for WriteKLV.
type LeaveEntry struct {
	Leave tilemapping.MachineWord
	Value float32
}

const (
	kwgAccepts  = uint32(0x800000)
	kwgIsEnd    = uint32(0x400000)
	kwgArcMask  = uint32(0x3fffff)
	kwgTileBits = 24
)

// klvTrieNode is a leave graph under construction. Its children are sorted by tile.
type klvTrieNode struct {
	tile     uint8
	accepts  bool
	children []*klvTrieNode
}

// klvWriter lays out rows of sibling nodes, sharing rows that come out the same.
type klvWriter struct {
	nodes []uint32
	rows  map[string]uint32
}

func (w *klvWriter) writeRow(children []*klvTrieNode) uint32 {
	if len(children) == 0 {
		return 0
	}
	row := make([]uint32, len(children))
	for i, c := range children {
		n := uint32(c.tile)<<kwgTileBits | w.writeRow(c.children)
		if c.accepts {
			n |= kwgAccepts
		}
		row[i] = n
	}
	row[len(row)-1] |= kwgIsEnd

	key := make([]byte, 0, 4*len(row))
	for _, n := range row {
		key = binary.LittleEndian.AppendUint32(key, n)
	}
	if idx, ok := w.rows[string(key)]; ok {
		return idx
	}
	idx := uint32(len(w.nodes))
	w.nodes = append(w.nodes, row...)
	w.rows[string(key)] = idx
	return idx
}

// WriteKLV writes leaves in the KLV2 format. See ReadKLV, and the wolges project,
// where the format comes from. Each leave's tiles must be sorted. entries gets sorted
// here, as a leave's position in that order is what finds its value.
func WriteKLV(w io.Writer, entries []LeaveEntry) error {
	if len(entries) == 0 {
		return errors.New("no leaves to write")
	}
	slices.SortFunc(entries, func(a, b LeaveEntry) int { return slices.Compare(a.Leave, b.Leave) })

	root := &klvTrieNode{}
	for i, e := range entries {
		if len(e.Leave) == 0 {
			return errors.New("a leave can't be empty")
		}
		if !slices.IsSorted(e.Leave) {
			return fmt.Errorf("leave %v isn't sorted", e.Leave)
		}
		if i > 0 && slices.Equal(e.Leave, entries[i-1].Leave) {
			return fmt.Errorf("leave %v appears twice", e.Leave)
		}
		n := root
		for _, ml := range e.Leave {
			// Entries are sorted, so a tile's node is the last one if it exists.
			var child *klvTrieNode
			if len(n.children) > 0 && n.children[len(n.children)-1].tile == uint8(ml) {
				child = n.children[len(n.children)-1]
			} else {
				child = &klvTrieNode{tile: uint8(ml)}
				n.children = append(n.children, child)
			}
			n = child
		}
		n.accepts = true
	}

	// The first two nodes point to the graph of words and to the GADDAG, which
	// leaves don't have.
	kw := &klvWriter{nodes: []uint32{kwgIsEnd, kwgIsEnd}, rows: map[string]uint32{}}
	rootRow := kw.writeRow(root.children)
	kw.nodes[0] |= rootRow
	if uint32(len(kw.nodes)) > kwgArcMask {
		return fmt.Errorf("%d nodes are too many for the format", len(kw.nodes))
	}

	var buf bytes.Buffer
	buf.Grow(8 + 4*len(kw.nodes) + 4*len(entries))
	values := make([]float32, len(entries))
	for i, e := range entries {
		values[i] = e.Value
	}
	for _, v := range []any{uint32(len(kw.nodes)), kw.nodes, uint32(len(entries)), values} {
		if err := binary.Write(&buf, binary.LittleEndian, v); err != nil {
			return err
		}
	}
	_, err := w.Write(buf.Bytes())
	return err
}
