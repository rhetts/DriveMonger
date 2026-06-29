// Package scan walks a directory tree and computes the disk space consumed by
// each directory and file, building an in-memory tree that the UI can render.
package scan

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
)

// parallelism caps how many directories are read concurrently. Scanning is
// I/O-bound, so we oversubscribe the CPU count to keep many reads in flight and
// hide per-directory latency, which is where most of a cold scan's time goes.
var parallelism = max(8, runtime.NumCPU()*4)

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
	sem := make(chan struct{}, parallelism)
	walk(ctx, rootNode, p, sem)
	sortTree(rootNode)
	if err := ctx.Err(); err != nil {
		return rootNode, err
	}
	return rootNode, nil
}

// walk fills in node's children and accumulates sizes. Subdirectories are
// scanned concurrently: each child directory is handed to a goroutine when a
// slot is free in sem, otherwise it is walked inline. A goroutine never holds a
// slot while waiting on its own children, so the recursion cannot deadlock.
//
// Concurrency safety: only this invocation (the directory's owner) appends to
// node.Children; each child goroutine mutates only its own subtree, and the
// parent reads child.Size after childWg.Wait() establishes happens-before.
// Progress counters are atomic.
func walk(ctx context.Context, node *Node, p *Progress, sem chan struct{}) {
	entries, err := os.ReadDir(node.Path)
	if err != nil {
		// Skip directories we cannot read (permissions, junctions, etc.).
		return
	}

	var childWg sync.WaitGroup
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			childWg.Wait()
			return
		default:
		}

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
			childWg.Add(1)
			select {
			case sem <- struct{}{}:
				go func(c *Node) {
					defer childWg.Done()
					defer func() { <-sem }()
					walk(ctx, c, p, sem)
				}(child)
			default:
				// No free slot: walk inline so we don't block holding nothing.
				walk(ctx, child, p, sem)
				childWg.Done()
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
	}

	// Wait for concurrently-scanned subdirectories to finish, then total up the
	// children's sizes (directory sizes are only known once their walk returns).
	childWg.Wait()
	for _, c := range node.Children {
		node.Size += c.Size
	}
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
