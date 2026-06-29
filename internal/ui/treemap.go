package ui

import (
	"image/color"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/widget"

	"github.com/rhetts/DriveMonger/internal/scan"
)

// Treemap is a custom widget that renders a directory's children as nested
// rectangles whose area is proportional to disk usage. Tapping a directory
// tile drills into it via the OnDrill callback.
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

// Tapped implements fyne.Tappable, dispatching the tap to the tile under the
// cursor.
func (t *Treemap) Tapped(e *fyne.PointEvent) {
	for _, ti := range t.tiles {
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
// and regenerates the rectangles and labels.
func (r *treemapRenderer) rebuild(size fyne.Size) {
	r.tm.tiles = r.tm.tiles[:0]
	r.objects = r.objects[:0]

	if r.tm.current == nil || !r.tm.current.IsDir || len(r.tm.current.Children) == 0 {
		// Nothing to show: leave a blank background.
		bg := canvas.NewRectangle(color.NRGBA{R: 30, G: 30, B: 34, A: 255})
		bg.Resize(size)
		r.objects = append(r.objects, bg)
		return
	}

	layoutTreemap(r.tm.current.Children, 0, 0, size.Width, size.Height, &r.tm.tiles)

	for i, ti := range r.tm.tiles {
		fill := tileColor(i)
		rect := canvas.NewRectangle(fill)
		rect.StrokeColor = color.NRGBA{R: 20, G: 20, B: 24, A: 255}
		rect.StrokeWidth = 1
		rect.Move(fyne.NewPos(ti.x, ti.y))
		rect.Resize(fyne.NewSize(ti.w, ti.h))
		r.objects = append(r.objects, rect)
	}

	// Labels drawn after all rectangles so they sit on top.
	for _, ti := range r.tm.tiles {
		if ti.w < 46 || ti.h < 18 {
			continue // too small to label legibly
		}
		label := ti.node.Name + "\n" + scan.HumanSize(ti.node.Size)
		txt := canvas.NewText(label, labelColor)
		txt.TextSize = 12
		txt.Move(fyne.NewPos(ti.x+4, ti.y+2))
		r.objects = append(r.objects, txt)
	}
}

var labelColor = color.NRGBA{R: 245, G: 245, B: 245, A: 255}

// tileColor returns a stable, visually distinct fill color for the i-th tile by
// stepping hue along the golden ratio.
func tileColor(i int) color.NRGBA {
	hue := math.Mod(float64(i)*0.61803398875, 1.0)
	r, g, b := hsvToRGB(hue, 0.55, 0.75)
	return color.NRGBA{R: r, G: g, B: b, A: 255}
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
