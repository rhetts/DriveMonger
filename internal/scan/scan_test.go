package scan

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestHumanSize(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{5 * 1024 * 1024 * 1024, "5.0 GB"},
	}
	for _, c := range cases {
		if got := HumanSize(c.in); got != c.want {
			t.Errorf("HumanSize(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestScanSizesAndSort(t *testing.T) {
	dir := t.TempDir()

	// dir/
	//   big.bin   (3000 bytes)
	//   sub/
	//     small.txt (10 bytes)
	mustWrite(t, filepath.Join(dir, "big.bin"), 3000)
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "sub", "small.txt"), 10)

	root, err := Scan(context.Background(), dir, &Progress{})
	if err != nil {
		t.Fatalf("Scan returned error: %v", err)
	}

	if root.Size != 3010 {
		t.Errorf("root size = %d, want 3010", root.Size)
	}
	if len(root.Children) != 2 {
		t.Fatalf("root has %d children, want 2", len(root.Children))
	}
	// Children must be sorted by descending size: big.bin (3000) before sub (10).
	if root.Children[0].Name != "big.bin" {
		t.Errorf("first child = %q, want big.bin (largest first)", root.Children[0].Name)
	}
	if !root.Children[1].IsDir || root.Children[1].Size != 10 {
		t.Errorf("second child = %+v, want sub dir of size 10", root.Children[1])
	}
}

func mustWrite(t *testing.T, path string, n int) {
	t.Helper()
	if err := os.WriteFile(path, make([]byte, n), 0o644); err != nil {
		t.Fatal(err)
	}
}
