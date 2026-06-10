package api

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
		"goHeapBytes", "goroutines", "uptimeSeconds"} {
		if _, ok := out[key]; !ok {
			t.Errorf("response missing %q", key)
		}
	}
	if cores, _ := out["cpuCores"].(float64); cores < 1 {
		t.Errorf("cpuCores = %v, want >= 1", out["cpuCores"])
	}
}
