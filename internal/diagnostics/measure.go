package diagnostics

import (
	"context"
	"strconv"
	"strings"
	"time"
)

type Sample struct {
	At       time.Time `json:"at"`
	VRAMMiB  uint64    `json:"device_vram_mib"`
	RAMBytes uint64    `json:"system_used_ram_bytes"`
	Error    string    `json:"error,omitempty"`
}

func Measure() Sample {
	s := Sample{At: time.Now().UTC()}
	m, _ := systemMemoryAndDisk()
	if values, ok := m.(map[string]uint64); ok {
		s.RAMBytes = values["total_bytes"] - values["available_bytes"]
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	raw := command(ctx, "nvidia-smi", "--query-gpu=memory.used", "--format=csv,noheader,nounits")
	for _, line := range strings.Split(raw, "\n") {
		n, e := strconv.ParseUint(strings.TrimSpace(line), 10, 64)
		if e != nil {
			s.Error = raw
		} else {
			s.VRAMMiB += n
		}
	}
	return s
}
