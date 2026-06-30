package scan

import "fmt"

// sizeUnits are the binary (1024-based) magnitude suffixes above bytes.
var sizeUnits = []string{"KB", "MB", "GB", "TB", "PB", "EB"}

// HumanSize formats a byte count as a human-readable string using binary
// (1024-based) units, e.g. 1536 -> "1.5 KB".
func HumanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), sizeUnits[exp])
}
