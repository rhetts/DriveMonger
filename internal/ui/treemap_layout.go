package ui

import (
	"image/color"

	"github.com/rhetts/DriveMonger/internal/scan"
)

// tile is a single rectangle in a laid-out treemap, tying a node to the pixel
// region it occupies.
type tile struct {
	node       *scan.Node
	x, y, w, h float32
	fill       color.NRGBA // assigned by the renderer when drawing
	subdivided bool        // true if this tile was further split into child tiles
}

// layoutTreemap arranges nodes (already sorted by descending size) into the
// rectangle (x, y, w, h) using a recursive binary "split by half the total
// size, cut the longer edge" partition. This keeps tiles reasonably square and,
// unlike a naive slice-and-dice, preserves size ordering. Results are appended
// to out.
func layoutTreemap(nodes []*scan.Node, x, y, w, h float32, out *[]tile) {
	if len(nodes) == 0 || w <= 0 || h <= 0 {
		return
	}
	if len(nodes) == 1 {
		*out = append(*out, tile{node: nodes[0], x: x, y: y, w: w, h: h})
		return
	}

	total := scan.TotalSize(nodes)
	if total <= 0 {
		// All zero-sized; fall back to equal horizontal slices so empty dirs
		// are still visible and clickable.
		sw := w / float32(len(nodes))
		for i, n := range nodes {
			*out = append(*out, tile{node: n, x: x + float32(i)*sw, y: y, w: sw, h: h})
		}
		return
	}

	// Find the split index where the cumulative size first reaches half the
	// total. Guarantee at least one node on each side.
	half := total / 2
	var acc int64
	split := 0
	for ; split < len(nodes)-1; split++ {
		acc += nodes[split].Size
		if acc >= half {
			split++ // include the node that crossed the halfway mark
			break
		}
	}
	if split < 1 {
		split = 1
	}
	if split > len(nodes)-1 {
		split = len(nodes) - 1
	}

	frac := float32(float64(scan.TotalSize(nodes[:split])) / float64(total))

	if w >= h {
		w1 := w * frac
		layoutTreemap(nodes[:split], x, y, w1, h, out)
		layoutTreemap(nodes[split:], x+w1, y, w-w1, h, out)
	} else {
		h1 := h * frac
		layoutTreemap(nodes[:split], x, y, w, h1, out)
		layoutTreemap(nodes[split:], x, y+h1, w, h-h1, out)
	}
}
