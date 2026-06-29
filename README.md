# DriveMonger

A desktop disk-space visualizer for Windows (and cross-platform), built in Go
with [Fyne](https://fyne.io). It's a modern, open replacement for the classic
**SpaceMonger**: point it at a drive or folder and it shows you exactly what is
eating your disk space.

![status](https://github.com/rhetts/DriveMonger/actions/workflows/build.yml/badge.svg)

## Features (v1)

- **Pick a drive or any folder** — choose a drive letter from the dropdown, or
  browse to any directory with the folder picker.
- **Background scan with live progress** — scanning runs off the UI thread and
  shows a running count of directories, files, and bytes; cancel any time.
- **Tree view** — a collapsible tree of every directory and file, sorted by
  size (largest first) at every level.
- **Treemap view** — an interactive map of nested rectangles whose area is
  proportional to size. Click a directory to drill in; use **⬆ Up** to go back.
- **Unreadable directories are skipped** gracefully (permission errors don't
  abort the scan).

## Requirements

Fyne renders with OpenGL via cgo, so building requires:

- **Go 1.22+**
- A **C compiler**:
  - Windows: [MinGW-w64](https://www.mingw-w64.org/) (e.g. `choco install mingw`)
  - macOS: Xcode command-line tools (`xcode-select --install`)
  - Linux: `gcc` plus the X11/GL dev headers (see the
    [Fyne prerequisites](https://docs.fyne.io/started/)).

## Build & run

```sh
go mod tidy
go run .
```

To produce a standalone binary:

```sh
go build -o DriveMonger.exe .      # Windows
go build -o DriveMonger .          # macOS / Linux
```

For a packaged app with an icon, install the Fyne tooling and run:

```sh
go install fyne.io/fyne/v2/cmd/fyne@latest
fyne package -os windows
```

## Project layout

```
main.go                     entry point
internal/scan/scan.go       directory walker -> size tree
internal/scan/format.go     human-readable byte formatting
internal/ui/ui.go           window, toolbar, tree, status bar, scan flow
internal/ui/treemap.go      custom treemap widget (drill-down)
internal/ui/treemap_layout.go  treemap partition algorithm
```

## Roadmap

- Free-space tile and percentage-of-drive bars
- Right-click actions (open in Explorer, delete, copy path)
- File-type coloring and filtering
- Remembering recent scans
- Export of the tree to CSV/JSON

## License

MIT — see [LICENSE](LICENSE).
