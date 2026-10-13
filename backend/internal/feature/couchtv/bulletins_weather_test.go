package couchtv

import (
	"context"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	body, err := os.ReadFile(filepath.Join("testdata", "bulletins", name))
	if err != nil {
		t.Fatal(err)
	}
	return body
}

// The forecast of 13 Oct 17:00 for Praha, Brno and Ostrava, as Open-Meteo sent it.
func TestParseForecast(t *testing.T) {
	cities := DefaultCities()[:3]
	got, err := parseForecast(readFixture(t, "openmeteo.json"), cities)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Name != "Praha" || got[2].Name != "Ostrava" {
		t.Fatalf("cities: %+v", got)
	}
	praha := got[0]
	// the configured position, not the model's grid cell (50.087627, 14.409841)
	if praha.Lat != 50.0880 || praha.Lon != 14.4208 {
		t.Errorf("Praha is at %v, %v", praha.Lat, praha.Lon)
	}
	if praha.Now != (CouchTvWeatherNow{Temperature: 14.9, Code: 1, IsDay: true}) {
		t.Errorf("Praha now: %+v", praha.Now)
	}
	if len(praha.Days) != forecastDays || praha.Days[0].Date != "2026-10-13" || praha.Days[4].Date != "2026-10-17" {
		t.Fatalf("Praha's days: %+v", praha.Days)
	}
	if d := praha.Days[1]; d.Min != 9.9 || d.Max != 17.3 || d.PrecipitationProbability == nil || *d.PrecipitationProbability != 70 {
		t.Errorf("Praha tomorrow: %+v", d)
	}
	for _, c := range []struct {
		city, day int
		daily     int // the source's daily code, the day's worst hour
		want      int
	}{
		// overcast for an hour of the day: partly cloudy
		{0, 0, 3, 2},
		// the showers fall at night and in the morning; the daytime stays partly cloudy
		{0, 1, 80, 2},
		// morning fog, then sun: mainly clear
		{0, 3, 45, 1},
		// rain from 15:00 on: a rainy day
		{0, 4, 61, 61},
		// rain at 9 and 10, dry after: showers
		{2, 1, 61, 80},
		// fog at 9, sun after: clear
		{2, 3, 45, 0},
	} {
		d := got[c.city].Days[c.day]
		if d.Code != c.want {
			t.Errorf("%s %s: code %d (the source's daily %d), want %d", got[c.city].Name, d.Date, d.Code, c.daily, c.want)
		}
	}
}

func TestParseForecastOneCity(t *testing.T) {
	var locs []json.RawMessage
	if err := json.Unmarshal(readFixture(t, "openmeteo.json"), &locs); err != nil {
		t.Fatal(err)
	}
	// asked for one place, Open-Meteo answers with an object rather than an array
	got, err := parseForecast(locs[1], []CouchTvCity{{Name: "Brno", Lat: 49.1952, Lon: 16.6080}})
	if err != nil || len(got) != 1 || got[0].Now.Temperature != 14.9 || len(got[0].Days) != forecastDays {
		t.Errorf("one city: %+v, %v", got, err)
	}
}

func TestParseForecastFailures(t *testing.T) {
	cities := DefaultCities()[:3]
	for name, body := range map[string]string{
		"refused":       `{"error":true,"reason":"Latitude must be in range of -90 to 90°. Given: 150.0."}`,
		"wrong count":   `[{"current":{"temperature_2m":1,"weather_code":0,"is_day":1}}]`,
		"no current":    `[{},{},{}]`,
		"not json":      `<html>Bad gateway</html>`,
		"too few days":  `{"current":{"temperature_2m":1,"weather_code":0,"is_day":1},"daily":{"time":["2026-10-13"]}}`,
		"a missing max": `{"current":{"temperature_2m":1,"weather_code":0,"is_day":1},"daily":{"time":["a","b","c","d","e"],"weather_code":[0,0,0,0,0],"temperature_2m_max":[1,null,1,1,1],"temperature_2m_min":[0,0,0,0,0]}}`,
	} {
		c := cities
		if strings.HasPrefix(body, "{") && name != "refused" {
			c = cities[:1]
		}
		if _, err := parseForecast([]byte(body), c); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
	_, err := parseForecast([]byte(`{"error":true,"reason":"Cannot initialize WeatherVariable"}`), cities)
	if err == nil || !strings.Contains(err.Error(), "Cannot initialize WeatherVariable") {
		t.Errorf("Open-Meteo's reason is lost: %v", err)
	}
}

func TestForecastURL(t *testing.T) {
	u, err := url.Parse(forecastURL(DefaultCities()[:2], "Europe/Prague"))
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	for key, want := range map[string]string{
		"latitude": "50.0880,49.1952", "longitude": "14.4208,16.6080", "timezone": "Europe/Prague",
		"wind_speed_unit": "ms", "forecast_days": "5", "hourly": "weather_code", "current": "temperature_2m,weather_code,is_day",
		"daily": "weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max",
	} {
		if got := q.Get(key); got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestDaytimeCode(t *testing.T) {
	repeat := func(code, n int) []int { return slices.Repeat([]int{code}, n) }
	for _, c := range []struct {
		name  string
		hours []int
		want  int
	}{
		{"no hours keep the daily code", nil, 63},
		{"clear", repeat(0, 10), 0},
		{"cloud cover averages", []int{0, 1, 1, 2, 2, 2, 3, 3, 3, 3}, 2},
		{"half up", []int{1, 1, 1, 1, 1, 2, 2, 2, 2, 2}, 2},
		{"morning fog does not count", append(repeat(45, 4), repeat(0, 6)...), 0},
		{"fog half the day", append(repeat(45, 3), append(repeat(48, 2), repeat(1, 5)...)...), 45},
		{"rime fog most of the day", append(repeat(48, 6), repeat(45, 4)...), 48},
		{"a shower", append([]int{81}, repeat(2, 9)...), 80},
		{"two hours of drizzle", append(repeat(53, 2), repeat(3, 8)...), 51},
		{"a snow shower", append([]int{73}, repeat(3, 9)...), 85},
		{"freezing rain stays", append([]int{66}, repeat(3, 9)...), 66},
		{"a wet day", append(repeat(61, 4), append(repeat(63, 2), repeat(3, 4)...)...), 61},
		{"a tie goes to the heavier", append(repeat(61, 3), append(repeat(63, 3), repeat(3, 4)...)...), 63},
		{"a lone storm is a shower", append([]int{95}, repeat(2, 9)...), 80},
		{"a stormy afternoon", append(repeat(95, 2), append(repeat(80, 2), repeat(2, 6)...)...), 95},
		{"hail", append(repeat(96, 2), repeat(95, 1)...), 96},
	} {
		if got := daytimeCode(c.hours, 63); got != c.want {
			t.Errorf("%s: %d, want %d", c.name, got, c.want)
		}
	}
}

func TestNewestCAP(t *testing.T) {
	// names reuse the day of the month: 31 Aug sorts last and 11 Jan is newer than 1 Jun
	// 2025, but 11 Oct is the newest; the drought report (alert_cap_70) is no warning
	name, err := newestCAP(readFixture(t, "cap-listing.html"))
	if err != nil || name != "alert_cap_50_110758.xml" {
		t.Errorf("newest: %q, %v", name, err)
	}
	if _, err := newestCAP([]byte("<html><pre></pre></html>")); err == nil {
		t.Error("an empty directory gave no error")
	}
}

func TestReadCAP(t *testing.T) {
	alert, err := readCAP(context.Background(), fixtureFetcher())
	if err != nil {
		t.Fatal(err)
	}
	if want := time.Date(2026, 10, 8, 8, 42, 52, 0, time.UTC); !alert.issued.Equal(want) || len(alert.Infos) != 6 {
		t.Errorf("issued %s with %d blocks", alert.issued, len(alert.Infos))
	}
}

// ČHMÚ's state of 8 Oct 2026 10:42: strong wind from 16:00 in the north-west and from
// 20:00 in the south and east, until 01:00 and 02:00.
func TestWarnings(t *testing.T) {
	alert, err := parseCAP(readFixture(t, "cap-wind.xml"))
	if err != nil {
		t.Fatal(err)
	}
	prague, _ := time.LoadLocation("Europe/Prague")
	at := func(day, hour int) time.Time { return time.Date(2026, 10, day, hour, 0, 0, 0, prague) }

	got := warningsOf(alert, at(8, 12), prague)
	if len(got) != 1 {
		t.Fatalf("%d warnings: %+v", len(got), got)
	}
	w := got[0]
	if w.Level != "yellow" || w.Event != "Silný vítr" || w.EventEn != "Moderate Wind Related Impact" {
		t.Errorf("warning: %s %q %q", w.Level, w.Event, w.EventEn)
	}
	if !w.Onset.Equal(at(8, 16)) || w.Expires == nil || !w.Expires.Equal(at(9, 2)) {
		t.Errorf("in force from %s to %v", w.Onset, w.Expires)
	}
	var regions []string
	for _, r := range w.Regions {
		name := r.Code + " " + r.Name
		if r.Partial {
			name += " (part)"
		}
		regions = append(regions, name)
	}
	want := []string{"CZ-31 Jihočeský kraj (part)", "CZ-41 Karlovarský kraj (part)", "CZ-42 Ústecký kraj (part)",
		"CZ-53 Pardubický kraj (part)", "CZ-63 Kraj Vysočina", "CZ-64 Jihomoravský kraj (part)", "CZ-71 Olomoucký kraj (part)"}
	if !slices.Equal(regions, want) {
		t.Errorf("regions:\n%q\nwant\n%q", regions, want)
	}
	if w.Regions[4].NameEn != "Vysočina Region" {
		t.Errorf("English name %q", w.Regions[4].NameEn)
	}

	// still in force in the evening, gone in the night after, not yet the day before
	if got := warningsOf(alert, at(8, 23), prague); len(got) != 1 {
		t.Errorf("at 23:00: %d warnings", len(got))
	}
	if got := warningsOf(alert, at(9, 3), prague); len(got) != 0 {
		t.Errorf("at 03:00 the next night: %+v", got)
	}
	if got := warningsOf(alert, at(7, 12), prague); len(got) != 0 {
		t.Errorf("the day before: %+v", got)
	}

	alert.Status = "Exercise"
	if got := warningsOf(alert, at(8, 12), prague); len(got) != 0 {
		t.Errorf("an exercise warned: %+v", got)
	}
}

// The orange heat warning of 31 Jul 2026 over Praha (the sample keeps 2 of 82 blocks).
func TestWarningsOrange(t *testing.T) {
	alert, err := parseCAP(readFixture(t, "cap-heat.xml"))
	if err != nil {
		t.Fatal(err)
	}
	prague, _ := time.LoadLocation("Europe/Prague")
	got := warningsOf(alert, time.Date(2026, 7, 31, 15, 0, 0, 0, prague), prague)
	if len(got) != 1 {
		t.Fatalf("%d warnings", len(got))
	}
	w := got[0]
	if w.Level != "orange" || w.Event != "Velmi vysoké teploty" || w.EventEn != "Major Heat Related Impact" ||
		w.Expires == nil || !w.Expires.Equal(time.Date(2026, 8, 1, 0, 0, 0, 0, prague)) {
		t.Errorf("warning: %+v", w)
	}
	if len(w.Regions) != 1 || w.Regions[0].Code != "CZ-10" || w.Regions[0].Partial {
		t.Errorf("regions: %+v", w.Regions)
	}
}

func TestWarningsQuietDay(t *testing.T) {
	alert, err := parseCAP(readFixture(t, "cap-quiet.xml"))
	if err != nil {
		t.Fatal(err)
	}
	prague, _ := time.LoadLocation("Europe/Prague")
	if got := warningsOf(alert, time.Date(2026, 10, 11, 12, 0, 0, 0, prague), prague); got == nil || len(got) != 0 {
		t.Errorf("a quiet day warned: %+v", got)
	}
}

func TestCzechiaOutline(t *testing.T) {
	points, err := CzechiaOutline()
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 644 || points[0] != points[len(points)-1] {
		t.Errorf("%d points, closed: %v", len(points), points[0] == points[len(points)-1])
	}
	for _, p := range points {
		if p[0] < 12 || p[0] > 18.9 || p[1] < 48.5 || p[1] > 51.1 {
			t.Fatalf("a point outside Czechia: %v", p)
		}
	}
}
