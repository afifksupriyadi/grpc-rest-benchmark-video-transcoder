// Package resource provides direct, point-in-time reads of process CPU and memory usage.
package resource

import "github.com/prometheus/procfs"

// Snapshot holds CPU and memory usage of the current process at one specific instant.
// - CPUSeconds is cumulative CPU time consumed since process start, in seconds
// - MemoryBytes is resident memory at this instant, in bytes
type Snapshot struct {
	CPUSeconds  float64
	MemoryBytes int
}

// Read captures the current process's CPU time and resident memory immediately.
// This reads directly from /proc/self/stat, bypassing Prometheus scrape entirely,
// so it can be called at any point in request-handling code.
func Read() (Snapshot, error) {
	proc, err := procfs.Self()
	if err != nil {
		return Snapshot{}, err
	}

	stat, err := proc.Stat()
	if err != nil {
		return Snapshot{}, err
	}

	return Snapshot{
		CPUSeconds:  stat.CPUTime(),
		MemoryBytes: stat.ResidentMemory(),
	}, nil
}
