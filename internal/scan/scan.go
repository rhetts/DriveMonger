// Package scan walks a directory tree and computes the disk space consumed by
// each directory and file, building an in-memory tree that the UI can render.
package scan

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
)

// Node is a single entry (file or directory) in the scanned tree.
type Node struct {
	Name     string  // base name of the entry
	Path     string  // absolute path
	Size     int64   // total bytes: own size for files, sum of descendants for dirs
	IsDir    bool    // true for directories
	Parent   *Node   // nil for the root
	Children []*Node // populated for directories, sorted by Size descending
}

// Progress carries live counters while a scan is running so the UI can show
// feedback. All fields are updated atomically by the scanner goroutine.
type Progress struct {
	Dirs  atomic.Int64 // directories visited
	Files atomic.Int64 // files visited
	Bytes atomic.Int64 // total bytes accounted for so far
}

// Scan walks root and returns the populated tree. The context can be cancelled
// to abort an in-progress scan; on cancellation the partially built tree is
// returned along with ctx.Err(). Unreadable directories (permission denied,
// etc.) are skipped rather than aborting the whole scan.
func Scan(ctx context.Context, root string, p *Progress) (*Node, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}

	rootNode := &Node{
		Name:  displayName(abs),
		Path:  abs,
		IsDir: info.IsDir(),
	}

	if !rootNode.IsDir {
		rootNode.Size = info.Size()
		if p != nil {
			p.Files.Add(1)
			p.Bytes.Add(info.Size())
		}
		return rootNode, nil
	}

	if p != nil {
		p.Dirs.Add(1)
	}
	walkErr := walk(ctx, rootNode, p)
	sortTree(rootNode)
	return rootNode, walkErr
}

// walk recursively fills in node's children and accumulates sizes. It returns
// ctx.Err() if the scan was cancelled.
func walk(ctx context.Context, node *Node, p *Progress) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	entries, err := os.ReadDir(node.Path)
	if err != nil {
		// Skip directories we cannot read (permissions, junctions, etc.).
		return nil
	}

	for _, entry := range entries {
		full := filepath.Join(node.Path, entry.Name())
		child := &Node{
			Name:   entry.Name(),
			Path:   full,
			IsDir:  entry.IsDir(),
			Parent: node,
		}

		if entry.IsDir() {
			if p != nil {
				p.Dirs.Add(1)
			}
			// Don't follow symlinked directories to avoid cycles and
			// double-counting; entry.IsDir() is false for symlinks so the
			// else-branch below handles them as plain entries.
			if err := walk(ctx, child, p); err != nil {
				node.Children = append(node.Children, child)
				node.Size += child.Size
				return err // propagate cancellation
			}
		} else {
			info, err := entry.Info()
			if err == nil {
				child.Size = info.Size()
			}
			if p != nil {
				p.Files.Add(1)
				p.Bytes.Add(child.Size)
			}
		}

		node.Children = append(node.Children, child)
		node.Size += child.Size
	}
	return nil
}

// sortTree orders every directory's children by descending size so the largest
// consumers come first in both the tree and the treemap.
func sortTree(node *Node) {
	if !node.IsDir {
		return
	}
	sort.Slice(node.Children, func(i, j int) bool {
		return node.Children[i].Size > node.Children[j].Size
	})
	for _, c := range node.Children {
		sortTree(c)
	}
}

// displayName returns a friendly label for a path. For a drive root such as
// "C:\" the base name is empty, so fall back to the path itself.
func displayName(path string) string {
	base := filepath.Base(path)
	if base == "." || base == string(filepath.Separator) || base == "" {
		return path
	}
	// Drive roots like "C:\" produce a base of "\"; show the volume instead.
	if vol := filepath.VolumeName(path); vol != "" && filepath.Clean(path) == vol+string(filepath.Separator) {
		return path
	}
	return base
}
