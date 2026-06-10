package system

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSysStatsGet(t *testing.T) {
	h := NewSysStats()
	rec := httptest.NewRecorder()
	h.Get(rec, httptest.NewRequest(http.MethodGet, "/admin/system", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var out map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	for _, key := range []string{"cpuPercent", "cpuCores", "memUsed", "memTotal", "load1",
		"goHeapBytes", "goroutines", "uptimeSeconds", "app", "ffmpeg"} {
		if _, ok := out[key]; !ok {
			t.Errorf("response missing %q", key)
		}
	}
	if cores, _ := out["cpuCores"].(float64); cores < 1 {
		t.Errorf("cpuCores = %v, want >= 1", out["cpuCores"])
	}
	app, _ := out["app"].(map[string]any)
	for _, key := range []string{"cpuPercent", "memBytes"} {
		if _, ok := app[key]; !ok {
			t.Errorf("app missing %q", key)
		}
	}
	ffmpeg, _ := out["ffmpeg"].(map[string]any)
	for _, key := range []string{"cpuPercent", "memBytes", "processes"} {
		if _, ok := ffmpeg[key]; !ok {
			t.Errorf("ffmpeg missing %q", key)
		}
	}
}

func TestStatCPUTicks(t *testing.T) {
	// comm contains spaces and a ')' to prove field splitting stays correct
	line := "1234 (ff mpeg (x)) S 1 1234 1234 0 -1 4194560 1207 0 0 0 1500 250 0 0 20 0 3 0 8000 100000 500 18446744073709551615"
	ticks, ok := statCPUTicks(line)
	if !ok {
		t.Fatal("statCPUTicks failed on valid line")
	}
	if ticks != 1750 { // utime 1500 + stime 250
		t.Errorf("ticks = %d, want 1750", ticks)
	}
	if _, ok := statCPUTicks("garbage with no paren"); ok {
		t.Error("expected failure on malformed line")
	}
}
