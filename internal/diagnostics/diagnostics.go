package diagnostics

import (
	"context"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

type Report struct {
	OS           string            `json:"os"`
	Arch         string            `json:"arch"`
	GoVersion    string            `json:"go_version"`
	CPUThreads   int               `json:"cpu_threads"`
	GPU          string            `json:"gpu"`
	Memory       any               `json:"memory"`
	Disks        any               `json:"disks"`
	Dependencies map[string]string `json:"dependencies"`
}

func command(ctx context.Context, name string, args ...string) string {
	b, e := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if e != nil {
		return "unavailable: " + e.Error()
	}
	return strings.TrimSpace(string(b))
}
func Collect() Report {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	r := Report{OS: runtime.GOOS, Arch: runtime.GOARCH, CPUThreads: runtime.NumCPU(), Dependencies: map[string]string{}}
	r.GoVersion = runtime.Version()
	r.GPU = command(ctx, "nvidia-smi", "--query-gpu=name,memory.total,memory.used,driver_version", "--format=csv,noheader")
	r.Memory, r.Disks = systemMemoryAndDisk()
	for _, name := range []string{"go", "llama-server"} {
		p, e := exec.LookPath(name)
		if e != nil {
			r.Dependencies[name] = "missing from PATH"
		} else {
			r.Dependencies[name] = p
		}
	}
	return r
}
