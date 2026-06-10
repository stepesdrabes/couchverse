//go:build !linux

package api

// Host CPU/memory sampling is only wired up for Linux (the deploy target). On
// other platforms the system card shows the Go-runtime metrics only.

func cpuTimes() (busy, total uint64, ok bool) { return 0, 0, false }

func hostMemory() (used, total int64, ok bool) { return 0, 0, false }

func loadAverage() (float64, bool) { return 0, false }
