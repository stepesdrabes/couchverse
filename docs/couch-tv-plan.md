# Couch TV - plan

Status: **approved** (2026-10-12)
Scope: one auto-programmed, always-on linear channel ("Couch TV") with pre-rendered, animated
broadcast cutscenes, delivered as a live HLS stream; web first, native clients later.

This document is the single source of truth for the effort. It records every decision made
during planning (section 1), what viewers and admins get, the programming model, the
architecture (scheduling, analysis, rendering, the live stream, clients), a phased roadmap with
exit criteria, and the risks and spikes to resolve first. Facts measured during planning are
in section 3.

---

## 1. Decisions

| ID | Decision |
|---|---|
| D1 | **One flagship channel**, on screen **"Couch TV"**. Code name `couchtv` (the web's `lib/tv/` is TV *mode*, so the feature never uses the bare name `tv`). |
| D2 | **True live**: everyone sees the same moment; tuning in joins mid-program; no pause or seek. **"Watch from the start"** opens the title in the normal on-demand player (signed-in viewers only). |
| D3 | **Cutscenes only between programs.** Programs are never interrupted. |
| D4 | **Anchors + flow**: fixed anchor slots start exactly on time (e.g. 18:00, 20:15); between anchors programs flow back to back, each break padding to the next 5-minute mark or anchor. |
| D5 | **Series**: weekday **strips** that advance in order (a persisted cursor, e.g. Simpsonovi 2 episodes a day at 18:00) plus **weekend marathon** blocks of a drama. |
| D6 | **24/7 with dayparts** (morning, afternoon, strips, prime time, late, overnight reruns). |
| D7 | **Auto + admin editor**: generated a week ahead from a weekly template; the admin EPG can swap, pin, remove, insert, lock and regenerate a day. Locked slots survive regeneration. |
| D8 | **Web first** (desktop, phones, and the Titan OS TV as the same SPA). Channel logic lives in the shared core so Apple and Android follow later with only screens left to build. |
| D9 | **Cutscenes are pre-rendered video**, at the quality of the planning samples: the same HTML/CSS/Web Animations templates rendered by headless Chromium frame by frame, audio rendered by the same Web Audio graph in an `OfflineAudioContext`. |
| D10 | **One live HLS stream**: the server stitches program packages and rendered breaks into a live playlist with `EXT-X-PROGRAM-DATE-TIME` and discontinuities. Every client just plays one URL. |
| D11 | **Presence + emoji reactions** ("7 watching now", avatars, floating reactions). No chat. |
| D12 | Channel watching counts toward **Continue watching**, **watch-time analytics** (its own TV bucket) and **ranks XP + new achievements** (time-based, social, loyalty). |
| D13 | **Cutscenes are English only.** The surrounding web UI stays localized (en/cs). |
| D14 | **Live client overlays** over programs: corner bug with LIVE and the clock, a "Now" caption at program start, a "Next" caption near the end. The **credits squeeze** is pre-rendered into the break, composited with the program's last seconds. |
| D15 | **Renderer = a sidecar container on the Pi** (`couchverse-render`: the same Go binary in render mode plus Chromium, using the Pi's GPU), memory-capped, low priority, never rendering while a transcode runs. The main image stays slim. |
| D16 | **Output: 1080p, 25 fps**, H.264 High (libx264, CRF ~18) + AAC stereo, loudness-managed. |
| D17 | **Smart-mix breaks**: credits squeeze, short ident, 1-2 promos, up next; a lineup card in top-of-hour breaks; the full epic ident before the 20:15 movie; extra promos pad longer gaps. |
| D18 | **Promos use artwork + real footage**: 3-6 short shots from the title itself (scene detection, first half of the runtime only, to avoid spoilers), cut on the soundtrack's beats. |
| D19 | **Web surfaces**: "TV" in the top nav (`/tv`), a **Guide** (`/tv/guide`), a **Home "On now" card**. No TV-mode-specific remote features (the pages still follow the TV-mode rules: arrow-reachable, `data-tv-autofocus`). |
| D20 | **Audio: the viewer picks.** The stream carries Czech and English audio renditions where a title has them (dub or original), defaulting to the display language; a missing language falls back to the original. |
| D21 | **Subtitles where available**, as selectable renditions, off by default; the choice persists across programs. |
| D22 | **No time-of-day content rules.** (The admin can still take a title off the channel entirely, see 6.4.) |
| D23 | **Access: signed-in users and anyone with the channel link.** One unguessable link the admin can copy, rotate or turn off. Guests get the live channel only (no "Watch from the start", no library), appear in presence as "Guest" with a name they type, and can react. |
| D24 | **Saturday premieres**: a recently added movie that has never aired takes the Saturday 20:15 slot, promoted all week. |
| D25 | **Promo soundtrack: automatic + admin override.** Series: the theme found by fingerprinting episodes. Movies: the end-credits music (the steadiest loud stretch near the end). The title editor gets a "Promo audio" picker with a waveform. |
| D26 | **No announcer voice in v1**; the audio mix reserves a voice bus so a local TTS voice can be added later. |
| D27 | **Ident: a full cut (10-12 s) and a short cut (4-5 s)** from one design and one score: epic in scale and build, **clean and restrained in execution** (precise geometry, flat typography, no flares, particles or chrome; owner feedback on the samples). The theme is epic but modern: clean low end, precise hits, a memorable mnemonic. |
| D28 | **Time zone**: a channel setting, default `Europe/Prague`; schedule rows are stored in UTC. |
| D29 | **Gated** by an admin flag `couchTvEnabled` (mirrors `couchEnabled`), **off by default** until the channel has a rendered schedule. |
| D30 | **The default weekly template is 5.1**, including **Saga Sunday** (the next film of a franchise every Sunday at 20:15, in order). |
| D31 | **Branch**: a new `couch-tv` branch from `main`; `native-clients` is retired once its last docs commit is on `main`, so the remote keeps two branches (`main`, `couch-tv`). |
| D32 | **The planning sample reel** (HTML/CSS/JS + README) is kept in `tools/couch-tv-samples/` as the reference for Phase 2; its media (soundtrack excerpts, artwork) is gitignored and fetched from the Pi read-only by a script. |

---

## 2. Goals, non-goals, success criteria

**Goals**
- A channel that feels like real, premium television: appointment programming, strips,
  premieres, seamless cuts, cutscenes as polished as the planning samples, a recognizable
  sonic identity.
- Zero babysitting: the schedule, the analysis, the renders and the stream all run unattended
  on the Raspberry Pi 4, recover from missing files and slow renders, and never stall the
  stream.
- Cheap for the Pi: programs reuse the existing Playback v2 packages; rendered video is reused
  wherever the content allows; heavy work runs at low priority in bounded memory.
- Clean architecture in the project's style: a feature package per side, a pure schedule
  generator, a typed contract, scenario-tested core logic.

**Non-goals (v1)**
- More than one channel, per-user channels, chat, timeshift, ABR ladders on the channel, an
  announcer voice, native client screens, cutscenes in Czech.

**Success criteria**
- Two browsers (and the Titan OS TV) tuned in show the same frame within ~2 s, across program
  changes, for a week.
- Over a 7-day soak on the Pi: no stream gaps; every break rendered before air (or replaced by
  an evergreen fallback, logged); render work under ~3 h/day; no OOM; transcodes still
  progress.
- `validate-hls` passes on live playlist snapshots; hls.js and Safari play program-to-break
  transitions without a visible stall.
- The owner signs off on the ident, the theme and each promo type in the admin preview.

---

## 3. Facts measured during planning

- **Library** (production, 2026-10-11): 12 movies with files (Harry Potter 1-8, Iron Man 1-3,
  Pulp Fiction), series Simpsonovi (785 files), South Park (321), Dexter (96), Věřte nevěřte
  (45), Invincible (33). Series fill most hours; the prime-time movie repeats roughly every
  12 days, so rotation rules must scale with library size.
- **Theme detection works on the Pi's ffmpeg** (7.1, `chromaprint` muxer present): comparing
  the first minutes of two episodes found the shared theme in every series tried (Simpsonovi
  29 s at 0:21, South Park 36 s, Dexter 34 s, Invincible's 9 s title sting).
- **Credits music** shows up in a loudness profile (ebur128 momentary loudness per 10 s) as a
  long, high, low-variance stretch near the end (Iron Man, Harry Potter, Pulp Fiction); Pulp
  Fiction's opening titles stood out the same way.
- **Render speed on the Pi 4** (headless Chromium 154 in Docker, a 1080p promo, seek + JPEG
  capture per frame): 1.6 fps CPU-only, **2.5 fps with the GPU**, 2.1 fps for a UI-like card,
  3.6 fps at 720p; PNG capture 0.7 fps. Peak container memory ~0.95 GB. Encoding is not the
  bottleneck (libx264 veryfast 7 fps on 2.6 cores; `h264_v4l2m2m` 33 fps).
- **Pi headroom**: 1.8 GB RAM with ~1.1 GB available (Postgres, Couchverse and other apps
  share it); 4.5 TB free on the media disk; 14 GB free on the SD card.
- **Code facts**: no cron (the hourly cleanup re-enqueues itself); no time zone handling (the
  process runs in UTC); TMDB's base-language tagline is discarded; no intro/credits markers;
  the couch follower is a linear player with drift correction but no wall-clock offset; the
  web has no animation library (Svelte transitions + CSS keyframes).

### 3.1 Spike S3: analysis accuracy (2026-10-12, production library, read-only)

Chromaprint (ffmpeg's `chromaprint` muxer, raw, mono) of the first 8 and last 6 minutes of
episodes 2-4 of the first, middle and last season of every series; keyframe `signalstats` and
`ebur128` momentary loudness of the last 12 minutes of all 12 movies. Pairs matched by the
longest run of items within 10 differing bits (one-item dropouts tolerated, 0.1238 s per item).

| Series | Intro | Outro (before the end) |
|---|---|---|
| Dexter | ~105 s, at 1:30-2:15 (S1 partly 34-108 s) | ~50-55 s at -50 to -60 s (S8 weaker) |
| South Park | 32-36 s, at 0:00 (later seasons) or after the cold open (S1) | ~29 s at -29 to -32 s |
| Věřte nevěřte | 35-40 s at 0:13-0:30 (S4: 11 s) | 6-13 s |
| Simpsonovi | S1: 29-54 s at 0:21; **S18 and S36: no intro in the files** | S1: 6-11 s; none later |
| Invincible | the 9-14 s title sting, at varying times (often past 8 min) | 62 s (S1), 90-200 s (S2, S4) |

Movies: picture-only credits detection (dark, desaturated keyframes with bright text, YAVG < 45,
SATAVG < 6, YMAX > 150) found the crawl in 7 of 12; it misses designed main-on-end sequences
(Harry Potter 3's parchment map credits never look dark) and is confused by post-credits
scenes (Iron Man 2 and 3). Credits music (the longest stretch of 10 s windows above -35 LUFS
with under 3.5 LU spread) was found in all 12, but on its own it also catches score under a
final scene.

Consequences for `analyze_markers` and `couchtv_analyze`:
- **Series**: intro = consensus of at least two of three pairs agreeing within 2 s, minimum
  8 s (the Invincible sting); search the first 12 minutes; a season with no consensus has **no
  intro marker** (expected, e.g. files whose opening was cut). Outro likewise from the tails.
  The series' promo bed comes from the season with the strongest intro match.
- **Movies**: `credits_at` combines the crawl detector (sampled at 2 fps at low resolution, plus
  a vertical-scroll correlation between frames, which also catches crawls over pictures) with
  the steady-music stretch, choosing the **later** candidate when they disagree, because
  starting the squeeze late only shortens the credits while starting early would cut the
  ending. A post-credits scene is not kept on the channel. No confident result: the squeeze
  covers the movie's last 10 s.
- The admin override (D25, markers editor) is the backstop for designed credits.
- Cost: the whole sample took ~8 minutes of nice'd work on the Pi; the full library (about 1,300
  episodes) is a few hours once, then incremental per new file.

---

## 4. The experience

### 4.1 Viewers
- **`/tv`**: the live channel, full-bleed. Live overlays (D14) are drawn by the client in the
  Couchverse design language, timed from the stream's program date so they match the
  picture, not the wall clock. Presence (avatars, count) and reactions sit in the player
  chrome. Controls: volume, audio and subtitle menus, fullscreen, reactions, Guide, and
  "Watch from the start" (signed in). No seek bar: a thin progress line shows the current
  program's position.
- **`/tv/guide`**: today and the next 7 days as a horizontal timeline grid (now-line, aired
  programs dimmed), program detail sheets (overview, episode, times), "Watch from the start"
  for what is on now or already aired, "Tune in" for now.
- **Home "On now" card**: what is on, progress, the next program, "Tune in".
- **Guests** open `/tv/join/<code>`, type a name, and land on `/tv` (live only).
- Cutscene language is English (D13); the UI around them follows the display language.

### 4.2 Breaks (between programs)
A break fills the gap between a program's credits and the next start (D4, D17):

```
program ... credits start
  -> credits squeeze (10 s: the program's last seconds shrink into a panel, "Up next" fills the rest)
  -> short ident (4-5 s)          | full ident (10-12 s) before 20:15
  -> promo 1 (20-30 s)            | body per title + a 4 s end card with the airing phrase
  -> [promo 2 / short promo]      | as the gap allows
  -> lineup card (10 s)           | top-of-hour breaks only
  -> up next (8 s)                | the next program, counting down into it
next program starts on the frame
```

Gaps are filled exactly (to the 40 ms frame) with a library of short stings and idents
(2, 3, 5 s). The minimum break (squeeze + short ident + up next) is ~23 s.

### 4.3 Admins (web)
- Couch TV settings: the flag, the guest link (copy, rotate, disable), the time zone, the
  weekly template editor (6.2).
- The **EPG editor**: the week as a grid; swap, pin, remove, insert, lock, regenerate a day;
  each row shows readiness (package prepared, break rendered).
- **Render health**: queue, backlog in minutes of video, ETA versus air time, failures, the
  clip cache size.
- Per title (in the title editor): "Promo audio" waveform picker, intro/credits markers,
  footage shots preview (re-pick), "Never on Couch TV".
- **Broadcast preview**: any template with any title, played live in the admin's browser
  (the templates are web code), before anything is rendered.

---

## 5. Programming

### 5.1 The weekly template (default, editable)

The agreed default for the current library (D30, Europe/Prague). **A** = anchor (starts
exactly), **F** = flow (follows the previous program).

| Time | Mon-Fri | Saturday | Sunday |
|---|---|---|---|
| 06:00 A | Morning toons: Simpsonovi + South Park reruns | Morning toons | Morning toons |
| 09:00 F | Series reruns (Věřte nevěřte, South Park) | Series reruns | Series reruns |
| 12:00 A | Matinee movie (rerun of a past prime-time movie) | Matinee movie | Family movie |
| F | Afternoon series block (reruns) | **Marathon** (a drama, 4 episodes, in order) | **Marathon** continues |
| 17:00 A | Věřte nevěřte strip (1 ep) | Věřte nevěřte | Věřte nevěřte |
| 18:00 A | **Simpsonovi strip** (2 eps, in order) | Simpsonovi | Simpsonovi |
| F | **South Park strip** (2 eps, in order) | South Park | South Park |
| 20:15 A | **Prime-time movie** | **Saturday premiere** (D24) or movie | **Saga Sunday**: the next film of a series (HP 1 to 8, Iron Man 1 to 3) |
| F | **Dexter strip** (1 ep; Fri: Invincible) | Late movie | Invincible strip |
| F | Late block: adult animation reruns | Late block | Late block |
| ~01:00 F | Overnight: tonight's movie again, then series reruns until 06:00 | Overnight | Overnight |

"Saga Sunday" needs franchise order: fetch TMDB `belongs_to_collection` with metadata (new
column), falling back to release order within a shared name prefix.

### 5.2 Generator rules
The generator is a **pure function** `Plan(template, library facts, history, locks, from,
to) -> []Airing`, table-tested without a database (mirrors `playback.Decide` and ranks'
rules):
- **Anchors** start exactly; **flow** items start at the previous item's end plus its break,
  rounded up to the next 5-minute mark; a flow block that would overrun its anchor is
  shortened (drop an episode) or swapped for something shorter.
- **Strips** keep a cursor per series (episode order across seasons); reruns pick from
  episodes that already aired on the channel (fall back to any episode) and avoid the last
  N days.
- **Marathons** continue a drama season in order across weekends.
- **Movies**: a movie does not repeat in prime time within `max(5, 0.8 x movie count)` days;
  matinee and overnight slots rerun recent prime-time movies.
- **Premieres** (D24): newest never-aired movie added in the last 28 days takes Saturday
  20:15; promo weight rises all week.
- **Readiness**: an item is airable when its package is prepared (6.6). The plan prefers
  ready items; anything scheduled 48 h+ ahead may be unready (its preparation is queued); at
  T-6 h an unready item is swapped for a ready one of the same slot type.
- **Durations** are exact: a program airs from 0 to its **credits start** (6.5), rounded to
  the package's segment boundary; the remainder of the credits plays inside the squeeze.
- Determinism: a seed per day, so regenerating an unchanged day is stable.

### 5.3 Horizon and locking
- The plan job keeps **7 days** generated; the next **24 h are frozen** (renders are already
  under way), except for explicit admin edits and readiness swaps.
- Admin edits are rows with `source='admin'` and/or `locked=true`; regeneration keeps them
  and replans around them.

### 5.4 Settled in Phase 1
Details the decisions above left open, as built (`internal/feature/couchtv`, FEATURES.md
"couchtv"):
- **The template is rules, not titles.** The default (5.1) names no title: slots carry a
  `fill` (strip, marathon, reruns, movie, matinee, premiere, saga, encore) and a genre `pick`
  (any, comedy, animation, drama, family); strips go to the series with the most episodes of
  their genre, each series once (pinned first, then genre strips, any-genre strips,
  marathons). On a library shaped like production's (section 3, with TMDB's genres; the
  week test in `plan_test.go`) that yields 5.1's lineup: Simpsonovi and South Park at 18:00,
  Věřte nevěřte at 17:00, Dexter Mon-Thu, Invincible Fri and Sun. Genres are matched by their
  English names and Czech labels, since TMDB names them in a title's base language. A slot
  may pin `titles`, which is how the owner fixes the lineup whatever the genres say. The
  weekend marathon finds no free drama there and shares Dexter's cursor, so the weekend
  catches up on the weekday strip.
- **A broadcast day** is a weekday's slots in template order, from its first anchor (06:00) to
  the next day's; times before the first anchor are the night after it (01:00 encore). The
  day's first slot must be an anchor. A flow slot with a `start` is a target: the reruns
  before it fill up to it (each rerun at least half over by then), anchors are hard limits.
- **Gaps**: a gap of `fillerMinMinutes` (5) or more before an anchor, a fixed row or the day's
  end gets `filler` reruns (or a movie) while one fits; what remains grows the break. With
  22-minute episodes on a 5-minute grid, breaks reach about 27 minutes in the worst case.
- **Overruns**: a block drops what does not fit before the next anchor; that episode airs the
  next day. A strip episode longer than its slot could ever hold is passed over, so a strip
  never stalls.
- **Strip cursors are derived, not stored** (the 6.2 question): the next episode follows the
  series' latest strip or marathon airing, read from `couchtv_airings` (every program records
  the `fill` that chose it), so replanning a day reads exactly what the days before it left
  and nothing has to be rewound. `couchtv_strips` became an admin's restart point
  (`next_episode_id` from `effective_from`), used until the strip airs again after it.
- **Movies**: a matinee reruns a movie that was in prime time within the rotation window, so
  it takes nothing from the rotation; the week's premiere candidate and the next saga film are
  held back from other slots. Saga order is the TMDB collection, else names sharing a prefix
  (two leading words of 8+ letters, or one name being the other's leading words), in release
  order; a finished franchise passes to the one shown longest ago. A slot whose premiere or
  saga cannot run airs the rotation with `fill: movie`, so promos never call it a premiere.
- **Readiness** before `couchtv_prepare` exists: the Playback v2 package (fMP4 copied source,
  every audio rendition, an H.264 rung for anything but 8-bit SDR H.264). Beyond 48 h it is
  ignored; within 48 h free choices prefer prepared media inside each rule's tier; within 6 h
  prepared media beats every tier, an unprepared strip episode or premiere gets a prepared
  stand-in (a rerun, or the rotation) and the job swaps planned rows the same way. Swaps run
  with every plan run for now; Phase 4 runs them as preparation finishes.
- **Determinism**: each day's choices draw from a seed of its date; regenerating an unchanged
  day gives the same rows, and the store writes by diff, so unchanged rows keep their ids
  (and later a break its composition).
- **The job**: `couchtv_plan` runs at startup, after a config save and daily at 04:00 local;
  each run queues the next daily run first. "Frozen" is per broadcast day: a day starting
  within 24 h keeps its plan once planned. The horizon reaches the broadcast day a week from
  now. `adminRegenerateDay` replans the day and every later one, so strips stay in order.
  The schedule is planned while `couchTvEnabled` is off.

---

## 6. Backend

### 6.1 Feature package and dependencies
- `internal/feature/couchtv`: schedule store, generator, plan/prepare/compose jobs, the live
  playlist handlers, guest link, the presence/reaction hub, admin operations.
- Imports: `catalog` (titles, `Localize`), `library` (`Prepare`, markers), `playback`
  (package layout and `rendition.json` reading through exported helpers), `artwork`,
  `analytics` (a `TvWatchRecorder` interface, like `CouchWatchRecorder`). **Ranks is never
  imported**: a `TvStatsRecorder` interface satisfied by `*ranks.Store` is wired in
  `internal/app` (mirrors `CouchStatsRecorder`). Nothing imports `couchtv`. The DAG in
  FEATURES.md gains one node.
- Settings key `couchtv` (typed, like `ranks.RankConfig`): `timeZone`, `template`, rotation
  knobs, `guestLink` (hash + enabled), break recipe weights. Flag `couchTvEnabled` with
  `flags.CouchTvOn` (D29).

### 6.2 Data model (new migration)
- `couchtv_airings`: `id, starts_at, ends_at, kind (program|break), title_id, episode_id,
  media_file_id, slot (template slot key), fill (the rule that chose the program: strip,
  marathon, reruns, movie, matinee, premiere, saga, encore, filler; it replaced the planned
  `block`), source (auto|admin), locked, state (planned|ready|aired|failed|replaced),
  credits_at_ms (offset where the squeeze begins), created_at`. Index on `starts_at`.
- `couchtv_strips`: `title_id, next_episode_id, effective_from, updated_at`: an admin's strip
  restart; the cursors themselves are derived from airings (5.4).
- `couchtv_titles`: `title_id, excluded, promo_audio_file_id, promo_audio_start_ms,
  promo_audio_duration_ms, promo_audio_source, shots jsonb (media file, start, duration,
  score), mood, analyzed_version`.
- `couchtv_clips`: `id, kind (ident_full|ident_short|sting|promo_body|promo_end|upnext|lineup|squeeze),
  key (hash of template version + inputs), state, duration_ms, path, size_bytes,
  rendered_at, last_used_at`. Unique on `key`.
- `couchtv_breaks`: `airing_id, recipe jsonb (clip ids and offsets), audio_path, state`.
- `library`: `media_markers (media_file_id, kind intro|credits|credits_music, start_s, end_s,
  source auto|admin, version)`. Owned by `library`, because they are media facts that VOD
  can use later (skip intro, an earlier "next episode" prompt).
- `analytics`: `tv_watch_time_daily (day, user_id null, title_id, seconds)` (null user for
  guests).
- `ranks`: new counters and achievements (6.9).
- `metadata`: keep the base-language **tagline** (`titles.tagline`), add `collection_tmdb_id`
  and `collection_name` (TMDB `belongs_to_collection`) for Saga Sunday. All of the above is
  migration `0024_couchtv` except the markers, analytics and ranks tables.

### 6.3 Jobs
No cron exists; every periodic job re-enqueues itself like `cleanup`.

| Job | Where | Does |
|---|---|---|
| `couchtv_plan` | app | At startup, daily at 04:00 local and on template/lock changes: extend to 7 days, replan unfrozen days, run readiness swaps (built in Phase 1, 5.4). |
| `couchtv_prepare` | app | Per airing: queue `library.Prepare` (the `package`; a 1080p H.264 rung for HEVC/AV1/10-bit/HDR sources); mark ready. |
| `analyze_markers` | app (`library`) | Per season: chromaprint the first 8 and last 6 minutes of every episode, pairwise-match within the season for intro and outro; per movie: loudness profile of the last 12 minutes for credits music and black/text-frame detection for credits start. |
| `couchtv_analyze` | app | Per title: pick the promo soundtrack excerpt (D25) and footage shots (`scdet` + motion energy, first 50% of the runtime, no faces-only close-ups at the end of acts), measure program loudness (ebur128). |
| `couchtv_compose` | app | Per break, 48 h ahead: build the recipe (6.5), request missing clips. |
| `couchtv_render` | **sidecar only** | Render one clip (6.7). |
| `couchtv_assemble` | **sidecar only** | Render the break's audio mix, concatenate its clips' video (stream copy), mux, segment, write the break's `rendition.json`. |

The sidecar registers only its two job types; the app never registers them. A render is
claimed only while no `transcode_hls` job runs (a claim condition), and each render job is
one clip (at most ~2 minutes of work), so a transcode never waits long.

### 6.4 Admin operations (typed, `admin` prefix)
`adminGetCouchTv`/`adminUpdateCouchTv` (settings, template), `adminListAirings` (range),
`adminUpdateAiring` (pin, lock, swap, remove), `adminInsertAiring`, `adminRegenerateDay`,
`adminRotateCouchTvLink`, `adminCouchTvRenderHealth`, `adminGetCouchTvTitle` /
`adminUpdateCouchTvTitle` (excluded, promo audio, shots), `adminGetMediaMarkers` /
`adminUpdateMediaMarkers` (in `library`), `adminCouchTvWaveform` (peaks for the picker).
Every operation gets a `TestAPIConformance` case. Built in Phase 1: `GET/PUT /admin/couchtv`,
`GET /admin/couchtv/airings?from&to` and `POST /admin/couchtv/days/{date}/regenerate`, not
gated by the flag.

### 6.5 Break composition
- Inputs: the gap (next start minus this program's `credits_at`), the slot (top of hour?
  before 20:15?), promo candidates weighted by airing soon, premiere, strip, and not shown
  in the last breaks.
- Output: an ordered recipe of clips summing exactly to the gap at 25 fps, plus the per-break
  audio mix description (beds, SFX cues, crossfades, ducking, target loudness).
- **Loudness**: programs are measured once (`couchtv_analyze`); a break is mixed at the
  loudness of the program around it (clamped to -23..-16 LUFS), so breaks never blast.

### 6.6 The live stream
- **Channel grant**: `grant.Grant{Scope: Channel, Subject: user or guest, Resource:
  "couchtv"}` (6 h, re-issued on the socket). Routes on a new `Routes.Channel` group under
  `/tv/{grant}/...`; a segment request is only served if its file belongs to an airing in the
  current window (+-10 min), so a guest's grant never reaches the library.
- **Playlists** are generated per request from the schedule by `hls.LiveSchedule` (S1), so a
  restart only causes a reconnect. Media sequence numbers count segments per rendition, so
  they are chained airing to airing (`LiveOrigin.After`): each airing stores its **origin**
  (media sequence per rendition, discontinuity sequence) once it goes on air, computed from
  the airing before it; aired airings never change.
  - Multivariant: one video variant (v1, no ABR) whose `CODECS` (`hls.CoverCodecs`),
    `RESOLUTION`, `FRAME-RATE` and bandwidths cover every item, an audio group (`cs`, `en`,
    `CHANNELS="2"`; each airing contributes its best match per language, falling back to the
    original; breaks carry their mix in both), a subtitle group (`cs`, `en`; empty WebVTT
    segments where missing). The default audio follows `?lang=`.
  - Media playlists: a sliding window (2 min), `EXT-X-PROGRAM-DATE-TIME` on the first segment
    and every discontinuity, `EXT-X-DISCONTINUITY` at every program/break boundary,
    `EXT-X-MAP` repeated after each, a segment listed as soon as it goes on air, and a
    **constant** target duration (RFC 8216 forbids changing it): 10 s, with programs whose
    package has a longer segment (`LiveItem.MaxSegment`) airing from the 1080p rung.
  - Programs reuse their Playback v2 packages (copied source or the 1080p rung, the AAC
    audio renditions, the segmented WebVTT) and air to a video segment boundary; the audio
    segment that runs past it is served **trimmed** (`mp4.Trim`, `LiveSchedule.Trim`),
    because AVPlayer plays whole segments. Breaks are their assembled renditions.
- **Sync**: hls.js's own live edge moves in whole segments, so viewers tuning in at different
  moments end up a segment apart (4.6-5.4 s in S1). The client holds the **program date** at
  server time minus the target latency (3 target durations): it plays up to 10 % faster or
  slower for a drift under 2 s and seeks beyond. The server clock reaches the client in the
  socket's `hello` (`serverTime`, ms). The Pi keeps NTP time.
- **Fallbacks**: an unrendered break uses an **evergreen** recipe (cached ident, promo bodies
  with a generic "Coming up on Couch TV" end card, a generic up next). A missing program is
  swapped (5.2); worst case a pre-rendered "Technical difficulties" card loops to the next
  airing.

### 6.7 Rendering (the sidecar)
- **Image**: a `render` target in the Dockerfile (the couchverse binary + Debian `chromium`,
  `ffmpeg`, Mesa), compose service `render` with `/dev/dri` and `/dev/video11`, the shared
  `/data` volume, `mem_limit: 1g`, `cpus: 2`, low nice. Command: `couchverse render`.
- **Templates**: the broadcast bundle (6.8) embedded in the binary, served on localhost to a
  long-lived headless Chromium (GPU flags from the benchmark), driven over CDP with
  `chromedp`.
- **Frame loop**: for each frame t (25 fps): seek every animation to t, wait one frame,
  capture JPEG (q 92), pipe to `ffmpeg -f image2pipe ... libx264 -crf 18 -g 50` (2 s GOP so
  clips concatenate cleanly). **Static holds are skipped**: when no animation is active
  between two frames the previous frame is reused (no capture).
- **Footage**: the shots chosen in `couchtv_analyze` are extracted once per title as 25 fps
  JPEG sequences (1080p, SDR, tone-mapped if needed) that the template shows frame by frame.
- **Squeeze**: the template renders the panel with a keyed hole and exports the hole's
  rectangle per frame; ffmpeg composites the program's last seconds (from `credits_at`) into
  it.
- **Audio**: the break mix is rendered in the same page by an `OfflineAudioContext` (the
  samples' engine), written as WAV, encoded AAC 192k.
- **Caching**: a clip's key hashes the template version, its data and its inputs' versions;
  a template change re-renders lazily (evergreen first).
- **Budget** (estimate for ~36 programs a day): unique video per day ~15 min (squeezes, up
  nexts, lineups, end cards) at ~2.5 fps is **~2-2.5 h of rendering a day**, minus static
  holds; promo bodies are one-off (~20 s per title, ~1.5 h for today's library). Renders run
  ~48 h ahead.

### 6.8 The broadcast package
- `clients/broadcast/`: a small TypeScript bundle (Vite library build, no framework), using
  the generated design tokens and Plus Jakarta Sans. It is the productionized samples:
  `core` (timeline, curves and springs from the tokens, accent palette from the core's
  derivation), `audio` (engine, synthesized SFX kit, the theme and sonic logo, beds,
  ducking, voice bus reserved), `templates` (one module per template, data in, timeline
  out).
- **Determinism rule**: every frame and every sample is a pure function of t and the data
  (seeded PRNG, no wall clock, no accumulated `requestAnimationFrame` state); a CI test
  renders golden frames and an audio hash.
- The same bundle powers the admin **Broadcast preview** (live in the browser) and the
  sidecar's renders, so what the admin approves is what airs.
- Templates (from the samples, D27): `ident` (full and short), `sting`, `upnext`, `promo`
  (moods `cinematic`, `grit`, `bright`; premiere variant; body plus end card), `strip`,
  `shortPromo`, `lineup`, `squeeze`, `standby` ("Technical difficulties"), `evergreenEnd`.

**Settled while building it (Phase 2)**; the full reference is FEATURES.md "Broadcast package".
- **Tokens**: `cargo xtask codegen` writes `clients/broadcast/src/generated/tokens.ts` (colours,
  the accent derivation, radii, type roles, springs, curves), the same way every client gets
  its tokens; the package depends on nothing else in the repo. The palette keeps the samples'
  one difference from the core's derivation: the ink walks HSL lightness to the same 4.5:1
  target, so a navy accent becomes a bright blue on air instead of grey.
- **Consumers**: the web compiles the package from source (`@couchverse/broadcast`, also copied
  into the Docker web stage); the sidecar serves `dist/render.html` (a Vite page build with
  hashed script, styles, fonts and logo), which exposes `window.couchtvRender`.
- **Spec and schema**: `clients/broadcast/src/spec.ts` is the contract;
  `contract/broadcast-spec.schema.json` is generated from it (`npm run schema`, drift fails CI),
  for the composer (6.5) to validate what it writes. Airings arrive as English phrases
  (`{eyebrow, day, time}`); beds as `{url, in, out}` in WAV, FLAC or Ogg Opus (open-source
  Chromium has no AAC); footage as 25 fps JPEG frame URLs per shot.
- **Durations**: full ident 11.52 s and the expand squeeze 11.52 s (whole frames); short ident
  5 s; stings any whole number of frames from 2 to 6 s (the hold absorbs it), so a gap can be
  closed by one sting; up next 8 s by default (6 to 12); a promo body ends on a frame-aligned
  cut snapped to the bed (cinematic about 17 s, premiere about 19 s, grit about 13 s, bright
  about 13.5 s) and its end card is 4 s; the body is airing-independent by construction (a test
  proves it), and the break audio renders the promo `full` so its bed runs across the cut.
- **Audio**: 48 kHz; a clip's or a break's render is normalized to its `loudness` (BS.1770
  integrated, default -18 LUFS) with the true peak capped at -1 dBTP (quieter rather than
  clipped), delivered as a float WAV; a break spec lists clips and their starts, each ringing
  out under the next (`crossfade`, 0.3 s). Two renders differ only by float noise (< 1e-5).
- **Squeeze**: the program panel's motion is data that drives both the animation and
  `keyRect(t)`. The simplest path is `program.frames` (the renderer extracts the program's last
  seconds as JPEG frames and the page draws them; no compositing). For an ffmpeg composite the
  panel is left transparent (`program.kind = key`) and a `matte` layer carries the alpha; a
  colour key was rejected (JPEG and 4:2:0 fringe it).
- **Static holds**: `isStatic` is exact but today's templates keep subtle motion in every frame
  (a breathing LIVE dot, slow pushes, grain), so the 6.7 budget should not count on skipped
  frames; freezing those motions on long holds is a later optimization.
- **Brand isolation**: the owner asked for a new ident and theme (typographic, a new lockup and
  possibly a display face); the lockup, corner bug, channel tag, sonic logo and theme, ident
  and stings live in `src/brand` so the remake replaces that folder only. The current ident is
  a placeholder.

### 6.9 Presence, reactions, progress, analytics, ranks
- **Hub**: an in-memory channel room (like couch's Hub, much simpler: no host). WebSocket
  `GET /tv/socket` (cookie or bearer for users, the channel cookie for guests). Frames in
  `couchtv/protocol.go` (`hello`, `presence`, `reaction`, `schedule_changed`, `report`),
  published with `couchverse couchtv-schema`, golden fixtures in `contract/fixtures/couchtv`.
- **Reports** (every ~10 s): the program date the player is showing. The server maps it to
  (airing, offset): `watch_progress` for signed-in viewers after 2 minutes of a program,
  `tv_watch_time_daily`, and ranks counters through `TvStatsRecorder`.
- **Ranks**: TV minutes earn XP at a `RankConfig` rate; achievements (absent when the flag is
  off): time-based (Prime-time regular, Night owl, Morning cartoons), social (Full house,
  Crowd reactor), loyalty (Appointment TV, Marathoner, Premiere night). Pull-based like the
  rest of ranks.

---

## 7. Clients

### 7.1 Shared core
- A `couchtv` module: the schedule window (now, next, guide days), presence and reactions
  over the existing `Socket` effect, guest join, reports, and the live player. Views:
  `CouchTvView` (now/next with times, progress, presence, reactions, overlay cues),
  `GuideView`, `OnNowView`. Times are computed from the player's reported **program date**,
  not the wall clock, so overlays match the picture.
- Player: `PlayerLoad` gains a live source (the channel multivariant) and `PlayerReport`
  gains `program_date_ms`. A wire-format change: `make contract`, every runtime handles it,
  Android's `WireFormatTest` covers the new variants even before Android uses them.
- Scenario tests: tune in mid-program, a program boundary (overlays switch on the picture's
  clock), guest join, reactions, schedule change, socket loss and reconnect.

### 7.2 Web
- `features/couchtv`: `TvPage`, `GuidePage`, `TvJoinPage`, the admin pages (settings,
  template, EPG, render health, preview), components (`LiveOverlay`: bug, Now, Next;
  `PresenceBar`; `OnNowCard`; the reaction layer reused from couch). Routes stay thin
  shells.
- `VideoPlayer` in a `live` mode (no seek, a program progress line); `ElementPlayer` with
  hls.js live settings and `playingDate` reports. HLS through hls.js everywhere (Safari's
  native player keeps working as a fallback).
- TV-mode rules apply (arrow-reachable controls, `data-tv-autofocus` on `/tv`'s main control).
- i18n keys for every UI string (en/cs); problem codes for new errors; an e2e spec for `/tv`
  (signed in and guest) on a seeded schedule with fixture clips.

### 7.3 Native (later)
The live stream plays in AVPlayer and ExoPlayer as is; the core module is shared. What remains
per platform: the screens (TV-first layouts), the live overlays, presence and reactions.

---

## 8. Quality
- Generator: table tests (anchors, flow rounding, strips, marathons, premieres, readiness
  swaps, locks, exact durations).
- Composer: recipes sum exactly to the gap; loudness targets.
- Playlists: stitcher unit tests and `validate-hls` on snapshots at boundaries; an hls.js
  playback test across program/break/program in CI; AVFoundation in the macOS HLS job.
- Broadcast: golden frames per template (pixel diff with tolerance) and an audio hash, in
  Chromium in CI.
- API conformance for every operation; protocol fixtures; core scenarios; Playwright e2e.
- `make lint test`, `make check`, `make core-test`, `make contract`, `make hls-check`,
  `make e2e` stay green; FEATURES.md and CLAUDE.md updated as each phase lands.

---

## 9. Roadmap

### Phase 0: Spikes
- **S1 Live stitching**: a hand-written live playlist over two real packages and a generated
  clip, with audio and subtitle groups and discontinuities; play in hls.js (Chrome), Safari
  and the Titan OS TV; `validate-hls`.
- **S2 Render pipeline on the Pi**: the sidecar image, GPU flags, static-hold skip, libx264
  pipe at 25 fps, a 60 s break assembled with its audio mix, memory under the cap.
- **S3 Analysis accuracy**: intro/outro for every series, credits start and credits music for
  every movie, footage picks for 3 movies; a report for the owner.
- **Exit**: all three demonstrated on the Pi; numbers recorded here.

### Spike results: S1 (live stitching), 2026-10-12

**What ran** (`make live-spike`: backend `TestLiveSpike`, `clients/web/e2e/live.mjs`,
`scripts/avplayer-probe.swift --live`, on an M1 Pro). Packages were made by the real probe
and transcode jobs: *Glass Harbor* from `make sample-media` (copied H.264 720p24 Constrained
Baseline, AC-3 5.1 as AAC stereo, English and Czech subtitles), *Static Bloom* from `make
sample-media` (HEVC Main 10 HDR10 as the 720p H.264 rung, English and Czech audio, English
subtitles), a generated *Night Shift* (copied H.264 High 1080p30, a keyframe every 10.5 s, one
untagged **mono** AAC track) and two stand-in breaks (1080p25 H.264 High, CRF 18, 2 s GOP,
AAC stereo 192k; 17.36 s and 12.48 s, packaged like a program). The loop *Glass Harbor,
break, Static Bloom, break, Night Shift (cut at 31.5 s, a video boundary no audio boundary
matches), break* aired through `hls.LiveSchedule` (2 min window, target duration 11, one
covering variant, `cs`/`en` audio and subtitles, 6 s WebVTT segments, empty ones where an
item has none). Players tuned in mid-item and played 130 s (3 to 5 boundaries each): five
hls.js 1.6.16 viewers in Playwright Chromium (two pairs 20 s apart: one on hls.js's own live
edge, one holding the program date) and one in Playwright WebKit, plus AVFoundation, all at
once; each viewer switched audio language and turned subtitles on midway. `validate-hls` ran
1.5 s before and after four boundaries. The pages sample the program date, the picture
(breaks are plain blue) and the dominant tone (each item and language has its own) four times
a second and check them against the schedule.

| Player | Stalls | Item change vs program date (picture / sound) | Subtitles | Notes |
|---|---|---|---|---|
| hls.js, Chromium | 0 | 0 of 1600 samples off; boundary seen -0.02..0.22 s / 0.11..0.33 s after it (once 1.24 s, while the hold seeked across it) | every cue sample on time (20-24 per viewer), none out of place | audio switch and subtitles fine; mono to stereo fine |
| hls.js, WebKit | one of 0.04 s | 0 of 426 off; 0.15..0.23 s | 22/22 | Web Audio reads MSE audio as silence, so sound was not measured |
| AVFoundation (macOS) | 0 | program date never jumps (after the fix below) | 2 audio + 2 subtitle options, selection works | an audio switch pauses ~0.35 s |
| `validate-hls` | - | 0 errors on 8 snapshots per run | - | warnings: target duration 11 > 10, `AVERAGE-BANDWIDTH` |

Boundary offsets include the sampling step (0.25 s) and, for sound, the 170 ms FFT window.
Codec, profile, resolution, frame-rate and channel changes across discontinuities
(Constrained Baseline 720p24, High 1080p25, High 1080p30, mono and stereo AAC) caused no
error, stall or blank frame in any player.

**Sync.** Two viewers 20 s apart on hls.js's own live edge differ by **4.6-5.4 s** (Chromium
and WebKit started together: 6.3 s): `liveSyncPosition` counts back from the playlist's last
segment, which moves in whole segments. Holding the program date at server time minus the
target latency brings them to **0.03-0.24 s median, 1.0-1.4 s worst** (worst while the
seeking hold caught up). Under the full load (seven players on eight cores) Chromium played
about 1 % slower than real time, drifting 1.2-1.7 s in two minutes; alone it kept time (0.99-1.00,
run 3). A slow device falls behind on its own edge, which is what the hold corrects.

**What broke and how it was fixed**
- **AVPlayer plays whole segments.** A program leaves the air on a video segment boundary;
  its audio segments are cut elsewhere (~6.0 s apart), so the last one runs past the item. With
  only its `EXTINF` shortened, hls.js overwrites the overrun with the next item (no audible
  tail), but AVPlayer played all of it: Night Shift ran 3.6 s long and the program date then
  jumped back at the break. Fix: `LiveSchedule.Trim` lists that segment under a URI the server
  answers with `mp4.Trim` (the AAC samples up to the cut, within half a frame); afterwards
  AVPlayer kept the date and `validate-hls` lost its `fmp4.duration` warnings.
- **The validator only knew VOD.** It now reads `EXT-X-DISCONTINUITY-SEQUENCE` and
  `EXT-X-PROGRAM-DATE-TIME`, checks each item's timeline from its own first segment, lines up
  every rendition with the video by program date and discontinuity number (`live.alignment`),
  requires three target durations in a live window (`live.window`), warns about an item
  without a date (`live.program-date-time`) or a date gap at a discontinuity (`live.gap`),
  errors on a date jump without one, maps live cues through their own item, and accepts a
  variant whose codec string, resolution and `CHANNELS` cover every item's.
- **Mono audio.** A mono AAC track is copied into the package as is; under `CHANNELS="2"`
  every player handled it (the validator now treats `CHANNELS` as a maximum for live).
- **Numbering cannot be a function of time.** Media sequence numbers must be consecutive per
  rendition, and an item's segment count differs per rendition (copied video cut at
  keyframes, audio every ~6 s), so numbers are chained item to item from a stored origin
  (`LiveOrigin.After`); the target duration is computed over all renditions so every playlist
  of a request keeps the same window.

**Settings to use**
- Items: programs as packaged (6 s segments, longer for sparse keyframes), breaks segmented
  at 6 s from their 2 s GOP; every item fitted to its time on air; the overrunning audio
  segment trimmed.
- `EXT-X-TARGETDURATION`: a channel constant (RFC 8216 forbids changing it), default 10;
  11 played everywhere, `validate-hls` only warns above 10. ffmpeg cuts a copied source at the
  first keyframe after each 6 s mark, so a segment can reach 6 s plus the longest GOP:
  measure `LiveItem.MaxSegment` over the Pi's packages before fixing the constant, and air a
  program whose package exceeds it from the 1080p rung.
- Window 2 min; segments listed as soon as they go on air; `EXT-X-PROGRAM-DATE-TIME` (UTC, ms)
  on the first segment and every discontinuity; a discontinuity at every item and
  `EXT-X-MAP` repeated after it; `EXT-X-DISCONTINUITY-SEQUENCE` once one has scrolled out.
- Multivariant: `hls.LiveMultivariant` with `CODECS` from `hls.CoverCodecs`
  (`avc1.640028,mp4a.40.2` here), the largest `RESOLUTION` and `FRAME-RATE`, bandwidths
  from the largest peaks; audio `CHANNELS="2"`, default from `?lang=`; subtitles
  `DEFAULT=NO,AUTOSELECT=YES`.
- Subtitles: the sidecar cues cut along a 6 s grid with `X-TIMESTAMP-MAP=MPEGTS:126000`
  (the packages' 1.4 s offset); an empty WebVTT file (header and the same map) for every grid
  step of an item without them. hls.js placed every cue right, also for viewers who joined
  mid-item.
- hls.js: `liveSyncDuration` = 3 target durations, `liveMaxLatencyDuration` = that plus 2;
  then the program-date hold: up to 10 % faster or slower below a 2 s drift, a seek beyond (the
  watch page and `e2e/live.mjs` use it; the measured runs above seeked at 0.5 s, which caught
  up in visible steps under load).

**Plan changes** (applied to 6.6): numbering origins stored per airing (`couchtv_airings`
gains an `origin` column, written once the airing is on air); a constant target duration with
the rung fallback in `couchtv_prepare`; the trimmed audio tail served by the channel route
(its window check must accept the trimmed URI); sync by program date, which needs the server
clock in the socket's `hello` (`serverTime`) and a program-date hold in `ElementPlayer`'s live
mode (and later in the native players). Separate-language files (model B) cannot feed a live
audio rendition unless they are cut to the played file's timeline, so v1 uses embedded tracks
and falls back to the original.

**Still to check on the Titan OS TV** (owner): `COUCHVERSE_SPIKE_ADDR=0.0.0.0:8099
COUCHVERSE_SPIKE_HOLD=30m make live-spike`, then open `http://<mac>:8099/watch.html` on the
TV and a laptop side by side: three boundaries without a stall or a blank frame (including
720p24 to 1080p25/30), left/right switches audio, up/down subtitles, cues on time, the
on-screen program date within 2 s of the laptop's, and no memory growth over the 30 minutes.

### Phase 1: Foundations
Flag, settings (time zone, template), migrations, the pure generator + `couchtv_plan`, the
markers table + `analyze_markers`, tagline and collection metadata, the admin EPG
(read-only). **Exit**: a week of plausible schedule visible in the admin, tests green.

### Phase 2: Broadcast package
`clients/broadcast` from the samples (ident, theme, every template, footage support), the
admin Broadcast preview, golden frames. **Exit**: the owner approves every template in the
preview.

### Phase 3: Rendering
The render sidecar, the clip cache, `couchtv_analyze`, `couchtv_compose`,
`couchtv_assemble`, render health. **Exit**: 48 h of breaks rendered on the Pi within budget.

### Phase 4: The live stream
Channel grants, playlist builder (video, audio, subtitles), `couchtv_prepare`, fallbacks,
guest link. **Exit**: a 24 h soak locally and on the Pi with no gaps; validator clean.

### Phase 5: Web viewer
The core module, `/tv`, overlays, Guide, home card, presence and reactions, guest join.
**Exit**: e2e green; the owner watches an evening on the TV.

### Phase 6: Integration and admin tools
Progress, analytics, ranks XP and achievements; the EPG editor; per-title tools (promo audio
picker, markers, shots, "Never on Couch TV"). **Exit**: conformance + scenarios green.

### Phase 7: Production
Deploy (app + sidecar), the 7-day soak with monitoring, tuning, docs. **Exit**: success
criteria (section 2) met; the flag on.

### Later
Apple and Android screens; an announcer voice (local TTS); skip intro and credits in VOD
(markers exist); ABR on the channel; more channels.

---

## 10. Risks
- **Render throughput** (2.5 fps): reuse, static-hold skipping, a 48 h lead, evergreen
  fallbacks; an optional render helper on a faster machine later.
- **Pi memory** (~0.95 GB peak for Chromium next to Postgres): a hard cap on the sidecar,
  one render at a time, never alongside a transcode.
- **HLS edge cases**: codec/resolution changes across discontinuities, long copied-source
  segments, subtitle timing across boundaries; S1 covers hls.js, Safari and the TV.
- **Detection misses** (credits starting over a scene, themes that change per season): admin
  overrides, and the squeeze is harmless when credits start a little late.
- **Package storage**: every aired program keeps its package (also useful for VOD); 4.5 TB
  free today; render-health shows the cache size.
- **Clock**: the server is the only clock; the Pi runs NTP.

---

## 11. Open questions
- Sign-off on the clean ident and the modern theme (D27), then on the remaining sample pieces;
  final approval happens per template in the Phase 2 Broadcast preview.
