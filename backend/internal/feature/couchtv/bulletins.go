package couchtv

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"

	"couchverse/internal/feature/jobs"
	"couchverse/internal/settings"
)

// BulletinsJob keeps the channel's bulletins fresh (plan 6.10): the weather for the
// configured cities with ČHMÚ's warnings, and the day's top news headlines. Nothing airs
// them yet; the breaks will.
const BulletinsJob = "couchtv_bulletins"

// Kinds of bulletins, as couchtv_bulletins stores them.
const (
	BulletinWeather = "weather"
	BulletinNews    = "news"
)

// A bulletin is fetched every bulletinEvery of its kind while the day is awake, from
// awakeHour:awakeMinute to awakeFor later in the channel's zone (05:30 to 00:30), and
// airs until bulletinValid after it was fetched.
var (
	bulletinEvery = map[string]time.Duration{BulletinWeather: time.Hour, BulletinNews: 20 * time.Minute}
	bulletinValid = map[string]time.Duration{BulletinWeather: 6 * time.Hour, BulletinNews: 3 * time.Hour}
)

const (
	awakeHour, awakeMinute = 5, 30
	awakeFor               = 19 * time.Hour
	// bulletinPriority puts the runs behind the queue's other work: each is a few small
	// requests nobody waits for
	bulletinPriority = -5
	// the hourly cleanup keeps bulletinKeep snapshots of each kind and none older than
	// bulletinMaxAge, as long as Český rozhlas lets a reader keep its texts
	bulletinKeep   = 6
	bulletinMaxAge = 24 * time.Hour
)

// BulletinsPayload is a couchtv_bulletins run of one kind. A periodic run names its slot
// (At), so every run queues the same next one; the run queued at startup has none.
type BulletinsPayload struct {
	Kind string `json:"kind"`
	At   string `json:"at,omitempty"`
}

// CouchTvBulletins is the newest bulletin of each kind.
type CouchTvBulletins struct {
	Weather *CouchTvWeather `json:"weather,omitempty" doc:"The newest weather bulletin; absent before the first."`
	News    *CouchTvNews    `json:"news,omitempty" doc:"The newest news bulletin; absent before the first."`
}

// CouchTvWeather is a weather bulletin: Open-Meteo's forecast for each configured city and
// ČHMÚ's warnings.
type CouchTvWeather struct {
	FetchedAt   time.Time            `json:"fetchedAt"`
	ValidUntil  time.Time            `json:"validUntil" doc:"When the forecast is too old to air."`
	Attribution string               `json:"attribution" doc:"The credit Open-Meteo's licence (CC BY 4.0) asks for wherever the data is shown."`
	TimeZone    string               `json:"timeZone" doc:"The zone the days are dates in, the channel's."`
	Cities      []CouchTvWeatherCity `json:"cities" doc:"The configured cities, in their order."`
	Warnings    *CouchTvWarnings     `json:"warnings,omitempty" doc:"ČHMÚ's warnings; absent when they could not be read."`
}

// CouchTvWeatherCity is one city's weather.
type CouchTvWeatherCity struct {
	Name string              `json:"name"`
	Lat  float64             `json:"lat" doc:"Where the city is (the configured position, not the forecast's grid cell)."`
	Lon  float64             `json:"lon"`
	Now  CouchTvWeatherNow   `json:"now"`
	Days []CouchTvWeatherDay `json:"days" doc:"Today and the next four days."`
}

// CouchTvWeatherNow is the weather at the time of the forecast's latest model step.
type CouchTvWeatherNow struct {
	Temperature float64 `json:"temperature" doc:"Air temperature, °C."`
	Code        int     `json:"code" minimum:"0" maximum:"99" doc:"The WMO weather code (Open-Meteo's table)."`
	IsDay       bool    `json:"isDay" doc:"Daylight, for a sun or a moon."`
}

// CouchTvWeatherDay is one day's forecast.
type CouchTvWeatherDay struct {
	Date                     string  `json:"date" format:"date" doc:"The day, in the channel's zone."`
	Min                      float64 `json:"min" doc:"The day's lowest temperature, °C (midnight to midnight)."`
	Max                      float64 `json:"max" doc:"The day's highest temperature, °C."`
	Code                     int     `json:"code" minimum:"0" maximum:"99" doc:"The WMO code of the daytime, 09-18: what most of those hours are, where the source's daily code is the day's worst hour."`
	PrecipitationProbability *int    `json:"precipitationProbability" minimum:"0" maximum:"100" doc:"The day's highest chance of precipitation, %; null when the forecast has none."`
}

// CouchTvWarnings are ČHMÚ's weather warnings for today.
type CouchTvWarnings struct {
	IssuedAt    time.Time        `json:"issuedAt" doc:"When ČHMÚ issued the state of warnings this was read from."`
	Attribution string           `json:"attribution" doc:"The credit to show with them."`
	Items       []CouchTvWarning `json:"items" doc:"Yellow, orange and red warnings in force now or later today, the most severe first; empty on a quiet day."`
}

// CouchTvWarning is one kind of warning (ČHMÚ's SIVS event at one level) over the kraje it
// covers today.
type CouchTvWarning struct {
	Level   string                 `json:"level" enum:"yellow,orange,red"`
	Event   string                 `json:"event" doc:"The event in Czech, as ČHMÚ names it."`
	EventEn string                 `json:"eventEn" doc:"The event in English, as ČHMÚ names it."`
	Onset   time.Time              `json:"onset" doc:"When it begins (the earliest of its areas)."`
	Expires *time.Time             `json:"expires" doc:"When it ends (the latest of its areas); null until further notice."`
	Regions []CouchTvWarningRegion `json:"regions" doc:"The kraje it covers, by code."`
}

// CouchTvWarningRegion is a kraj a warning covers.
type CouchTvWarningRegion struct {
	Code    string `json:"code" enum:"CZ-10,CZ-20,CZ-31,CZ-32,CZ-41,CZ-42,CZ-51,CZ-52,CZ-53,CZ-63,CZ-64,CZ-71,CZ-72,CZ-80" doc:"The kraj's ISO 3166-2 code."`
	Name    string `json:"name" doc:"In Czech."`
	NameEn  string `json:"nameEn" doc:"In English."`
	Partial bool   `json:"partial" doc:"Only some of its districts (ORP) are warned."`
}

// CouchTvNews is a news bulletin: the most important stories of the last hours, ranked by
// how many outlets carry them.
type CouchTvNews struct {
	FetchedAt  time.Time          `json:"fetchedAt"`
	ValidUntil time.Time          `json:"validUntil" doc:"When the headlines are too old to air."`
	Stories    []CouchTvNewsStory `json:"stories" doc:"The top stories of the last 12 hours, most important first: 3 to 5."`
	Feeds      []CouchTvNewsFeed  `json:"feeds" doc:"What each source answered."`
}

// CouchTvNewsStory is one story, told by the headline of the outlet the channel prefers
// among those carrying it.
type CouchTvNewsStory struct {
	Headline    string    `json:"headline" doc:"The outlet's own headline, never shortened or reworded (Český rozhlas forbids it): markup and entities resolved, spaces normalised and a no-break space after a one-letter preposition or conjunction, so a template wraps it as Czech typesetting does."`
	Outlet      string    `json:"outlet" enum:"ct24,irozhlas,seznamzpravy,novinky,idnes" doc:"The outlet the headline is from."`
	OutletName  string    `json:"outletName" doc:"The outlet's name, the plain-text label to show with the headline (never its logo)."`
	URL         string    `json:"url" format:"uri" doc:"The article."`
	PublishedAt time.Time `json:"publishedAt"`
	Coverage    []string  `json:"coverage" enum:"ct24,irozhlas,seznamzpravy,novinky,idnes,cnnprima,denik" doc:"Every outlet carrying the story, those used only for ranking included."`
	EditorsPick bool      `json:"editorsPick" doc:"In ČT24's editors' selection."`
	Trending    bool      `json:"trending" doc:"Among ČT24's most read of the day."`
	Score       float64   `json:"score" doc:"The ranking: an outlet carrying it 1, the editors' selection 1.5 more, trending 1 more, eased by age."`
}

// CouchTvNewsFeed is how one source answered.
type CouchTvNewsFeed struct {
	Name  string `json:"name" doc:"The source, e.g. ČT24 or ČT24 trending."`
	Items int    `json:"items" doc:"The news items it carried, sport, culture, lifestyle, opinion and shows left out."`
	Error string `json:"error,omitempty" doc:"Why it could not be read; absent when it was."`
}

// Bulletins runs couchtv_bulletins.
type Bulletins struct {
	Store    *Store
	Settings *settings.Store
	Jobs     *jobs.Store
	// HTTP fetches the sources; nil uses a plain client (tests route its transport)
	HTTP *http.Client
	// Now is the clock; nil means time.Now (tests set it).
	Now func() time.Time

	once  sync.Once
	fetch *fetcher
	// alert is the last state of warnings ČHMÚ sent, used while it cannot be read
	alert *capAlert
	// seen are the news items of the last hours: Novinky's and iDNES's feeds reach back
	// two or three hours, so a story keeps counting the outlets that carried it after it
	// scrolled out of their feeds
	seen []newsItem
}

func (b *Bulletins) now() time.Time {
	if b.Now != nil {
		return b.Now()
	}
	return time.Now()
}

func (b *Bulletins) fetcher() *fetcher {
	b.once.Do(func() { b.fetch = newFetcher(b.HTTP) })
	return b.fetch
}

// Start queues a run of each kind now, which fetches only what went stale while the
// server was down, and starts their chains.
func (b *Bulletins) Start(ctx context.Context) error {
	for _, kind := range []string{BulletinWeather, BulletinNews} {
		if _, err := b.Jobs.EnqueueJobOnce(ctx, BulletinsJob, BulletinsPayload{Kind: kind},
			jobs.EnqueueOpts{Priority: bulletinPriority, MaxAttempts: 1}); err != nil {
			return err
		}
	}
	return nil
}

// Handle is a couchtv_bulletins run: it queues the kind's next run, then fetches the
// bulletin and stores it when it is good enough to air. A bad one leaves the last good
// one in place.
func (b *Bulletins) Handle(ctx context.Context, job *jobs.Job, _ func(int)) error {
	var p BulletinsPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return err
	}
	every, ok := bulletinEvery[p.Kind]
	if !ok {
		return fmt.Errorf("no bulletin of kind %q", p.Kind)
	}
	cfg := LoadConfig(ctx, b.Settings)
	now := b.now()
	// the next run is queued first, so a failing run cannot end the chain
	next := nextBulletin(now, cfg.location(), every)
	if _, err := b.Jobs.EnqueueJobOnce(ctx, BulletinsJob, BulletinsPayload{Kind: p.Kind, At: next.UTC().Format(time.RFC3339)},
		jobs.EnqueueOpts{RunAt: next, Priority: bulletinPriority, MaxAttempts: 1}); err != nil {
		return err
	}
	if !cfg.Bulletins.Enabled {
		return nil
	}
	if p.At == "" {
		fresh, err := b.Store.bulletinSince(ctx, p.Kind, now.Add(-every))
		if err != nil || fresh {
			return err
		}
	}
	if p.Kind == BulletinWeather {
		return b.weather(ctx, cfg, now)
	}
	return b.news(ctx, now)
}

// nextBulletin is the first run after now on a grid of every from the start of the awake
// hours in loc. The clocks change at night, so within those hours a wall-clock step is a
// step of real time.
func nextBulletin(now time.Time, loc *time.Location, every time.Duration) time.Time {
	l := now.In(loc)
	for d := -1; ; d++ {
		open := time.Date(l.Year(), l.Month(), l.Day()+d, awakeHour, awakeMinute, 0, 0, loc)
		if now.Before(open) {
			return open
		}
		if at := open.Add((now.Sub(open)/every + 1) * every); at.Sub(open) <= awakeFor {
			return at
		}
	}
}

// weather fetches and stores a weather bulletin.
func (b *Bulletins) weather(ctx context.Context, cfg CouchTvConfig, now time.Time) error {
	f := b.fetcher()
	body, err := f.get(ctx, forecastURL(cfg.Bulletins.Cities, cfg.TimeZone))
	if err != nil {
		return fmt.Errorf("the forecast: %w", err)
	}
	cities, err := parseForecast(body, cfg.Bulletins.Cities)
	if err != nil {
		return fmt.Errorf("the forecast: %w", err)
	}
	w := CouchTvWeather{FetchedAt: now, ValidUntil: now.Add(bulletinValid[BulletinWeather]), Attribution: WeatherAttribution,
		TimeZone: cfg.TimeZone, Cities: cities}
	if alert, err := readCAP(ctx, f); err != nil {
		slog.Warn("couchtv: ČHMÚ's warnings could not be read", "err", err)
	} else {
		b.alert = &alert
	}
	if b.alert != nil {
		w.Warnings = &CouchTvWarnings{IssuedAt: b.alert.issued, Attribution: WarningsAttribution,
			Items: warningsOf(*b.alert, now, cfg.location())}
	}
	return b.Store.saveBulletin(ctx, BulletinWeather, w.FetchedAt, w.ValidUntil, w)
}

// news fetches the feeds, ranks their stories and stores the bulletin when it has enough
// of them to air.
func (b *Bulletins) news(ctx context.Context, now time.Time) error {
	fresh, trending, feeds := fetchNews(ctx, b.fetcher())
	// what a feed says now comes first: an edited headline replaces the one seen before
	b.seen = mergeNews(slices.Concat(fresh, b.seen), trending, now)
	failed := 0
	for _, f := range feeds {
		if f.Error != "" {
			failed++
			slog.Warn("couchtv: a news source could not be read", "source", f.Name, "err", f.Error)
		}
	}
	if failed == len(feeds) {
		return errors.New("no news source could be read")
	}
	stories := rankNews(b.seen, now)
	if len(stories) < minStories {
		slog.Warn("couchtv: too few stories for a news bulletin, the last one stays", "stories", len(stories))
		return nil
	}
	n := CouchTvNews{FetchedAt: now, ValidUntil: now.Add(bulletinValid[BulletinNews]), Stories: stories, Feeds: feeds}
	return b.Store.saveBulletin(ctx, BulletinNews, n.FetchedAt, n.ValidUntil, n)
}

type bulletinsOutput struct{ Body CouchTvBulletins }

// Bulletins reads the newest bulletin of each kind.
func (h *Admin) Bulletins(ctx context.Context, _ *struct{}) (*bulletinsOutput, error) {
	var out CouchTvBulletins
	var w CouchTvWeather
	if ok, err := h.scheduler.Store.latestBulletin(ctx, BulletinWeather, &w); err != nil {
		return nil, err
	} else if ok {
		out.Weather = &w
	}
	var n CouchTvNews
	if ok, err := h.scheduler.Store.latestBulletin(ctx, BulletinNews, &n); err != nil {
		return nil, err
	} else if ok {
		out.News = &n
	}
	return &bulletinsOutput{Body: out}, nil
}

func (s *Store) saveBulletin(ctx context.Context, kind string, fetchedAt, validUntil time.Time, data any) error {
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	_, err = s.q.Exec(ctx, `INSERT INTO couchtv_bulletins (kind, fetched_at, valid_until, data) VALUES ($1, $2, $3, $4)`,
		kind, fetchedAt, validUntil, body)
	return err
}

// latestBulletin reads the newest bulletin of a kind into out, and says whether there is one.
func (s *Store) latestBulletin(ctx context.Context, kind string, out any) (bool, error) {
	var body []byte
	err := s.q.QueryRow(ctx, `SELECT data FROM couchtv_bulletins WHERE kind = $1 ORDER BY fetched_at DESC LIMIT 1`, kind).
		Scan(&body)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(body, out)
}

// bulletinSince says whether a bulletin of a kind was fetched after since.
func (s *Store) bulletinSince(ctx context.Context, kind string, since time.Time) (bool, error) {
	var fresh bool
	err := s.q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM couchtv_bulletins WHERE kind = $1 AND fetched_at > $2)`, kind, since).
		Scan(&fresh)
	return fresh, err
}

// PruneBulletins keeps the newest bulletinKeep bulletins of each kind and none older than
// bulletinMaxAge. It runs in the hourly cleanup.
func PruneBulletins(ctx context.Context, s *Store, now time.Time) error {
	_, err := s.q.Exec(ctx, `
		DELETE FROM couchtv_bulletins WHERE fetched_at < $1 OR id IN (
			SELECT id FROM (SELECT id, row_number() OVER (PARTITION BY kind ORDER BY fetched_at DESC) AS n
			                FROM couchtv_bulletins) newest WHERE n > $2)`, now.Add(-bulletinMaxAge), bulletinKeep)
	return err
}
