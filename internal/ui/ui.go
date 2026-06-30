// Package ui builds the DriveMonger desktop interface: a folder/drive picker, a
// collapsible size tree, and an interactive treemap.
package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/rhetts/DriveMonger/internal/scan"
)

// UI holds the application's widgets and the currently scanned tree.
type UI struct {
	win fyne.Window

	root        *scan.Node
	nodesByPath map[string]*scan.Node

	tree      *widget.Tree
	treemap   *Treemap
	tmCurrent *scan.Node // node whose children the treemap is showing

	driveSelect *widget.Select
	crumb       *widget.Label
	upBtn       *widget.Button
	status      *widget.Label
}

// Run creates the window and starts the Fyne event loop. It blocks until the
// window is closed.
func Run() {
	a := app.NewWithID("com.rhetts.drivemonger")
	u := &UI{
		win:         a.NewWindow("DriveMonger"),
		nodesByPath: map[string]*scan.Node{},
	}
	u.win.SetContent(u.buildContent())
	u.win.Resize(fyne.NewSize(1024, 720))
	u.win.ShowAndRun()
}

func (u *UI) buildContent() fyne.CanvasObject {
	// --- top toolbar: drive picker, open-folder, rescan ---
	u.driveSelect = widget.NewSelect(windowsDrives(), func(d string) {
		if d != "" {
			u.startScan(d)
		}
	})
	u.driveSelect.PlaceHolder = "Select drive…"

	openBtn := widget.NewButton("Open Folder…", func() {
		dlg := dialog.NewFolderOpen(func(list fyne.ListableURI, err error) {
			if err != nil || list == nil {
				return
			}
			u.startScan(uriToPath(list))
		}, u.win)
		dlg.Resize(fyne.NewSize(760, 560))
		dlg.Show()
	})

	rescanBtn := widget.NewButton("Rescan", func() {
		if u.root != nil {
			u.startScan(u.root.Path)
		}
	})

	toolbar := container.NewHBox(
		widget.NewLabel("Drive:"), u.driveSelect,
		openBtn, rescanBtn,
	)

	// --- tree tab ---
	u.tree = u.buildTree()

	// --- treemap tab ---
	u.treemap = NewTreemap()
	u.treemap.OnDrill = u.setTreemapNode
	u.treemap.OnSelect = u.showStatus

	u.upBtn = widget.NewButton("⬆ Up", func() {
		if u.tmCurrent != nil && u.tmCurrent.Parent != nil {
			u.setTreemapNode(u.tmCurrent.Parent)
		}
	})
	u.upBtn.Disable()
	u.crumb = widget.NewLabel("No folder scanned")

	// Live control to tune the adaptive-nesting threshold while testing values.
	tierValue := widget.NewLabel(fmt.Sprintf("%.0f%%", u.treemap.TierAreaFraction*100))
	tierSlider := widget.NewSlider(0.05, 0.50)
	tierSlider.Step = 0.05
	tierSlider.Value = u.treemap.TierAreaFraction
	tierSlider.OnChanged = func(v float64) {
		u.treemap.TierAreaFraction = v
		tierValue.SetText(fmt.Sprintf("%.0f%%", v*100))
		u.treemap.Refresh()
	}
	tierControl := container.NewHBox(
		widget.NewLabel("Add tier when box >"),
		container.NewGridWrap(fyne.NewSize(150, 34), tierSlider),
		tierValue,
		widget.NewLabel("of display"),
	)

	treemapTop := container.NewBorder(nil, nil, u.upBtn, tierControl, u.crumb)
	treemapTab := container.NewBorder(treemapTop, nil, nil, nil, u.treemap)

	tabs := container.NewAppTabs(
		container.NewTabItem("Tree", u.tree),
		container.NewTabItem("Treemap", treemapTab),
	)
	tabs.SetTabLocation(container.TabLocationTop)

	// --- status bar ---
	u.status = widget.NewLabel("Pick a drive or folder to begin.")

	return container.NewBorder(toolbar, u.status, nil, nil, tabs)
}

func (u *UI) buildTree() *widget.Tree {
	t := widget.NewTree(
		func(uid widget.TreeNodeID) []widget.TreeNodeID {
			if uid == "" {
				if u.root == nil {
					return nil
				}
				return []widget.TreeNodeID{u.root.Path}
			}
			n := u.nodesByPath[uid]
			if n == nil {
				return nil
			}
			ids := make([]widget.TreeNodeID, len(n.Children))
			for i, c := range n.Children {
				ids[i] = c.Path
			}
			return ids
		},
		func(uid widget.TreeNodeID) bool {
			if uid == "" {
				return u.root != nil
			}
			n := u.nodesByPath[uid]
			return n != nil && n.IsDir && len(n.Children) > 0
		},
		func(branch bool) fyne.CanvasObject {
			return widget.NewLabel("template item placeholder")
		},
		func(uid widget.TreeNodeID, branch bool, o fyne.CanvasObject) {
			n := u.nodesByPath[uid]
			if n == nil {
				o.(*widget.Label).SetText(uid)
				return
			}
			o.(*widget.Label).SetText(fmt.Sprintf("%s   —   %s", n.Name, scan.HumanSize(n.Size)))
		},
	)
	t.OnSelected = func(uid widget.TreeNodeID) {
		if n := u.nodesByPath[uid]; n != nil {
			u.showStatus(n)
			if n.IsDir && len(n.Children) > 0 {
				u.setTreemapNode(n)
			}
		}
	}
	return t
}

// showStatus updates the status bar to describe a single node.
func (u *UI) showStatus(n *scan.Node) {
	u.status.SetText(fmt.Sprintf("%s  —  %s", n.Path, scan.HumanSize(n.Size)))
}

// startScan runs a scan of path in the background, showing a cancellable
// progress dialog, and installs the result when finished.
func (u *UI) startScan(path string) {
	prog := &scan.Progress{}
	ctx, cancel := context.WithCancel(context.Background())

	msg := widget.NewLabel("Starting…")
	progDialog := dialog.NewCustom("Scanning "+path, "Cancel", msg, u.win)
	progDialog.SetOnClosed(cancel)
	progDialog.Show()

	// Periodically refresh the progress label.
	go func() {
		ticker := time.NewTicker(150 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				dirs, files, bytes := prog.Dirs.Load(), prog.Files.Load(), prog.Bytes.Load()
				text := fmt.Sprintf("Dirs: %d   Files: %d   Size: %s", dirs, files, scan.HumanSize(bytes))
				fyne.Do(func() { msg.SetText(text) })
			}
		}
	}()

	go func() {
		root, err := scan.Scan(ctx, path, prog)
		fyne.Do(func() {
			progDialog.Hide()
			if err != nil && errors.Is(err, context.Canceled) {
				return
			}
			if err != nil {
				dialog.ShowError(err, u.win)
				return
			}
			u.setRoot(root)
		})
	}()
}

// setRoot installs a freshly scanned tree and refreshes both views.
func (u *UI) setRoot(root *scan.Node) {
	u.root = root
	u.nodesByPath = map[string]*scan.Node{}
	indexNodes(root, u.nodesByPath)

	u.tree.Refresh()
	u.tree.OpenBranch(root.Path)

	u.setTreemapNode(root)
	u.status.SetText(fmt.Sprintf("%s  —  %s total", root.Path, scan.HumanSize(root.Size)))
}

// setTreemapNode points the treemap at node and updates the breadcrumb/up state.
func (u *UI) setTreemapNode(node *scan.Node) {
	u.tmCurrent = node
	u.crumb.SetText(fmt.Sprintf("%s   (%s)", node.Path, scan.HumanSize(node.Size)))
	u.treemap.SetNode(node)
	if node.Parent != nil {
		u.upBtn.Enable()
	} else {
		u.upBtn.Disable()
	}
}

// indexNodes records every node by its path so the tree and treemap can resolve
// UIDs back to nodes.
func indexNodes(n *scan.Node, m map[string]*scan.Node) {
	m[n.Path] = n
	for _, c := range n.Children {
		indexNodes(c, m)
	}
}

// windowsDrives returns the existing drive roots (C:\, D:\, …). On non-Windows
// platforms the probe simply finds nothing and the picker stays empty.
func windowsDrives() []string {
	var out []string
	for c := 'A'; c <= 'Z'; c++ {
		p := fmt.Sprintf("%c:\\", c)
		if _, err := os.Stat(p); err == nil {
			out = append(out, p)
		}
	}
	return out
}

// uriToPath converts a Fyne folder URI to an OS path, fixing the leading-slash
// quirk Fyne produces for Windows drive paths ("/C:/Users" -> "C:\Users").
func uriToPath(u fyne.URI) string {
	p := u.Path()
	if len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		p = p[1:]
	}
	return filepath.FromSlash(p)
}
