package system

import (
	"net/http"
	"runtime"
	"sync/atomic"
	"time"

	"couchverse/internal/httpx"
)

// SysStats reports host and process resource usage for the admin overview.
// CPU% is sampled in the background from successive /proc/stat reads so the
// endpoint never blocks.
type SysStats struct {
	start      time.Time
	cpuPercent atomic.Int64 // tenths of a percent; -1 when unavailable
}

func NewSysStats() *SysStats {
	s := &SysStats{start: time.Now()}
	s.cpuPercent.Store(-1)
	go s.sample()
	return s
}

func (s *SysStats) sample() {
	prevBusy, prevTotal, ok := cpuTimes()
	if !ok {
		return
	}
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		busy, total, ok := cpuTimes()
		if !ok {
			continue
		}
		dTotal := total - prevTotal
		if dTotal > 0 {
			pct := float64(busy-prevBusy) / float64(dTotal) * 100
			s.cpuPercent.Store(int64(pct * 10))
		}
		prevBusy, prevTotal = busy, total
	}
}

func (s *SysStats) Get(w http.ResponseWriter, r *http.Request) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	out := map[string]any{
		"cpuPercent":    -1.0,
		"cpuCores":      runtime.NumCPU(),
		"memUsed":       0,
		"memTotal":      0,
		"load1":         -1.0,
		"goHeapBytes":   mem.HeapAlloc,
		"goroutines":    runtime.NumGoroutine(),
		"uptimeSeconds": int64(time.Since(s.start).Seconds()),
	}
	if cpu := s.cpuPercent.Load(); cpu >= 0 {
		out["cpuPercent"] = float64(cpu) / 10
	}
	if used, total, ok := hostMemory(); ok {
		out["memUsed"] = used
		out["memTotal"] = total
	}
	if load, ok := loadAverage(); ok {
		out["load1"] = load
	}
	httpx.JSON(w, http.StatusOK, out)
}
