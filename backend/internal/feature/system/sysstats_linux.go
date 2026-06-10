//go:build linux

package system

import (
	"bufio"
	"os"
	"strconv"
	"strings"
)

// cpuTimes returns busy and total jiffies from /proc/stat for the whole host.
func cpuTimes() (busy, total uint64, ok bool) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		return 0, 0, false
	}
	fields := strings.Fields(sc.Text())
	if len(fields) < 5 || fields[0] != "cpu" {
		return 0, 0, false
	}
	var idle uint64
	for i, f := range fields[1:] {
		v, err := strconv.ParseUint(f, 10, 64)
		if err != nil {
			continue
		}
		total += v
		if i == 3 || i == 4 { // idle + iowait
			idle += v
		}
	}
	return total - idle, total, true
}

// hostMemory reports used and total RAM in bytes from /proc/meminfo.
func hostMemory() (used, total int64, ok bool) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, 0, false
	}
	defer f.Close()

	var memTotal, memAvailable int64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 2 {
			continue
		}
		kb, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			continue
		}
		switch fields[0] {
		case "MemTotal:":
			memTotal = kb * 1024
		case "MemAvailable:":
			memAvailable = kb * 1024
		}
	}
	if memTotal == 0 {
		return 0, 0, false
	}
	return memTotal - memAvailable, memTotal, true
}

// selfCPUTicks returns utime+stime of this process in clock ticks.
func selfCPUTicks() (uint64, bool) {
	data, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return 0, false
	}
	return statCPUTicks(string(data))
}

func selfRSS() (int64, bool) {
	return readVmRSS("/proc/self/status")
}

func readVmRSS(path string) (int64, bool) {
	f, err := os.Open(path)
	if err != nil {
		return 0, false
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 2 && fields[0] == "VmRSS:" {
			kb, err := strconv.ParseInt(fields[1], 10, 64)
			if err != nil {
				return 0, false
			}
			return kb * 1024, true
		}
	}
	return 0, false
}

// ffmpegProcs scans /proc for ffmpeg processes (transcode jobs, JIT sessions,
// artwork resizes), returning per-pid CPU ticks, summed resident memory and
// the process count.
func ffmpegProcs() (map[int]uint64, int64, int) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, 0, 0
	}
	ticks := map[int]uint64{}
	var rss int64
	for _, e := range entries {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		comm, err := os.ReadFile("/proc/" + e.Name() + "/comm")
		if err != nil || strings.TrimSpace(string(comm)) != "ffmpeg" {
			continue
		}
		stat, err := os.ReadFile("/proc/" + e.Name() + "/stat")
		if err != nil {
			continue
		}
		t, ok := statCPUTicks(string(stat))
		if !ok {
			continue
		}
		ticks[pid] = t
		if r, ok := readVmRSS("/proc/" + e.Name() + "/status"); ok {
			rss += r
		}
	}
	return ticks, rss, len(ticks)
}

func loadAverage() (float64, bool) {
	data, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0, false
	}
	fields := strings.Fields(string(data))
	if len(fields) == 0 {
		return 0, false
	}
	v, err := strconv.ParseFloat(fields[0], 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
