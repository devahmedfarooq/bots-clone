//go:build !windows

package diagnostics

import (
	"os"
	"strings"
)

func systemMemoryAndDisk() (any, any) {
	b, e := os.ReadFile("/proc/meminfo")
	if e != nil {
		return "unavailable", "use df -k ."
	}
	return strings.TrimSpace(string(b)), "use df -k ."
}
