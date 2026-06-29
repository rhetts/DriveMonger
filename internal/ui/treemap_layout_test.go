package ui

import (
	"testing"

	"github.com/rhetts/DriveMonger/internal/scan"
)

// TestLayoutTreemapCoversArea checks that every node gets a tile and that the
// tiles' combined area is (approximately) the full canvas area, with no tile
// spilling outside the bounds.
func TestLayoutTreemapCoversArea(t *testing.T) {
	nodes := []*scan.Node{
		{Name: "a", Size: 500},
		{Name: "b", Size: 300},
		{Name: "c", Size: 150},
		{Name: "d", Size: 50},
	}
	const w, h = 400, 200

	var tiles []tile
	layoutTreemap(nodes, 0, 0, w, h, &tiles)

	if len(tiles) != len(nodes) {
		t.Fatalf("got %d tiles, want %d", len(tiles), len(nodes))
	}

	var area float32
	for _, ti := range tiles {
		if ti.x < -0.01 || ti.y < -0.01 || ti.x+ti.w > w+0.01 || ti.y+ti.h > h+0.01 {
			t.Errorf("tile %q out of bounds: %+v", ti.node.Name, ti)
		}
		area += ti.w * ti.h
	}

	const total = float32(w * h)
	if diff := area - total; diff > 1 || diff < -1 {
		t.Errorf("tiles cover area %.1f, want ~%.1f", area, total)
	}
}

// TestLayoutTreemapProportional checks that a node twice as large gets roughly
// twice the area.
func TestLayoutTreemapProportional(t *testing.T) {
	nodes := []*scan.Node{
		{Name: "big", Size: 800},
		{Name: "small", Size: 200},
	}
	var tiles []tile
	layoutTreemap(nodes, 0, 0, 1000, 1000, &tiles)

	areaOf := func(name string) float32 {
		for _, ti := range tiles {
			if ti.node.Name == name {
				return ti.w * ti.h
			}
		}
		return -1
	}
	big, small := areaOf("big"), areaOf("small")
	if big <= 0 || small <= 0 {
		t.Fatalf("missing tiles: big=%.1f small=%.1f", big, small)
	}
	ratio := big / small
	if ratio < 3.5 || ratio > 4.5 {
		t.Errorf("area ratio big/small = %.2f, want ~4", ratio)
	}
}
