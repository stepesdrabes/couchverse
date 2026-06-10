package system

import (
	"net/http"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"couchverse/internal/httpx"
)

// SysStats reports host and process resource usage for the admin overview.
// CPU% is sampled in the background from successive /proc reads so the
// endpoint never blocks. Besides the host totals it attributes usage to the
// Go process itself and to ffmpeg children (transcodes, JIT sessions).
type SysStats struct {
	start time.Time
	mu    sync.Mutex
	snap  procSample
}

type procSample struct {
	cpuPercent    float64 // whole host; -1 when unavailable
	appCPUPercent float64 // this Go process; -1 when unavailable
	ffCPUPercent  float64 // all ffmpeg processes; -1 when unavailable
	appRSS        int64
	ffRSS         int64
	ffProcs       int
}

func NewSysStats() *SysStats {
	s := &SysStats{
		start: time.Now(),
		snap:  procSample{cpuPercent: -1, appCPUPercent: -1, ffCPUPercent: -1},
	}
	go s.sample()
	return s
}

func (s *SysStats) sample() {
	prevBusy, prevTotal, ok := cpuTimes()
	if !ok {
		return
	}
	prevApp, _ := selfCPUTicks()
	prevFF, _, _ := ffmpegProcs()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		busy, total, ok := cpuTimes()
		if !ok {
			continue
		}
		app, appOK := selfCPUTicks()
		ff, ffRSS, ffProcs := ffmpegProcs()

		snap := procSample{cpuPercent: -1, appCPUPercent: -1, ffCPUPercent: -1,
			ffRSS: ffRSS, ffProcs: ffProcs}
		// jiffies and clock ticks share the same unit, so all three
		// percentages are fractions of total machine capacity.
		if dTotal := total - prevTotal; dTotal > 0 {
			snap.cpuPercent = float64(busy-prevBusy) / float64(dTotal) * 100
			if appOK && app >= prevApp {
				snap.appCPUPercent = float64(app-prevApp) / float64(dTotal) * 100
			}
			var ffDelta uint64
			for pid, t := range ff {
				// only pids present in both samples; new ones count next tick
				if p, ok := prevFF[pid]; ok && t >= p {
					ffDelta += t - p
				}
			}
			snap.ffCPUPercent = float64(ffDelta) / float64(dTotal) * 100
		}
		if rss, ok := selfRSS(); ok {
			snap.appRSS = rss
		}

		s.mu.Lock()
		s.snap = snap
		s.mu.Unlock()
		prevBusy, prevTotal, prevApp, prevFF = busy, total, app, ff
	}
}

// statCPUTicks parses utime+stime (fields 14 and 15) from a /proc/<pid>/stat
// line, splitting after the last ')' so spaces in comm cannot shift fields.
func statCPUTicks(stat string) (uint64, bool) {
	i := strings.LastIndexByte(stat, ')')
	if i < 0 {
		return 0, false
	}
	fields := strings.Fields(stat[i+1:])
	if len(fields) < 13 { // fields[0] is field 3 (state), utime/stime are 11/12
		return 0, false
	}
	utime, err1 := strconv.ParseUint(fields[11], 10, 64)
	stime, err2 := strconv.ParseUint(fields[12], 10, 64)
	if err1 != nil || err2 != nil {
		return 0, false
	}
	return utime + stime, true
}

func (s *SysStats) Get(w http.ResponseWriter, r *http.Request) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	s.mu.Lock()
	snap := s.snap
	s.mu.Unlock()

	out := map[string]any{
		"cpuPercent":    snap.cpuPercent,
		"cpuCores":      runtime.NumCPU(),
		"memUsed":       0,
		"memTotal":      0,
		"load1":         -1.0,
		"goHeapBytes":   mem.HeapAlloc,
		"goroutines":    runtime.NumGoroutine(),
		"uptimeSeconds": int64(time.Since(s.start).Seconds()),
		"app": map[string]any{
			"cpuPercent": snap.appCPUPercent,
			"memBytes":   snap.appRSS,
		},
		"ffmpeg": map[string]any{
			"cpuPercent": snap.ffCPUPercent,
			"memBytes":   snap.ffRSS,
			"processes":  snap.ffProcs,
		},
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
