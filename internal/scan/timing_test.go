package scan

import (
	"context"
	"os"
	"testing"
	"time"
)

// TestScanTiming is a temporary manual benchmark: set SCAN_PATH to a real
// directory to time a full scan.
func TestScanTiming(t *testing.T) {
	p := os.Getenv("SCAN_PATH")
	if p == "" {
		t.Skip("set SCAN_PATH to time a scan")
	}
	start := time.Now()
	prog := &Progress{}
	root, err := Scan(context.Background(), p, prog)
	dur := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("scanned %s in %s: %d dirs, %d files, %s",
		p, dur, prog.Dirs.Load(), prog.Files.Load(), HumanSize(root.Size))
}
