package couchtv

import (
	"fmt"
	"math"
	"time"
)

// Clip kinds of the bulletins, one per kind of bulletin (couchtv_clips.kind).
const (
	ClipWeather = BulletinWeather
	ClipNews    = BulletinNews
)

// newsCredit is the news' credit line: the headlines are the outlets' own (D13 keeps them
// Czech), read from their feeds at a time.
const newsCredit = "Headlines from the outlets’ RSS feeds, as of %s"

// weatherSpec is the weather bulletin's spec (spec.ts WeatherSpec). Its days are named from
// the day it was fetched, which is the day it airs: a forecast is never valid past midnight.
func weatherSpec(w CouchTvWeather) Spec {
	loc, err := time.LoadLocation(w.TimeZone)
	if err != nil {
		loc = time.UTC
	}
	fetched := civil(w.FetchedAt.In(loc))
	cities := make([]SpecCity, len(w.Cities))
	for i, c := range w.Cities {
		city := SpecCity{Name: c.Name, Lat: c.Lat, Lon: c.Lon,
			Now:  SpecWeatherNow{Temperature: degrees(c.Now.Temperature), Code: c.Now.Code, Day: c.Now.IsDay},
			Days: make([]SpecWeatherDay, 0, len(c.Days))}
		for _, d := range c.Days {
			date, err := time.Parse(time.DateOnly, d.Date)
			if err != nil {
				continue
			}
			weekday := date.Weekday().String()
			day := SpecWeatherDay{Label: weekday, Weekday: weekday, Short: weekday[:3], Min: degrees(d.Min), Max: degrees(d.Max),
				Code: d.Code, Precipitation: d.PrecipitationProbability}
			switch int(math.Round(date.Sub(fetched).Hours() / 24)) {
			case 0:
				day.Label = "Today"
			case 1:
				day.Label = "Tomorrow"
			}
			city.Days = append(city.Days, day)
		}
		cities[i] = city
	}
	spec := Spec{Template: "weather", Duration: frameSeconds(weatherFrames), AsOf: localTime(w.FetchedAt, loc), Cities: cities,
		Credits: &SpecCredits{Forecast: WeatherAttribution, Warnings: WarningsAttribution, Borders: OutlineAttribution}}
	if w.Warnings != nil {
		warnings := make([]SpecWarning, 0, len(w.Warnings.Items))
		when := func(t time.Time) SpecTime {
			l := t.In(loc)
			if civil(l).Equal(fetched) {
				return SpecTime{Time: l.Format("15:04")}
			}
			return SpecTime{Day: l.Weekday().String(), Time: l.Format("15:04")}
		}
		for _, x := range w.Warnings.Items {
			warning := SpecWarning{Level: x.Level, Event: x.EventEn, Active: !x.Onset.After(w.FetchedAt), From: when(x.Onset),
				Regions: make([]SpecRegion, len(x.Regions))}
			if x.Expires != nil {
				until := when(*x.Expires)
				warning.Until = &until
			}
			for j, r := range x.Regions {
				warning.Regions[j] = SpecRegion{Code: r.Code, Name: r.NameEn, Partial: r.Partial}
			}
			warnings = append(warnings, warning)
		}
		spec.Warnings = &warnings
	}
	return spec
}

// newsSpec is the news bulletin's spec (spec.ts NewsSpec): the headlines whole, each with its
// outlet's name, and the credit line as of the time they were read in loc.
func newsSpec(n CouchTvNews, loc *time.Location) Spec {
	asOf := localTime(n.FetchedAt, loc)
	stories := make([]SpecStory, len(n.Stories))
	for i, s := range n.Stories {
		stories[i] = SpecStory{Headline: s.Headline, Outlet: s.OutletName, Outlets: max(1, len(s.Coverage)), Trending: s.Trending}
	}
	return Spec{Template: "news", Duration: frameSeconds(newsFrames), AsOf: asOf, Stories: stories,
		Credit: fmt.Sprintf(newsCredit, asOf)}
}

// degrees rounds a temperature to whole degrees, as the bulletins show it.
func degrees(c float64) int { return int(math.Round(c)) }
