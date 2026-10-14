package couchtv

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"
)

// The weather of 13 Oct 17:02 as the bulletin airs it: English days from the day it was
// fetched, whole degrees, the warnings in English with their times, the credits.
func TestWeatherSpec(t *testing.T) {
	prague, _ := time.LoadLocation("Europe/Prague")
	cities, err := parseForecast(readBulletinFixture(t, "openmeteo.json"), DefaultCities()[:3])
	if err != nil {
		t.Fatal(err)
	}
	fetched := time.Date(2026, 10, 13, 17, 2, 0, 0, prague)
	w := CouchTvWeather{FetchedAt: fetched, ValidUntil: weatherUntil(fetched, prague), TimeZone: "Europe/Prague", Cities: cities,
		Warnings: &CouchTvWarnings{Items: []CouchTvWarning{}}}
	spec := weatherSpec(w)
	if err := validateSpec(spec); err != nil {
		t.Fatal(err)
	}
	prg := spec.Cities[0]
	if spec.AsOf != "17:02" || spec.Duration != frameSeconds(weatherFrames) || prg.Name != "Prague" || prg.Now.Temperature != 15 || !prg.Now.Day {
		t.Errorf("the spec: %+v", spec)
	}
	var labels []string
	for _, d := range prg.Days {
		labels = append(labels, d.Label+"/"+d.Short)
	}
	if want := []string{"Today/Tue", "Tomorrow/Wed", "Thursday/Thu", "Friday/Fri", "Saturday/Sat"}; !slices.Equal(labels, want) {
		t.Errorf("days %v, want %v", labels, want)
	}
	if d := prg.Days[1]; d.Min != 10 || d.Max != 17 || d.Precipitation == nil || *d.Precipitation != 70 {
		t.Errorf("tomorrow %+v", d)
	}
	if c := spec.Credits; c.Forecast != "Weather data by Open-Meteo.com" || c.Warnings != "Source: ČHMÚ" ||
		c.Borders != "Borders: ČÚZK / geoBoundaries (CC BY 4.0)" {
		t.Errorf("credits %+v", c)
	}
	if spec.Warnings == nil || len(*spec.Warnings) != 0 {
		t.Errorf("a quiet day's warnings: %v", spec.Warnings)
	}

	// ČHMÚ unread: no warnings at all
	w.Warnings = nil
	if spec := weatherSpec(w); spec.Warnings != nil || validateSpec(spec) != nil {
		t.Errorf("unread warnings: %+v", spec.Warnings)
	}

	// the wind warning of 8 Oct, fetched at noon: from 16:00 to Friday 02:00, in English
	alert, err := parseCAP(readBulletinFixture(t, "cap-wind.xml"))
	if err != nil {
		t.Fatal(err)
	}
	noon := time.Date(2026, 10, 8, 12, 0, 0, 0, prague)
	w.FetchedAt = noon
	w.Warnings = &CouchTvWarnings{Items: warningsOf(alert, noon, prague)}
	spec = weatherSpec(w)
	if err := validateSpec(spec); err != nil {
		t.Fatal(err)
	}
	wind := (*spec.Warnings)[0]
	if wind.Level != "yellow" || wind.Event != "Moderate Wind Related Impact" || wind.Active || wind.From != (SpecTime{Time: "16:00"}) ||
		wind.Until == nil || *wind.Until != (SpecTime{Day: "Friday", Time: "02:00"}) {
		t.Errorf("the wind warning %+v", wind)
	}
	if r := wind.Regions[4]; r.Code != "CZ-63" || r.Name != "Vysočina Region" || r.Partial {
		t.Errorf("a region %+v", r)
	}
	// in force by the evening
	w.FetchedAt = time.Date(2026, 10, 8, 19, 0, 0, 0, prague)
	if spec := weatherSpec(w); !(*spec.Warnings)[0].Active {
		t.Error("the warning is not in force at 19:00")
	}
}

// The news of 13 Oct 17:02 as the bulletin airs it: the headlines whole, labelled with their
// outlets, and the credit line.
func TestNewsSpec(t *testing.T) {
	prague, _ := time.LoadLocation("Europe/Prague")
	now := time.Date(2026, 10, 13, 17, 2, 0, 0, prague)
	fresh, trending, _ := fetchNews(context.Background(), fixtureFetcher())
	stories := rankNews(mergeNews(fresh, trending, now), now)
	spec := newsSpec(CouchTvNews{FetchedAt: now, ValidUntil: now.Add(3 * time.Hour), Stories: stories}, prague)
	if err := validateSpec(spec); err != nil {
		t.Fatal(err)
	}
	if spec.AsOf != "17:02" || spec.Duration != frameSeconds(newsFrames) || spec.Credit != "Headlines from the outlets’ RSS feeds, as of 17:02" ||
		len(spec.Stories) != len(stories) {
		t.Errorf("the spec: %+v", spec)
	}
	for i, s := range spec.Stories {
		if s.Headline != stories[i].Headline || s.Outlet != stories[i].OutletName || s.Outlets != len(stories[i].Coverage) {
			t.Errorf("story %d: %+v from %+v", i, s, stories[i])
		}
	}
	if top := spec.Stories[0]; top.Outlet != "ČT24" || !strings.HasPrefix(top.Headline, "Bulharsko hlásí další útok na lodě v"+string(noBreakSpace)) ||
		!top.Trending {
		t.Errorf("the top story %+v", top)
	}
}
