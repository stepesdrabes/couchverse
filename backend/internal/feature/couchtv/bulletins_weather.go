package couchtv

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"math"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// The weather bulletin: Open-Meteo's forecast, one request for every city, and ČHMÚ's
// warnings (plan 6.10).
const (
	openMeteoURL = "https://api.open-meteo.com/v1/forecast"
	// WeatherAttribution is the credit Open-Meteo's licence (CC BY 4.0) asks for
	// wherever its data is shown.
	WeatherAttribution = "Weather data by Open-Meteo.com"
	// WarningsAttribution names ČHMÚ, whose open data asks for its source.
	WarningsAttribution = "Zdroj: ČHMÚ"
	forecastDays        = 5
	// a day's symbol shows the weather of its daytime hours, 09:00 to 18:00
	daytimeFrom, daytimeTo = 9, 18
)

// forecastURL asks for the current weather, the hourly codes the daytime symbols come
// from and five days of the rest, in the channel's zone; wind (none asked for yet) in m/s,
// as Czech forecasts state it.
func forecastURL(cities []CouchTvCity, timeZone string) string {
	lat := make([]string, len(cities))
	lon := make([]string, len(cities))
	for i, c := range cities {
		lat[i] = strconv.FormatFloat(c.Lat, 'f', 4, 64)
		lon[i] = strconv.FormatFloat(c.Lon, 'f', 4, 64)
	}
	q := url.Values{}
	q.Set("latitude", strings.Join(lat, ","))
	q.Set("longitude", strings.Join(lon, ","))
	q.Set("current", "temperature_2m,weather_code,is_day")
	q.Set("hourly", "weather_code")
	q.Set("daily", "weather_code,temperature_2m_max,temperature_2m_min,precipitation_probability_max")
	q.Set("timezone", timeZone)
	q.Set("forecast_days", strconv.Itoa(forecastDays))
	q.Set("wind_speed_unit", "ms")
	return openMeteoURL + "?" + q.Encode()
}

// forecastLocation is one location of Open-Meteo's answer. Every value may be null where
// the model has none.
type forecastLocation struct {
	Current struct {
		Temperature *float64 `json:"temperature_2m"`
		Code        *float64 `json:"weather_code"`
		IsDay       *float64 `json:"is_day"`
	} `json:"current"`
	Hourly struct {
		Time []string   `json:"time"`
		Code []*float64 `json:"weather_code"`
	} `json:"hourly"`
	Daily struct {
		Time   []string   `json:"time"`
		Code   []*float64 `json:"weather_code"`
		Max    []*float64 `json:"temperature_2m_max"`
		Min    []*float64 `json:"temperature_2m_min"`
		Precip []*float64 `json:"precipitation_probability_max"`
	} `json:"daily"`
}

// parseForecast reads Open-Meteo's answer for cities: an array in the order they were
// asked for (only the first carries no location_id), or one object for a single city.
// Positions stay the configured ones, not the grid cells the model used.
func parseForecast(body []byte, cities []CouchTvCity) ([]CouchTvWeatherCity, error) {
	body = bytes.TrimSpace(body)
	var locs []forecastLocation
	if len(body) > 0 && body[0] == '{' {
		var answer struct {
			Error  bool   `json:"error"`
			Reason string `json:"reason"`
		}
		if err := json.Unmarshal(body, &answer); err != nil {
			return nil, err
		}
		if answer.Error {
			return nil, fmt.Errorf("Open-Meteo refused: %s", answer.Reason)
		}
		var one forecastLocation
		if err := json.Unmarshal(body, &one); err != nil {
			return nil, err
		}
		locs = []forecastLocation{one}
	} else if err := json.Unmarshal(body, &locs); err != nil {
		return nil, err
	}
	if len(locs) != len(cities) {
		return nil, fmt.Errorf("%d locations for %d cities", len(locs), len(cities))
	}
	out := make([]CouchTvWeatherCity, len(cities))
	for i, loc := range locs {
		city, err := loc.city(cities[i])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", cities[i].Name, err)
		}
		out[i] = city
	}
	return out, nil
}

func (l forecastLocation) city(c CouchTvCity) (CouchTvWeatherCity, error) {
	cur := l.Current
	if cur.Temperature == nil || cur.Code == nil || cur.IsDay == nil {
		return CouchTvWeatherCity{}, errors.New("no current weather")
	}
	out := CouchTvWeatherCity{Name: c.Name, Lat: c.Lat, Lon: c.Lon,
		Now:  CouchTvWeatherNow{Temperature: *cur.Temperature, Code: wmoCode(*cur.Code), IsDay: *cur.IsDay == 1},
		Days: make([]CouchTvWeatherDay, 0, forecastDays)}
	daytime := map[string][]int{}
	for i, t := range l.Hourly.Time {
		if i >= len(l.Hourly.Code) || l.Hourly.Code[i] == nil || len(t) < len("2006-01-02T15") {
			continue
		}
		if h, err := strconv.Atoi(t[11:13]); err == nil && h >= daytimeFrom && h <= daytimeTo {
			daytime[t[:10]] = append(daytime[t[:10]], wmoCode(*l.Hourly.Code[i]))
		}
	}
	d := l.Daily
	if len(d.Time) < forecastDays {
		return out, fmt.Errorf("%d days of forecast", len(d.Time))
	}
	for i, date := range d.Time[:forecastDays] {
		if i >= len(d.Code) || i >= len(d.Max) || i >= len(d.Min) || d.Code[i] == nil || d.Max[i] == nil || d.Min[i] == nil {
			return out, fmt.Errorf("no forecast for %s", date)
		}
		day := CouchTvWeatherDay{Date: date, Min: *d.Min[i], Max: *d.Max[i], Code: daytimeCode(daytime[date], wmoCode(*d.Code[i]))}
		if i < len(d.Precip) && d.Precip[i] != nil {
			p := int(math.Round(*d.Precip[i]))
			day.PrecipitationProbability = &p
		}
		out.Days = append(out.Days, day)
	}
	return out, nil
}

// wmoCode keeps a code within the WMO table's range.
func wmoCode(v float64) int {
	return min(max(int(math.Round(v)), 0), 99)
}

// daytimeCode is the weather a day's daytime hours mostly have. The source's daily code
// is the day's most severe hour, so a 0.1 mm shower would make a sunny day rainy and
// morning fog a foggy one. Two stormy hours make a stormy day, three wet ones a wet day
// (its most frequent kind), one or two wet ones a day of showers, fog for half the hours a
// foggy one; otherwise the cloud cover the dry hours average to. Without hourly codes the
// daily one stands.
func daytimeCode(hours []int, daily int) int {
	if len(hours) == 0 {
		return daily
	}
	var storms, wet, fog []int
	cloud, dry := 0, 0
	for _, c := range hours {
		switch {
		case c >= 95:
			storms = append(storms, c)
			wet = append(wet, c)
		case c >= 51:
			wet = append(wet, c)
		case c == 45 || c == 48:
			fog = append(fog, c)
		default:
			cloud += min(c, 3)
			dry++
		}
	}
	switch {
	case len(storms) >= 2:
		return mostCommon(storms)
	case len(wet) >= 3:
		return mostCommon(wet)
	case len(wet) > 0:
		return showersOf(mostCommon(wet))
	case 2*len(fog) >= len(hours):
		return mostCommon(fog)
	}
	return int(math.Floor(float64(cloud)/float64(dry) + 0.5))
}

// mostCommon is the most frequent code, the more severe (higher) one on a tie.
func mostCommon(codes []int) int {
	count := map[int]int{}
	best := codes[0]
	for _, c := range codes {
		count[c]++
		if count[c] > count[best] || count[c] == count[best] && c > best {
			best = c
		}
	}
	return best
}

// showersOf is the passing form of a wet code: light drizzle, snow showers or rain
// showers; freezing precipitation stays what it is.
func showersOf(c int) int {
	switch {
	case c >= 51 && c <= 55:
		return 51
	case c == 56 || c == 57 || c == 66 || c == 67:
		return c
	case c >= 71 && c <= 77 || c == 85 || c == 86:
		return 85
	}
	return 80
}

// capDir is ČHMÚ's directory of CAP messages. It has no "latest" name: each SIVS message
// (alert_cap_50_DDHHMM.xml, the time UTC) is the whole state of warnings, a new one comes
// most mornings and with every change, and names reuse the day of the month.
const capDir = "https://opendata.chmi.cz/meteorology/weather/alerts/cap/"

// capListed is a SIVS message in the directory's listing (alert_cap_70 is the drought
// report) and the date the listing gives it.
var capListed = regexp.MustCompile(`href="(alert_cap_50_\d{6}\.xml)"[^\n]*?\s(\d{2}-[A-Z][a-z]{2}-\d{4} \d{2}:\d{2})\s`)

// newestCAP is the newest SIVS message by the listing's date, its name only telling the
// day of the month.
func newestCAP(listing []byte) (string, error) {
	best, bestAt := "", time.Time{}
	for _, m := range capListed.FindAllSubmatch(listing, -1) {
		at, err := time.Parse("02-Jan-2006 15:04", string(m[2]))
		if err != nil {
			continue
		}
		if name := string(m[1]); at.After(bestAt) || at.Equal(bestAt) && name > best {
			best, bestAt = name, at
		}
	}
	if best == "" {
		return "", errors.New("the directory lists no warning message")
	}
	return best, nil
}

// readCAP reads ČHMÚ's newest state of warnings.
func readCAP(ctx context.Context, f *fetcher) (capAlert, error) {
	listing, err := f.get(ctx, capDir)
	if err != nil {
		return capAlert{}, err
	}
	name, err := newestCAP(listing)
	if err != nil {
		return capAlert{}, err
	}
	body, err := f.get(ctx, capDir+name)
	if err != nil {
		return capAlert{}, err
	}
	return parseCAP(body)
}

// capAlert is a CAP 1.2 message as ČHMÚ sends it: an info block per event, area and
// language (cs, en-GB), on quiet days "no warning" blocks of severity Minor.
type capAlert struct {
	Sent   string    `xml:"sent"`
	Status string    `xml:"status"`
	Infos  []capInfo `xml:"info"`
	issued time.Time
}

type capInfo struct {
	Language   string     `xml:"language"`
	Event      string     `xml:"event"`
	Onset      string     `xml:"onset"`
	Expires    string     `xml:"expires"`
	EventCodes []capValue `xml:"eventCode"`
	Parameters []capValue `xml:"parameter"`
	Areas      []capArea  `xml:"area"`
}

type capValue struct {
	Name  string `xml:"valueName"`
	Value string `xml:"value"`
}

// capArea names a kraj, a kraj with the districts (ORP) it covers in brackets, or an
// air-quality zone; its CISORP geocodes are the districts, whose code starts with the
// kraj's.
type capArea struct {
	Desc     string     `xml:"areaDesc"`
	Geocodes []capValue `xml:"geocode"`
}

func parseCAP(body []byte) (capAlert, error) {
	var a capAlert
	if err := xml.Unmarshal(body, &a); err != nil {
		return a, err
	}
	sent, err := time.Parse(time.RFC3339, strings.TrimSpace(a.Sent))
	if err != nil {
		return a, fmt.Errorf("a warning message sent %q", a.Sent)
	}
	a.issued = sent
	return a, nil
}

func capValueOf(values []capValue, name string) string {
	for _, v := range values {
		if strings.TrimSpace(v.Name) == name {
			return strings.TrimSpace(v.Value)
		}
	}
	return ""
}

// capLevels are the awareness levels a bulletin carries ("2; yellow; Moderate"); level 1
// is ČHMÚ's "no warning".
var capLevels = map[string]string{"2": "yellow", "3": "orange", "4": "red"}

// kraj is a region the warnings name, with the first two digits of its districts'
// CISORP codes and how many districts it has.
type kraj struct {
	code, name, nameEn string
	prefix             string
	districts          int
}

var krajs = []kraj{
	{"CZ-10", "Hlavní město Praha", "Prague", "11", 1},
	{"CZ-20", "Středočeský kraj", "Central Bohemian Region", "21", 26},
	{"CZ-31", "Jihočeský kraj", "South Bohemian Region", "31", 17},
	{"CZ-32", "Plzeňský kraj", "Plzeň Region", "32", 15},
	{"CZ-41", "Karlovarský kraj", "Karlovy Vary Region", "41", 7},
	{"CZ-42", "Ústecký kraj", "Ústí nad Labem Region", "42", 16},
	{"CZ-51", "Liberecký kraj", "Liberec Region", "51", 10},
	{"CZ-52", "Královéhradecký kraj", "Hradec Králové Region", "52", 15},
	{"CZ-53", "Pardubický kraj", "Pardubice Region", "53", 15},
	{"CZ-63", "Kraj Vysočina", "Vysočina Region", "61", 15},
	{"CZ-64", "Jihomoravský kraj", "South Moravian Region", "62", 21},
	{"CZ-71", "Olomoucký kraj", "Olomouc Region", "71", 13},
	{"CZ-72", "Zlínský kraj", "Zlín Region", "72", 13},
	{"CZ-80", "Moravskoslezský kraj", "Moravian-Silesian Region", "81", 22},
}

var levelRank = map[string]int{"yellow": 1, "orange": 2, "red": 3}

// warningsOf are the alert's yellow, orange and red warnings in force now or later today
// in loc: an event at a level is one warning, its areas and times joined, its name taken
// from its Czech and its English block.
func warningsOf(a capAlert, now time.Time, loc *time.Location) []CouchTvWarning {
	out := []CouchTvWarning{}
	if a.Status != "Actual" {
		return out
	}
	l := now.In(loc)
	tomorrow := time.Date(l.Year(), l.Month(), l.Day()+1, 0, 0, 0, 0, loc)
	type group struct {
		warning   CouchTvWarning
		districts map[string]bool
	}
	groups := map[string]*group{}
	var order []string
	for _, info := range a.Infos {
		level := capLevels[strings.TrimSpace(strings.Split(capValueOf(info.Parameters, "awareness_level"), ";")[0])]
		onset, err := time.Parse(time.RFC3339, strings.TrimSpace(info.Onset))
		if level == "" || err != nil {
			continue
		}
		var expires *time.Time
		if e := strings.TrimSpace(info.Expires); e != "" {
			t, err := time.Parse(time.RFC3339, e)
			if err != nil {
				continue
			}
			expires = &t
		}
		if expires != nil && !expires.After(now) || !onset.Before(tomorrow) {
			continue
		}
		event := capValueOf(info.EventCodes, "SIVS")
		if event == "" {
			event = capValueOf(info.Parameters, "awareness_type")
		}
		key := level + "|" + event
		g := groups[key]
		if g == nil {
			g = &group{warning: CouchTvWarning{Level: level, Onset: onset, Expires: expires}, districts: map[string]bool{}}
			groups[key] = g
			order = append(order, key)
		}
		w := &g.warning
		switch name := strings.TrimSpace(info.Event); {
		case strings.HasPrefix(info.Language, "cs") && w.Event == "":
			w.Event = name
		case strings.HasPrefix(info.Language, "en") && w.EventEn == "":
			w.EventEn = name
		}
		if onset.Before(w.Onset) {
			w.Onset = onset
		}
		if w.Expires != nil && (expires == nil || expires.After(*w.Expires)) {
			w.Expires = expires
		}
		for _, area := range info.Areas {
			for _, code := range area.Geocodes {
				if strings.TrimSpace(code.Name) == "CISORP" {
					g.districts[strings.TrimSpace(code.Value)] = true
				}
			}
		}
	}
	for _, key := range order {
		g := groups[key]
		w := g.warning
		w.Regions = regionsOf(g.districts)
		if len(w.Regions) == 0 {
			continue
		}
		if w.Event == "" {
			w.Event = w.EventEn
		}
		if w.EventEn == "" {
			w.EventEn = w.Event
		}
		out = append(out, w)
	}
	slices.SortStableFunc(out, func(a, b CouchTvWarning) int {
		if d := levelRank[b.Level] - levelRank[a.Level]; d != 0 {
			return d
		}
		return a.Onset.Compare(b.Onset)
	})
	return out
}

// regionsOf are the kraje the districts lie in; a kraj not warned in every district is
// partial.
func regionsOf(districts map[string]bool) []CouchTvWarningRegion {
	count := map[string]int{}
	for d := range districts {
		if len(d) == 4 {
			count[d[:2]]++
		}
	}
	out := []CouchTvWarningRegion{}
	for _, k := range krajs {
		if n := count[k.prefix]; n > 0 {
			out = append(out, CouchTvWarningRegion{Code: k.code, Name: k.name, NameEn: k.nameEn, Partial: n < k.districts})
		}
	}
	return out
}

// czechiaGeoJSON is Czechia's border for the weather map: geoBoundaries' CZE ADM0
// outline (gbOpen, boundary data by ČÚZK, CC BY 4.0), simplified to 643 points by
// Ramer-Douglas-Peucker at 2 px with the country 1600 px wide. Wherever the map is shown
// it is credited with OutlineAttribution.
//
//go:embed czechia.geojson
var czechiaGeoJSON []byte

// OutlineAttribution is the credit the border's licence asks for.
const OutlineAttribution = "Hranice: ČÚZK / geoBoundaries (CC BY 4.0)"

// CzechiaOutline is Czechia's border as [longitude, latitude] points, closed: the last
// point is the first.
func CzechiaOutline() ([][2]float64, error) {
	var doc struct {
		Features []struct {
			Geometry struct {
				Type        string         `json:"type"`
				Coordinates [][][2]float64 `json:"coordinates"`
			} `json:"geometry"`
		} `json:"features"`
	}
	if err := json.Unmarshal(czechiaGeoJSON, &doc); err != nil {
		return nil, err
	}
	if len(doc.Features) != 1 || doc.Features[0].Geometry.Type != "Polygon" || len(doc.Features[0].Geometry.Coordinates) == 0 {
		return nil, errors.New("the outline is not one polygon")
	}
	return doc.Features[0].Geometry.Coordinates[0], nil
}
