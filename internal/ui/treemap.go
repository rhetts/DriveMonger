package ui

import (
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"

	"github.com/rhetts/DriveMonger/internal/scan"
)

// maxTiers is how many levels of nested boxes the treemap shows by default
// (e.g. 3 = folder -> subfolder -> its files).
const maxTiers = 3

// Layout constants for nested tiles (in pixels).
const (
	headerH      = 16 // strip reserved at the top of a parent tile for its label
	innerPad     = 2  // padding between a parent tile's edge and its nested children
	minNestEdge  = 14 // a parent tile needs at least this much room to be subdivided
	subLabelMinH = 16 // an un-subdivided tile needs at least this height to be labeled
	labelPad     = 3  // inset of label text from its tile's top-left corner
)

var (
	bgColor     = color.NRGBA{R: 30, G: 30, B: 34, A: 255} // empty-treemap background
	borderColor = color.NRGBA{R: 20, G: 20, B: 24, A: 255} // tile outline
	labelColor  = color.NRGBA{R: 245, G: 245, B: 245, A: 255}
)

// Treemap is a custom widget that renders a directory's children as rectangles
// whose area is proportional to disk usage. Each directory tile is subdivided
// one more level to show its own children (SpaceMonger-style nested boxes).
// Tapping a tile drills into it via the OnDrill callback.
type Treemap struct {
	widget.BaseWidget

	current *scan.Node
	tiles   []tile // last computed layout, retained for hit-testing taps

	// OnDrill is invoked when the user taps a directory tile that has children.
	OnDrill func(*scan.Node)
	// OnSelect is invoked when the user taps any tile (file or directory).
	OnSelect func(*scan.Node)
}

// NewTreemap returns an empty treemap widget.
func NewTreemap() *Treemap {
	t := &Treemap{}
	t.ExtendBaseWidget(t)
	return t
}

// SetNode sets the directory whose children are displayed and redraws.
func (t *Treemap) SetNode(n *scan.Node) {
	t.current = n
	t.Refresh()
}

// Tapped implements fyne.Tappable, dispatching the tap to the innermost tile
// under the cursor. Because nested (deeper) tiles are appended after their
// parent, iterating in reverse finds the deepest match first.
func (t *Treemap) Tapped(e *fyne.PointEvent) {
	for i := len(t.tiles) - 1; i >= 0; i-- {
		ti := t.tiles[i]
		if e.Position.X >= ti.x && e.Position.X < ti.x+ti.w &&
			e.Position.Y >= ti.y && e.Position.Y < ti.y+ti.h {
			if t.OnSelect != nil {
				t.OnSelect(ti.node)
			}
			if ti.node.IsDir && len(ti.node.Children) > 0 && t.OnDrill != nil {
				t.OnDrill(ti.node)
			}
			return
		}
	}
}

// CreateRenderer implements fyne.Widget.
func (t *Treemap) CreateRenderer() fyne.WidgetRenderer {
	return &treemapRenderer{tm: t}
}

type treemapRenderer struct {
	tm      *Treemap
	objects []fyne.CanvasObject
}

func (r *treemapRenderer) MinSize() fyne.Size { return fyne.NewSize(240, 180) }

func (r *treemapRenderer) Layout(size fyne.Size) { r.rebuild(size) }

func (r *treemapRenderer) Refresh() {
	r.rebuild(r.tm.Size())
	canvas.Refresh(r.tm)
}

func (r *treemapRenderer) Objects() []fyne.CanvasObject { return r.objects }

func (r *treemapRenderer) Destroy() {}

// rebuild recomputes the tile layout for the current node over the given size
// and regenerates the rectangles and labels. It lays out nested boxes up to
// maxTiers levels deep (current node's children, their children, and so on).
func (r *treemapRenderer) rebuild(size fyne.Size) {
	r.tm.tiles = r.tm.tiles[:0]
	r.objects = r.objects[:0]

	if r.tm.current == nil || !r.tm.current.IsDir || len(r.tm.current.Children) == 0 {
		// Nothing to show: leave a blank background.
		bg := canvas.NewRectangle(bgColor)
		bg.Resize(size)
		r.objects = append(r.objects, bg)
		return
	}

	r.place(r.tm.current.Children, 0, 0, size.Width, size.Height, 0, color.NRGBA{})

	// Tiles are appended parent-before-child, so drawing in order puts each
	// nested box on top of its parent. Labels come last, above all rectangles.
	for _, ti := range r.tm.tiles {
		r.objects = append(r.objects, rectFor(ti))
	}
	for _, ti := range r.tm.tiles {
		if obj := labelFor(ti); obj != nil {
			r.objects = append(r.objects, obj)
		}
	}
}

// place lays nodes out within the rectangle and recurses into directory tiles
// that have room, up to maxTiers deep. Each tile is appended to r.tm.tiles in
// parent-before-child order. depth is the current tier (0-based); parentFill is
// the fill of the enclosing tile, used to tint nested tiles.
func (r *treemapRenderer) place(nodes []*scan.Node, x, y, w, h float32, depth int, parentFill color.NRGBA) {
	var local []tile
	layoutTreemap(nodes, x, y, w, h, &local)

	for i := range local {
		t := local[i]
		t.depth = depth
		if depth == 0 {
			t.fill = tileColor(i)
		} else {
			t.fill = shade(parentFill, i)
		}
		// Subdivide a directory tile when another tier is allowed and it has
		// both children and enough room for a header plus nested boxes.
		t.subdivided = depth+1 < maxTiers &&
			t.node.IsDir && len(t.node.Children) > 0 &&
			t.w > 2*innerPad+minNestEdge && t.h > headerH+innerPad+minNestEdge
		r.tm.tiles = append(r.tm.tiles, t)

		if t.subdivided {
			r.place(t.node.Children,
				t.x+innerPad, t.y+headerH,
				t.w-2*innerPad, t.h-headerH-innerPad,
				depth+1, t.fill)
		}
	}
}

// rectFor builds the colored, bordered rectangle for a tile.
func rectFor(ti tile) *canvas.Rectangle {
	rect := canvas.NewRectangle(ti.fill)
	rect.StrokeColor = borderColor
	rect.StrokeWidth = 1
	rect.Move(fyne.NewPos(ti.x, ti.y))
	rect.Resize(fyne.NewSize(ti.w, ti.h))
	return rect
}

// labelFor builds the text label for a tile, or nil if the tile is too small to
// label legibly. The text is truncated to the tile width so it never spills
// over neighboring tiles.
func labelFor(ti tile) fyne.CanvasObject {
	const textSize float32 = 11
	// A subdivided tile only exposes its header strip (its body is covered by
	// nested children), so it just needs room for the header; an un-subdivided
	// tile shows in full and needs enough height for a standalone label.
	minH := float32(headerH)
	if !ti.subdivided {
		minH = subLabelMinH
	}
	if ti.w < 40 || ti.h < minH {
		return nil
	}
	label := fitText(ti.node.Name+"  "+scan.HumanSize(ti.node.Size), ti.w-2*labelPad, textSize)
	if label == "" {
		return nil
	}
	txt := canvas.NewText(label, labelColor)
	txt.TextSize = textSize
	txt.Move(fyne.NewPos(ti.x+labelPad, ti.y+1))
	return txt
}

// fitText truncates s (with an ellipsis) so it fits within w pixels at the
// given text size, using an approximate average glyph width.
func fitText(s string, w, textSize float32) string {
	maxChars := int(w / (textSize * 0.58))
	if maxChars <= 1 {
		return ""
	}
	r := []rune(s)
	if len(r) <= maxChars {
		return s
	}
	return string(r[:maxChars-1]) + "…"
}

// tileColor returns a stable, visually distinct fill color for the i-th
// top-level tile by stepping hue along the golden ratio.
func tileColor(i int) color.NRGBA {
	hue := math.Mod(float64(i)*0.61803398875, 1.0)
	r, g, b := hsvToRGB(hue, 0.55, 0.72)
	return color.NRGBA{R: r, G: g, B: b, A: 255}
}

// shade returns a lighter variant of base for a nested child tile, with a small
// per-sibling variation so adjacent children remain distinguishable while
// staying in their parent's color family.
func shade(base color.NRGBA, j int) color.NRGBA {
	f := 0.40 + 0.10*float64(j%3) // 0.40, 0.50, 0.60 lightening
	lighten := func(c uint8) uint8 {
		return uint8(float64(c) + (255-float64(c))*f)
	}
	return color.NRGBA{R: lighten(base.R), G: lighten(base.G), B: lighten(base.B), A: 255}
}

func hsvToRGB(h, s, v float64) (uint8, uint8, uint8) {
	i := math.Floor(h * 6)
	f := h*6 - i
	p := v * (1 - s)
	q := v * (1 - f*s)
	t := v * (1 - (1-f)*s)
	var r, g, b float64
	switch int(i) % 6 {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	case 5:
		r, g, b = v, p, q
	}
	return uint8(r * 255), uint8(g * 255), uint8(b * 255)
}
