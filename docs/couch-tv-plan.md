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
| D14 | **Live client overlays** over programs: corner bug with LIVE and the clock, a "Now" caption and a "Next" caption. The bug stays up through the whole program; the captions show with the player's controls (first built timed: Now at program start, Next near the end, the bug with the controls; the owner asked for the bug always on and the captions on hover). The **credits squeeze** is pre-rendered into the break, composited with the program's last seconds. |
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
| D25 | **Promo soundtrack: automatic + admin override.** Series: the theme found by fingerprinting episodes. Movies: the end-credits music (the steadiest loud stretch near the end). The title editor gets a "Promo audio" picker with a waveform. Since 2026-10-13 the title's own soundtrack from the web (official previews from iTunes and Deezer) comes first and the excerpt is the fallback (6.3, "Promo audio from the web"). |
| D26 | **No announcer voice in v1**; the audio mix reserves a voice bus so a local TTS voice can be added later. |
| D27 | *(Superseded by D33 on 2026-10-13: C()UCH TV looked too much like Prima COOL.)* **Ident: a full cut (10-12 s) and a short cut (4-5 s)** from one design and one score. After two rejected rounds the owner asked for an ident "very inspired by" the Prima COOL idents (typographic, brackets that open as frames, pure colour, typed and edited text) with a more epic theme. Chosen (2026-10-12): the **C()UCH TV** brand (the O of COUCH splits into brackets that are also the couch's armrests; Saira; red, black and white), **ident variant B "frame"** (the brackets burst out to frame the screen and cut on the beat, then slam back into the O) and **theme 1 "hybrid epic"** (a D minor build lifting to D major on the hit, the A4 D5 A5 G5 F#5 hook on brass). Reference: `tools/couch-tv-samples`. |
| D28 | **Time zone**: a channel setting, default `Europe/Prague`; schedule rows are stored in UTC. |
| D29 | **Gated** by an admin flag `couchTvEnabled` (mirrors `couchEnabled`), **off by default** until the channel has a rendered schedule. |
| D30 | **The default weekly template is 5.1**, including **Saga Sunday** (the next film of a franchise every Sunday at 20:15, in order). |
| D31 | **Branch**: a new `couch-tv` branch from `main`; `native-clients` is retired once its last docs commit is on `main`, so the remote keeps two branches (`main`, `couch-tv`). |
| D32 | **The planning sample reel** (HTML/CSS/JS + README) is kept in `tools/couch-tv-samples/` as the reference for Phase 2; its media (soundtrack excerpts, artwork) is gitignored and fetched from the Pi read-only by a script. |
| D33 | **The identity is the Couchverse logo** (2026-10-13, supersedes D27): the app's own logo (a couch with a popcorn bucket on it) and the wordmark `couchtv` in Plus Jakarta Sans ExtraBold at -0.035 em, lowercase, `couch` in white and `tv` in the red #e50914 (as the app pairs `couchverse`); Plus Jakarta Sans only; red, near-black (#050505) and white with tonal depth; no taglines, ever. Four approved full idents (journey, cinematic, morph, assemble) all end the same way: the couch alone on black, a popcorn bucket drops onto it (gravity, a small rebound, the frame giving a little), the two become the logo, then `couch`, `t`, `v` type in. A stacked lockup (as the idents end) and a row lockup for small sizes (the corner bug, the promo tag, the web's overlays), drawn as paths. Epic in vision, clean and modern in execution: no brackets, flares, particles, chrome, squash-and-settle or cartoon sofas. Reference: `clients/broadcast/concepts` (local, untracked). |
| D34 | **One sonic logo, A4 E5 D5** (the fifth, ninth and root of D major; morph's and assemble's, 2026-10-13) on `couch`, `t`, `v`: the beat after the landing, then half a beat apart at 75 bpm, the last note held while D add9 settles, on a felt piano, a glass halo and a bowed swell; every screen is scored with one generated house palette (warm detuned pads, felt piano, glass, bowed swells, sub, soft impacts, air, a long generated hall). The **short ident** (5.2 s; 4.52 s until 2026-10-14) is the shared popcorn ending alone, opening on the couch alone in its lockup place; **stings** (5-10 s; 2-6 s until 2026-10-14) are one design, the popcorn drop and the whole logo with its chord ringing out, the ending placed by their length; the **promo tag** is the row lockup landing on the compact sonic logo; the **credits squeeze hands over into the short ident** (handover `ident`: the program shrinks onto the couch and dissolves into it, its last frame the ident's first). |
| D35 | **All four full idents air, in turn** (2026-10-13). Journey, cinematic, morph and assemble, the four the owner approved, are the `ident` template's full cut (`design`), ported unchanged but for the sonic logo, A4 E5 D5 in all four (journey's and cinematic's motifs re-scored wherever they state it). The composer gives a break's first full ident the broadcast day's design (each day the next, so all four air within four days), the full idents a long gap repeats in the same break the designs after it, and the evergreen break always journey, so it is composed once. They render on the Pi, once per template version, at night: estimated 4 to 16 minutes each but cinematic, 65 to 116 (6.7); a render collects its canvas reads as it goes, which keeps it inside the sidecar's 1 GB. If cinematic's first render on the Pi runs past ~90 minutes, it ships pre-rendered. |
| D36 | **Bulletins bind late, in slots of the breaks.** A break that carries a bulletin keeps a fixed-length slot for it after the promos and the lineup, before up next, holding a fallback of exactly that length (promos and stings, renderable ahead like any part), so the break is composed, rendered and assembled complete on its own. It is assembled in sections around its slots (a package each), its sound mixed per section so nothing rings across a slot's edge and each slot normalized on its own. When the live stream puts the break on air, a slot that airs whole takes the newest rendered bulletin still valid when the slot ends, at the slot's loudness, else its fallback; pinned, it never changes (6.10). |
| D37 | **When the bulletins air** (defaults the owner tunes later): the news (54.4 s) in the breaks that end on the hour every two hours from 06:00 to midnight (06:00, 08:00 ... 24:00; the owner chose every two hours over every hour, 2026-10-13), the weather (38.4 s) in the first break ending within the hour after 07:00, 12:00 and 17:00 and in the break before the evening movie, never where a bulletin fetched on the grid 15 minutes before the break goes on air would have expired by its end; a break keeps a slot only beside the squeeze, the short ident, a short promo and the shortest up next. The lengths are the owner's "news and weather should be longer" (2026-10-14: the 20 s and 15 s placeholders were too short to read): each is a whole number of beats at the house's 75 bpm (68 and 48), so every scene change lands on the music. |
| D38 | **A bulletin renders once, when it is the one that airs**: the bulletins job asks for a clip on its kind's last run in time for the next slot (one render per slot, not per fetch), the sidecar renders it as `couchtv_bulletin_render` ahead of every other clip, never waiting for a transcode or the quiet hours, and only while the channel is on. The clip carries its sound at every whole LU a break is mixed at (-23 to -16) and a slot's sound is mixed at its break's loudness in whole LU, so a bulletin sounds as loud as the fallback it replaces. A bulletin's clip goes an hour after its bulletin stops being valid. |
| D39 | **D13 extended to the bulletins** (owner, 2026-10-13): English chrome (titles, labels, weekdays, times, the credits "Weather data by Open-Meteo.com", "Source: ČHMÚ", "Borders: ČÚZK / geoBoundaries (CC BY 4.0)", "Headlines from the outlets’ RSS feeds, as of 17:40"), ČHMÚ's English event and kraj names, Prague by its English name; the headlines stay Czech, whole and unaltered, each labelled with its outlet's name, since the outlets' terms forbid altering or shortening them and a translation would alter them. |
| D40 | **Czechia's outline lives in the broadcast package**, a module generated from the server's `czechia.geojson` (`npm run outline`), not in every spec: it never changes from one bulletin to the next and a template's version covers a change; the spec carries the cities' positions and the map's credit. A forecast airs until midnight at the latest, since its days are named from the day it was fetched. |
| D41 | **Screens show the title's own logo and real scenes** (owner, 2026-10-13: film logos and scenes, not only plain images; quality is the main focus). Every title in a spec carries its logo in the language of its name (else one not tied to a language) and, on every screen that shows it in motion (up next, the squeeze's next card, the lineup, the strip, short and full promos and their end cards), its analysed footage shots, the frame URLs the promo body uses, so each shot is extracted once for all of them; the squeeze's ending is the program itself. The production library was linked to TMDB before logos existed, so a backfill gives every linked title the logos it lacks (its content languages and English) once per backfill version, never replacing one. What a Pi draws on every frame stays bounded: logos within 1600 x 800 (stored within 2560 x 1280; TMDB's run to 12000 px wide), and the screens that render for every break take the backdrop 1920 wide, while the promo body, rendered once per title, keeps the original its stills crop into. |
| D42 | **Sharp and flat** (owner, 2026-10-14): "ALL of the screens should be re-done in new CouchTV branding... It should be in a sharp (meaning no border-radius, modern and flat), with nice animations." No border radius anywhere (panels, cards, chips, the squeeze's program panel with its key and matte, the corner bug's marker and rule, the web's overlay): only the logo's own curves and the type stay round. Flat: no shadows, glows or bevels; night corner shades (radial gradients) keep type legible over a picture. Applies to every screen redone in the new brand (D33). |
| D43 | **The long promo is a trailer cut from the title itself** (owner, 2026-10-14: "promos can be LONGER (can be like a minute, minute and a half... and can include some part of the movie - or its trailer? - with sounds and stuff)"). Official trailers are not in the library and are never downloaded (their hosts' terms), so the channel cuts its own: 6 to 9 scenes of 3 to 8 s from the first half of the title (5 to 50 % of a movie's runtime, of three episodes spread over a series), 40 to 60 s of them, each cut where its lines begin and end and with its own sound, taken from the original-language track (English where the media has it, else the default one), over the soundtrack bed, ending on the promo's 4 s end card: a minute to a minute and a half with its titles. The scenes are picked in `couchtv_analyze` from the subtitles of the dialogue track's language when they follow the sound (the picker measures an offset of a few seconds and shifts them), else from the speech the sound holds, rated as a trailer editor would (short, whole, punchy lines, questions, a back-and-forth, stakes, the title's own name; not exposition, small talk or crudeness) with a calm opener and a punchline to close on when there are some. The strongest candidates (premiere, movie, saga; a strip or marathon in a break of 8 minutes or more) air one when a break has room for it, one a break and a title's not again within four hours; it renders once per title in the quiet hours, three a day at most (6.5, 6.7). The admin can veto it: drop scenes, choose no trailer for a title, or hand the pick back to the analysis (6.4); the composer reads the scenes a title has, so a cut trailer is another clip and the old one airs no more. |

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
  shortened (drop an episode) or swapped for something shorter. Slack no filler takes before
  an anchor, a fixed row or the day's end is spread over the breaks since the last item that
  cannot move, on the 5-minute grid, so breaks stay around 5 minutes (5.4).
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
  end gets `filler` reruns (or a movie) while one fits. What remains, when it would leave a
  break over 5 minutes (`breakTarget`), is spread over the flow since the last program that
  cannot move (one at its anchor, a fixed row, the day's first program): every break grows
  from its shortest by whole 5-minute steps, the shortest first, so starts stay on the grid
  and no break ends up more than 5 minutes longer than the shortest. On a library shaped
  like production's the 12-22 minute breaks before 12:00, 17:00, 20:15 and 06:00 become
  breaks of about 3 and 8 minutes (a 22-minute episode leaves about 3 minutes to the next
  mark, one step more makes 8). A gap with nothing movable before it (a program at its
  anchor right before the next anchor) stays as it is. Flow only moves later, so a rerun
  chosen to be half over by a flow target may start a few minutes later than that. A
  replan after part of the flow aired (those rows are fixed) spreads the rest the same way.
- **Overruns**: a block drops what does not fit before the next anchor; that episode airs the
  next day. A strip episode longer than its slot could ever hold is passed over, so a strip
  never stalls.
- **Strip cursors are derived, not stored** (the 6.2 question): the next episode follows the
  series' latest strip or marathon airing, read from `couchtv_airings` (every program records
  the `fill` that chose it), so replanning a day reads exactly what the days before it left
  and nothing has to be rewound. `couchtv_strips` became an admin's restart point
  (`next_episode_id` from `effective_from`), used until the strip airs again after it.
- **Series that may play at random** (2026-10-13; `titles.allow_random_playback`, on for
  Simpsonovi, South Park, Futurama and Věřte nevěřte in production) keep their strips and
  marathons without a cursor: each pick draws an episode that fits from the day's seed, never
  one that airs that day already, those that never aired first, then the one aired longest
  ago (outside `rerunGapDays` first, an episode fixed within the gap after it last); reruns of
  it draw from all its episodes. Each episode's place in the draw is a hash of the day's seed
  and the episode, so a replan changes a pick only when its own candidates change. Readiness,
  removals and fixed rows work as for any strip; its strip promo drops the "in order" count;
  a change of the flag in the title editor queues a plan run, so the days not frozen yet
  follow it.
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
  score), mood, analyzed_version, trailer jsonb (the trailer's scenes: picture file, audio file
  and stream, start, length, lines with their text), trailer_source, trailer_version (D43)`.
- `couchtv_clips`: `id, kind (ident_full|ident_short|sting|promo_body|promo_end|trailer_body|trailer_end|upnext|lineup|squeeze),
  key (hash of template version + inputs), state, duration_ms, path, size_bytes,
  rendered_at, last_used_at`. Unique on `key`.
- `couchtv_breaks`: `airing_id, recipe jsonb (clip ids and offsets), audio_path, state`.
- `library`: `media_markers (media_file_id, kind intro|credits|credits_music, start_s, end_s,
  source auto|admin, version)`. Owned by `library`, because they are media facts that VOD
  can use later (skip intro, an earlier "next episode" prompt). Built as migration
  `0025_media_markers` with times in ms (6.3).
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
| `analyze_markers` | app (`library`) | Per season: chromaprint the first 12 and last 6 minutes of every episode, pairwise-match within the season for intro and outro; per movie: loudness profile of the last 12 minutes for credits music and black/text-frame detection for credits start (built in Phase 3, below). |
| `couchtv_analyze` | app | Per title: pick the promo soundtrack excerpt (D25) and footage shots (`scdet` + motion energy, first 50% of the runtime, no faces-only close-ups at the end of acts), measure program loudness (ebur128) (built in Phase 3, below; no face detection), and pick the scenes of the title's trailer (D43, below). |
| `couchtv_compose` | app | Per break, 24 h ahead (48 h until 2026-10-12): build the recipe (6.5), request missing clips. |
| `couchtv_render` | **sidecar only** | Render one clip (6.7). |
| `couchtv_assemble` | **sidecar only** | Render the break's audio mix, concatenate its clips' video (stream copy), mux, segment, write the break's `rendition.json`. |
| `couchtv_bulletins` | app | The weather (hourly) and the news headlines (every 20 minutes) from 05:30 to 00:30, and at startup (6.10). |

The sidecar registers only its two job types; the app never registers them. A render is
claimed only while no `transcode_hls` job runs (a claim condition), and each render job is
one clip (at most ~2 minutes of work), so a transcode never waits long.

**Settled while building the analysis (Phase 3)**; the full reference is FEATURES.md
"library" (markers) and "couchtv" (the analysis job and title operations).
- **Storage**: migration `0025_media_markers`. `media_markers` keys on (file, kind, source),
  so the analysis' row and the admin's coexist and the admin's wins; times are `start_ms` /
  `end_ms` (ms, like `credits_at_ms`), and an admin row without a start says "none" (a wrong
  detection suppressed). For a series episode `credits` is the end credits as a range (the
  outro); for a movie a start only. `media_files.markers_version` records which analyzer
  looked at a file, so files without markers are not analyzed again and `MarkersVersion` bumps
  re-run (`couchtv_titles.analyzed_version` does the same for `couchtv_analyze`).
  `couchtv_titles` gained `shots_source`, `loudness_lufs` and `analyzed_at`; shots are
  `[{mediaFileId, startMs, durationMs, score}]`.
- **Jobs**: one `analyze_markers` type with three payloads: a file (queued by the prober and
  the backfill), a season (`titleId` plus one season, queued by an episode's job two minutes
  on, so an upload batch is matched once) and none (the backfill at every start).
  Fingerprints are cached per file (`cache/fingerprints`, about 35 KB an episode), so a new
  episode only fingerprints itself. Each file is matched with its **three nearest other
  episodes**; a pair **agrees at either end** within 2 s (one end is often lost to a dropout
  or dialogue over the theme: South Park S1 and Invincible in the S3 fingerprints only reach a
  consensus that way), and the marker is the span the agreeing pairs share. Analysis jobs
  run at priority -5 (new files) or -10 (backfills), one slot each, every ffmpeg under
  `nice -n 19`. Without ffmpeg's chromaprint muxer series wait unanalyzed.
- **Movie credits**: the crawl detector samples the last 12 minutes at 2 fps and 480x270 (S3's
  card thresholds; a scroll of 1-40 rows per sample at a steady speed); a credits stretch is
  60 s with 85 % credits-like samples, followed by at least 60 % (a post-credits scene may
  follow). With both detectors the later start wins **unless the picture already shows
  credits all the way from the earlier one** (on the S3 logs the music often settles minutes
  into a crawl; taking it would air those minutes of credits). The music alone counts only
  when it lasts 90 s and ends within 45 s of the end (Harry Potter 3's parchment credits get
  no marker: the admin's backstop). Replayed on the S3 logs (keyframe stats standing in for
  the 2 fps frames): Iron Man 2 airs to its crawl's music (its dark designed titles start
  before the window), Pulp Fiction to its crawl.
- **Promo audio**: excerpts are at most 40 s (the loudest 40 s of a longer stretch, a later
  window only winning by 0.5 LU); a series' intro comes from the season where most episodes
  share one (at least three before smaller ones), its median episode.
- **Shots**: keyframes are the candidates (cheap on the Pi and on most cuts); cuts are scored
  as scdet scores them, computed on the same 160x90 frames rather than by the filter; a
  series' shots come from three episodes spread over it (two each). No face detector, but
  (wave 3, after the owner saw Pulp Fiction's overdose, a face filling the frame with blood
  on it, on every screen that shows the title's scenes) a **picture check** on the same
  thumbnails, now decoded with chroma: `pictureRisk` (`picture.go`, 0 safe to 1 avoid) reads
  gore (dark saturated red in or beside the biggest skin-toned blob), a close-up (that blob
  filling the frame: at most 0.7, never "avoid"), much skin and a frame mostly blood-red;
  skin is a hue (2-24 degrees at 28-68 % saturation, so dark skin reads too and tan walls,
  wood, pink and fire do not), fire and the sun's bright orange switch the skin readings
  off, and red bigger than the skin it is beside is a dress, not a wound. A stretch's still
  is the one with the most contrast less its risk, a candidate shot (rated every 0.5 s of
  its length, not only its first frame) is scored times one less its risk, and one with two
  frames in a row at 0.85 or more never airs (decided on every decoded frame: the review
  found the overdose's blood between the 0.5 s checks, 0.74 sampled against 1.00 at 3250.3 to
  3250.5 s), so wide and medium shots come before faces and gore is left out
  (a title may end with fewer shots; with none the promos use artwork stills). It is tuned to
  miss rather than to flag, because colour cannot tell skin from a pale warm wall: shadowed
  faces and cartoon skin mostly read as safe. On the local Pulp Fiction, Iron Man 3, Dexter
  and Futurama only the overdose shot changed (for a medium shot of a man at a window). The
  picks have their own version (`ShotsVersion`, `couchtv_titles.shots_version`, migration
  0034): production re-picks every title's shots at the next start through `picksOnly`
  jobs that leave the loudness and the promo audio alone, admin picks stay. The new picks
  change each title's `Title.footage`, which is in the specs of every screen showing its
  scenes, so their clip keys change and **those clips render again** (lazily, evergreen
  first) for the titles whose picks changed, and for no other. The extra cost is chroma
  conversion and the check, about 0.2 ms per usable keyframe and per sampled shot frame on
  an M1 (the decode, not the check, is what costs: locally the user CPU of a film's pick
  moved by less than the run to run noise, 9 s for Pulp Fiction and 16-18 s for Iron Man 3);
  expect the Pi to stay near its decode cost, about 2 s of Go more per film.
- **Loudness**: a movie gets a full ebur128 pass (it also feeds the promo fallback); a series
  four episodes spread over it, averaged by energy, stored per title (the break mix clamps
  it to -23..-16 LUFS anyway).
- **Airing to the credits**: the planner reads each file's effective `credits` start, so
  programs end there (`credits_at_ms`); days already frozen keep the plan they had.
- **Trailer scenes** (2026-10-14, D43; `trailer_*.go`): a part of `couchtv_analyze` with a version of its
  own (`TrailerVersion`, apart from `AnalyzeVersion`, so a better picker never measures the loudness
  of whole films again; an admin's scenes, `trailer_source = 'admin'`, are never overwritten).
  *Sound*: the dialogue is the original-language track, not the default one: an English stream of
  the picture's file (the default among several, then the most channels, never a commentary or an
  audio description), else an English alternate file of the same movie or episode (model B, same
  timeline), else the default track; scenes cut it through `StreamURL` (6.7), a surround stream
  downmixed with its centre channel up. *Text*: the fullest non-forced subtitle track in the
  dialogue's language (a track whose language is not known has none), read as WebVTT and cleaned
  (markup, sound descriptions, speaker labels, lyrics and on-screen captions dropped); cues less than
  a second apart are an exchange, and a scene plays a run of an exchange's cues that opens a thought
  and ends one with 1.2 to 6.4 s of speech. *Rating*: shape (2.5 to 5.2 s of speech is best), brevity
  (a dozen words), content (a question, an exclamation, a back-and-forth, stakes, names, the title's
  own words; minus filler, small talk, crudeness, carrying on a line before it), then the sound
  (how clearly the speech stands over what is behind it; a line buried in music or effects is
  refused) and the picture (not dark, flat, washed out or a montage; the first and last frame not
  black; and a violence check, `SceneViolence`, which `internal/app` hands to `Analyzer.Risk`: every
  frame of the scene is read for gore and for a frame of graphic red as the footage shots read them
  (`picture.go`; the picker's frames are limited range, converted to full), never for faces or
  skin, since a trailer's lines are dialogue shot in close-ups; a reading that two frames in a row
  reach counts, from 0.5 the scene is refused, below it the reading takes half of itself off the
  scene's picture score; it reads the plain case, a bloodied face at medium-close or closer, and
  not thin blood at a distance, which at 160 x 90 is a dozen dark red pixels like the shadow of a
  warm wall: the aftermath of Pulp Fiction's overdose, in the scenes a minute and ten minutes after
  its bloody close-up, passes, and no threshold that catches it spares Dexter's family kitchen). *Sync*: subtitles of another
  release are often a constant few seconds off, so the picker measures the offset on the sound of
  its 40 best exchanges (how many have their first cue where a pause ends and their last where one
  begins, and how much likelier speech is inside the cues than away from them, at every
  quarter second from -4 to +4) and shifts the cues by it (none when they are in step: another
  offset must beat it clearly); cues that follow the sound at no offset are not used, and the speech
  path takes over for the file. *Speech* (no subtitles, or out of step): a cheap voice-activity detector (the energy in
  300 to 3400 Hz in 20 ms frames against the noise floor around it, kept where it swells at a
  syllable rate, with a hangover) finds the utterances, bounded by pauses of 0.28 s or more, and
  the exchanges of one to four of them; two profiles of it, one that avoids false alarms (finding
  speech) and one that avoids misses (finding pauses). *Placing*: a scene starts in the pause that ends where the first line
  is heard to start, 0.45 s ahead of it (on the shot's first frame when a cut is within a second of
  that, and with no cut in its first second), and ends in the pause that starts where the last line
  stops, 0.6 s behind it (on the end of a shot when one is near, with no flash of the next shot in
  its last half second); a line with no pause either side, with the sound running on into it,
  is refused; lengths 3 to 8 s in whole frames. *Choosing*: the best first of up to 24 scenes
  per movie (12 per episode), 120 s apart in a file, at most three from an episode, not two that
  say much the same, at most 60 s together, in story order; a calm opener and a punchline count for
  the first and last place when they are good scenes; scenes below 0.5 go while more than six scenes and 36 s
  remain, so a short trailer of strong scenes beats a long one with filler; fewer than three is none.
  *Cost* (M1, `nice` 19 on a busy machine; the Pi is 4.5 to 8 times a core): a two-hour movie with subtitles (Iron Man 3: 7,833 s, 2,171 cues) took 60 to 80 CPU seconds, about 25 to 70 s of wall time on a busy machine (the sound of about 24 exchanges as 16 kHz mono, 20 to 60 s each, 4 s of audio decode per half hour, then 5 to 12 s of 160 x 90 frames for each scene the sound allowed); an episode with subtitles (Dexter, 53 min) about 70 CPU s, two 22-minute episodes together (Futurama) about 30; a movie without subtitles (Pulp Fiction, 154 min) listens once to the 70 minutes of its first half (about 4 s of decode a half hour) and took about 35 CPU s in all. On the Pi (4.5 to 8 times a core, `nice` 19) that is 5 to 10 minutes a movie and 2 to 4 for the three episodes of a series, once per picker version: the backfill at the first start after the deploy is 1.5 to 2 hours of the Pi's background time for today's library (12 movies and 6 series), behind the renders (the analysis yields to the sidecar).
- **Promo audio from the web** (2026-10-13, after the owner found the in-file excerpts poor:
  the library is mostly Czech dubs, so an intro or the credits can carry dubbed vocals or a
  voice-over, and a film's credits often end on a pop song or quiet score): a new
  `couchtv_soundtrack` job looks for the title's own soundtrack in iTunes Search and Deezer,
  both keyless with an official 30-second preview of every track (iTunes AAC, Deezer MP3),
  politely paced (iTunes one request every 4 s against its ~20 a minute, Deezer one a
  second). A pure matcher scores what they return, since searches are noisy ("iron man
  soundtrack" brings Black Sabbath, AC/DC's "Iron Man 2" album and "Epic Version" covers
  before Ramin Djawadi's score): the album named after the title (a different number is a
  different film, so each Harry Potter film gets its own score album), the track's role (main
  title, theme, title track, opening), the original recording over covers (TMDB's composer,
  the show itself as the artist, the title's own soundtrack album; karaoke, tribute, piano or
  epic versions and the usual cover acts never count), the year. Only a confident top match
  (from 0.7) is applied on its own; the admin hears every candidate through the server, picks
  one or searches with their own words. The bed is now: the admin's excerpt of the title's
  own audio, else the soundtrack from the web (the admin's choice or the confident match),
  else the analysis' excerpt. The chosen preview is downloaded once into `DATA_DIR`, and the
  render sidecar cuts its bed as FLAC (its Chromium has no AAC) from a URL naming the catalog,
  the track and the stretch, so a new choice renders the promos again. Run on the production
  titles (without TMDB), it picks each film's own score or soundtrack, the Simpsons' and
  Dexter's original main titles, Primus' South Park theme and Christopher Tyng's Futurama
  theme. **Terms**: the previews are meant for promoting the music; Couch TV is a private
  server playing them in its own promos for its few viewers, never redistributing them, and
  the catalogs' terms should be reread before anything here is offered more widely.

### 6.4 Admin operations (typed, `admin` prefix)
`adminGetCouchTv`/`adminUpdateCouchTv` (settings, template), `adminListAirings` (range),
`adminUpdateAiring` (pin, lock, swap, remove), `adminInsertAiring`, `adminRegenerateDay`,
`adminRotateCouchTvLink`, `adminCouchTvRenderHealth`, `adminGetCouchTvTitle` /
`adminUpdateCouchTvTitle` (excluded, promo audio, shots, the trailer's scenes: kept by their file and start, none being no trailer for the title, absent handing the trailer back to the picker alone), `adminGetMediaMarkers` /
`adminUpdateMediaMarkers` (in `library`), `adminCouchTvWaveform` (peaks for the picker).
Every operation gets a `TestAPIConformance` case. Built in Phase 1: `GET/PUT /admin/couchtv`,
`GET /admin/couchtv/airings?from&to` and `POST /admin/couchtv/days/{date}/regenerate`, not
gated by the flag. Built in Phase 3 (no UI yet): `GET/PUT /admin/couchtv/titles/{id}`, `POST
/admin/couchtv/titles/{id}/analyze` (`adminAnalyzeCouchTvTitle`, the re-run: markers, then
the title's analysis), `GET /admin/couchtv/media-files/{id}/waveform?fromMs&toMs&points`
(the loudest momentary loudness per step) and `GET/PUT /admin/media-files/{id}/markers`.

**As built (Phase 6)**; the full reference is FEATURES.md "couchtv" (endpoints, web admin).
- **The editor's operations**: `POST /admin/couchtv/airings` (`adminInsertAiring`: a movie or
  an episode at a time, 201 with the airing) and `PATCH /admin/couchtv/airings/{id}`
  (`adminUpdateAiring` with an `action`: `lock`, `unlock`, `pin` another title or episode in
  the row and/or from another start, `swap` with another program, `remove`). A pin, swap or
  insert makes an admin row (`source='admin'`), which replanning keeps like a locked one.
  Everything but a lock runs in the planning transaction and replans the program's broadcast
  day and every later one (`Scheduler.Edit`, which `adminRegenerateDay` now shares), so strips
  stay in order; frozen days are replanned too, being explicit admin edits (5.3). Each edit
  queues `couchtv_prepare` and `couchtv_compose` at once.
- **Removing a planner's pick has to stick**: the planner is deterministic, so deleting the
  row would bring the same program back at the next replan. A removed auto row stays as
  `state = 'replaced'` (every read already skipped that state) and the planner leaves its
  movie or episode out of that broadcast day: rotations and reruns choose another, a strip
  passes the episode over (it is skipped, not postponed; moving it is a pin). Removing an
  admin's own row deletes it, which gives the planner's choice back.
- **Fixed rows count toward their slot** instead of consuming it: before, a locked strip
  episode silenced the rest of its strip and a locked rerun turned its block into fillers. A
  slot now airs `count` minus its fixed rows; a one-program slot with a fixed row still does
  not run again.
- **Fills of admin rows** (`adminFill`): a row keeps its fill while its title stays (another
  episode of a strip restarts the strip there; the same film keeps a saga's place), else a
  movie takes `movie` in a prime-time slot (`movie`, `premiere`, `saga`) and `matinee`
  elsewhere, a series `reruns`, a gap filler stays `filler`; an inserted program is
  `matinee` or `reruns`. So an edit never moves another series' strip cursor, never calls an
  old movie a premiere and never advances a franchise.
- **Checks**: a placed program starts after now, within the planned week (`outside_horizon`),
  not into what is on air or right after it (`on_air`: an airing that started and runs up to
  the new start, since the break leading into it announces it), and a shortest break clear of
  every program that stays (`overlap`); its title is published, on the channel and playable
  (`not_published`, `excluded`, `no_media`). A program that started only takes a lock.
- **Stale compositions**: an unchanged break row keeps its id and so its composition; when
  the program beside it changes, its up next or squeeze would be stale. The diff now drops
  the composition of every not-yet-aired break beside a new program, and in-place edits do the
  same for the rows they change (`Store.resetBreaksAround`).
- **Config errors** carry stable codes (`couchtv.ConfigError`: `invalid_time_zone`,
  `invalid_slot`, `day_needs_anchor`, ...) with `problem_` strings, and the template editor
  repeats the checks to point at the slot and day before Save.
- **Schedule rows** carry a break's render state (`render`: `uncomposed`, `composed`,
  `rendering`, `ready`, `failed`); readiness is `couchtv_media` (6.6).
- **The web admin** (4.3): one Couch TV section with tabs for the schedule (a week of day
  columns in the channel's time zone, programs and breaks, a now-line, a side panel with the
  actions), the template, render health and the broadcast preview; the title editor's Couch TV
  panel (never on Couch TV, mood, promo audio over the waveform, footage shots, markers per
  file, the analysis re-run). Stills and the promo excerpt play through the playback payload's
  media grant for the file (the frame endpoint, its stream), never a URL of their own. The
  guest link (`adminRotateCouchTvLink`) waits for Phase 5's guests.

### 6.5 Break composition
- Inputs: the gap (next start minus this program's `credits_at`), the slot (top of hour?
  before 20:15?), promo candidates weighted by airing soon, premiere, strip, and not shown
  in the last breaks.
- Output: an ordered recipe of clips summing exactly to the gap at 25 fps, plus the per-break
  audio mix description (beds, SFX cues, crossfades, ducking, target loudness).
- **Loudness**: programs are measured once (`couchtv_analyze`); a break is mixed at the
  loudness of the program around it (clamped to -23..-16 LUFS), so breaks never blast.

**Settled in Phase 3** (`couchtv.Compose`, `couchtv.Composer`; FEATURES.md "couchtv"):
- **Order**: squeeze (`ident` handover into the short ident; `expand` straight into up next
  when a break is too short for an ident, 17.5 to 22 s), ident (the full cut when the next
  program is the evening movie: `movie`, `premiere`, `saga`; its design the broadcast day's,
  D35), promos, then the lineup (only
  when the break ends on the hour and a promo still fits beside it), stings, up next. The
  minimum is 21.8 s (squeeze 10.6, short ident 5.2, the shortest up next 6); from 17.5 s the
  squeeze expands into up next, from 6 s up next stands alone, from 5 s a sting alone; under
  5 s nothing fits and the break fails over to the evergreen one (the planner never makes a
  break under 23 s).
- **Closing the gap**: up next (6-12 s), the lineup (10-16 s) and stings (5-10 s, as few as
  possible) take up whatever the fixed parts leave, every part at its default before any
  grows further; promos may borrow what up next and the lineup can give back. A sting's
  longest is twice its shortest, so stings alone close any gap of 5 s or more, whatever the
  parts around them (a longer bulletin slot, a trailer-length promo); where nothing flexible
  (up next, the lineup) can take up what a promo leaves, a promo that would leave under 5 s
  gives way to a smaller one, the rule a slot's fallback has always followed. Lengths are
  whole frames; a gap that is not (a program airing to a segment boundary) is rounded to the
  nearest frame, and the live stream fits the item to its slot.
- **Promos**: each title at its first airing after the break, ranked by its fill's weight
  (premiere 6, movie and saga 2, strip and marathon 1.5, matinee 1, the rest 0.4) over one
  plus the days until it airs; the next program is left out (up next announces it) and the
  titles of the last three breaks come last. A title gets the full promo (body plus end
  card) once its body is rendered (bodies are requested as soon as a break could use them),
  the strip promo when it is a strip at a fixed time, else the short promo. A gap longer
  than one round of candidates starts another round after a short ident (very long gaps
  repeat promos; with nothing to promote the full ident repeats), so the planner's fillers
  matter for long gaps.
- **Trailers** (2026-10-14, D43; `compose_trailer.go`): the long promo for the strongest candidates. A
  candidate holds its title's scenes as spec values (`Candidate.Trailer`: each scene's frames, its
  own sound as a bed from its audio file and stream, its lines joined where closer than 0.35 s) and
  `TrailerFrames`, the length of its rendered body (0 until rendered). When a break has room
  `promos` airs one trailer first among its promos: the best-ranked candidate whose trailer is
  rendered, that is eligible (premiere, movie, saga, fill weight 2 or more; a strip or marathon only in
  a break of 8 minutes or more), that is not the next program, whose trailer did not air in a break
  within four hours either side (`BreakInput.RecentTrailers`, from the breaks' `recipe.trailers`;
  the Recent rule alone only reaches three breaks back) and whose body and end card fit in
  what is left plus what the flexible parts give back (so the break keeps its squeeze, ident and
  up next); the other promos fill the rest, and the title is not promoted twice. The trailer is
  `Part: "body"` (timeless, rendered once per title), its end card `Part: "end"` with the airing
  (4 s like a promo's, no sound of its own), the break's sound item the `full` spec with the
  body's fixed airing in its key (a trailer sounds the same whenever it airs); clip kinds
  `trailer_body` and `trailer_end`. The slots' fallbacks (`slotFallback`, 15 to 60 s) never carry
  one. A scene of a file that is gone or lost its source is left out of the spec (one failed
  extraction fails a render), and a trailer with fewer than three scenes is none; nothing is composed
  or asked for before the sidecar publishes the `trailer` template.
  Trailers are asked for as promo bodies are, for the candidates a break could use, the best-ranked first, but
  only while fewer than three exist that are waiting, rendering or were rendered in the last 18
  hours, and they render behind every break (`tidyRenders`): see 6.7 for the cost. A promo or
  trailer body whose key is no longer the current one of its title (new artwork, shots or scenes,
  a template version: each asks for another) is cancelled by `tidyRenders` and its clip dropped,
  or its render would run before the current ones (all bodies share a priority, the oldest is
  claimed first) and the dead trailer would count toward the three. The Pi had 71 body renders
  pending for 18 titles on 2026-10-14, about fifty of them stale.
- **Phrases**: `Today`, `Tonight` (from 17:00), `Tomorrow`, `This week` with the day,
  `Premiere`, `Saga` (Saga Sunday) and `Marathon` (the weekend block) with the day; strips
  `Every night`/`Every day` (all week), `Every weekday`, else `Every week`, with their days
  lit.
- **Inputs**: English names and taglines (the base text when the base language is English,
  else the English translation; the tagline split in two near the middle, at punctuation if
  close); backdrop original and poster w780; footage shots from `couchtv_titles.shots` as
  `[{mediaFileId, startMs, durationMs, score}]`, else stills from the artwork; the bed is the
  promo soundtrack excerpt, else none (the template's own sound); the mood from
  `couchtv_titles.mood`, else from the genres. The squeeze shows the program from where it
  left the air (the break's start), only when credits markers exist; otherwise the program
  aired in full and the squeeze shows its backdrop without a credits roll.
- **The title's own logo and scenes** (2026-10-13, D41): every title in a spec carries its
  logo in the language of its name (English, else the base language's for a title without an
  English name; else one not tied to a language), bounded to 1600 x 800 with the aspect of its
  stored size, and, on every screen that shows it in motion (up next, the squeeze's next card,
  the lineup's rows, the strip, short and full promos and their end cards; not the squeeze's
  ending, where the program plays), its footage shots as the promo body's frame URLs, none
  without analysis (nor of a file that is gone, which would fail the render). Those screens
  render for every break, so they take the backdrop 1920 wide; only the promo body, rendered
  once per title, keeps the original its stills crop into.
- **Loudness**: `couchtv_titles.loudness_lufs` (new, for `couchtv_analyze` to fill) of the
  program before the break, else after it, clamped to -23..-16 LUFS, else -18. Clips render at
  the default -18 (their sound is only heard on its own); only the break spec carries the
  break's loudness.
- **Evergreen**: the full ident (always journey), up to three rendered promo bodies (by title
  name) with the evergreen end card, the short ident. The broadcast package has no generic up
  next, so the live stream cuts the evergreen break to the gap and loops the standby card past
  its end. It is recomposed whenever its clips' keys change; the newest assembled one is kept.
- **Lifecycle**: the compose job runs at startup, after every plan run and hourly, composing
  the next 24 h (48 h until the first day on the Pi showed how much a replan wastes); a replan
  or swap drops a break's composition with its airing, and the run cancels the renders no
  composition uses any more; renders wait for the quiet hours (01:00-09:00) unless what they
  are for airs within 3 h, so the Pi renders at night; a failed break
  is composed again after 6 h, a failed clip is requeued when a break asks for it again.
  Renders are queued soonest-airing first (the job priority is minus the airing's minute;
  the evergreen break and standby card first of all).
- **Bulletin slots** (2026-10-13, 6.10, D36-D37): a break that carries the news or the weather
  keeps a slot of the bulletin's length before up next, holding promos and stings of exactly
  that length, which air when no bulletin can.

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
    **constant** target duration (RFC 8216 forbids changing it): 16 s, with programs whose
    package has a longer segment (`LiveItem.MaxSegment`) airing from the 1080p rung. (It
    was 10 s until the first day on the Pi: almost every copied source had a few 11-16 s
    segments, so nearly every program waited for a 1080p encode that runs about as long as
    the program, and the channel aired the standby card instead.)
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

**Settled while building it (Phase 4)**; the full reference is FEATURES.md "couchtv".
- **What an airing airs is pinned with its origin.** Besides `origin`, an airing stores its
  `feed` when it goes on air: its parts (a program from its package, a break's rendition, the
  slate), each with its time on air and the renditions it reads. A break that finishes
  rendering mid-air, a rung that arrives later or a restart can then never change what is
  listed. Airings are pinned in order by the first playlist request after they start (state
  `aired`); an airing whose predecessor never went on air (nobody was watching) starts a fresh
  numbering from the clock (two media sequence numbers and one discontinuity per second since
  2026-01-01, more than a chain grows), so numbers never go back.
- **Programs start on time; breaks absorb the rest.** A program starts at its row's start and
  airs to the video boundary nearest `credits_at_ms` (else its end), never past the next
  program's start; its break runs from there to the next program. `couchtv_prepare` makes the
  rows exact beforehand: it records each airable file in a new `couchtv_media` table with its
  cut (`on_air_ms`, measured for the credits it saw), the planner's `Media.onAir` uses that
  cut, and frozen rows are fitted to it (moving the break, which loses its composition), so a
  break is composed for the gap that airs. Compose a break once its program is ready to avoid a
  recompose.
- **Readiness is `couchtv_media`.** A file is ready when its package has something the channel
  can air: the copied source if it is 8-bit SDR H.264 up to 1080p with no segment over 10 s
  before its cut, else the tallest H.264 rung up to 1080p (a VOD 720p rung counts while the
  channel's 1080p rung transcodes), plus its AAC audio. The job queues the package and the rung
  of the source's class (1080p at most) through `library.Prepare` and runs every 10 minutes,
  after each plan run and at startup, running the readiness swaps each time (5.4).
- **Audio and subtitles**: per channel language, the file's own AAC rendition in it (the default
  among several), else the original; a break's mix in both; a file without audio airs the
  slate's silence. Subtitles per language, a full track before a forced one, empty WebVTT
  otherwise. The default audio follows `?lang=`, else Czech.
- **Breaks and fallbacks** (the render sidecar's model, 6.7 "As built"): a break airs its
  assembled package (`Store.ReadyBreak`); one that is not ready airs the evergreen break
  (`Store.Evergreen`) cut to the gap, then the standby card (`Store.Standby`) looped for the
  rest; without either, the built-in slate is the last resort. A break's own package shorter
  than the gap airs after a filler, so its up next counts into the program; a longer one is cut.
  The same fillers (the standby card, else the slate) cover a program that cannot air, a hole in
  the schedule and, for up to 6 h, the time after the last airing.
- **Looping**: the standby card and the slate loop by retiming: copy n is segment n mod S of
  pass n / S, moved by whole passes (`mp4.Retime`), so a stretch of any length is one item of
  whole segments and its last copy is cut. The slate is generated, not rendered: one 2 s segment
  per track (a dark 1080p25 frame with "Couch TV" through ffmpeg's drawtext, plain where ffmpeg
  lacks it; H.264 High without B-frames; silent AAC).
- **The grant and the routes**: `grant.Channel` (resource `grant.CouchTv`, 6 h; a guest will
  carry their id in the grant's `Couch` field) on `Routes.Channel` (`/tv/{grant}`, gated by the
  flag). Every file is reached through an airing (`airings/{id}/{track}/{file}`), served only
  within 10 minutes of its time on air and only from the renditions its feed names; the
  standby card and the slate have routes of their own. `tuneCouchTv` hands out the grant, the stream, `serverTime` (ms) and
  now/next; `getCouchTvGuide` lists programs.
- **Measured** (`make couchtv-live`, M1 Pro): `validate-hls` finds no error around program,
  break, evergreen-after-slate, slate and hole boundaries (the `AVERAGE-BANDWIDTH` warning of
  S1 remains); the players' results are in the Phase 4 entry of the roadmap.
- **Bulletin slots** (2026-10-13, 6.10, D36): a break with slots airs its sections, a part
  each (the parts after an airing's first under `airings/<id>/parts/<n>/`); a slot binds when
  the break goes on air, to the newest rendered bulletin still valid when it ends, else its
  fallback. Measured (`make couchtv-live` three times, the M1 Pro under a load average of 20-45
  from other work): `validate-hls` finds no error around the edges of a slot that airs a
  bulletin (into the break, into and out of the bulletin, into the next program);
  AVFoundation plays through them (9 boundaries, no stall, the program date about 1.1 s
  behind the clock over 128 s); hls.js matches picture and sound to the program date and
  changes item within 0.35 s of it for every viewer. The viewers on hls.js's own live edge
  fail the drift check (their program date 2-8.6 s behind the clock where 1.5 s is allowed,
  items at 84-100 % speed under that load), as the same test without a slot does on the same
  machine (4.0 and 3.7 s, items at 93-100 %); in one run every viewer lost about 2 s at one
  moment of wall time while the bulletin was on screen, in the last it played at 98-100 %
  speed for all. Worth a run on an idle machine.

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

**As built (Phase 3)**; the full reference is FEATURES.md "couchtv".
- **Process**: `couchverse render` is the same program as the app, built without the SPA for
  its image (the Dockerfile's `render` target: Debian trixie `chromium`, `ffmpeg`, Mesa GL and
  EGL; compose service `render` behind the `render` profile with `/dev/dri`, `/dev/video11`,
  the data volume, `mem_limit: 1g`, `cpus: 2`). It waits for the app's migrations, lowers its
  priority to nice 10 (Chromium and ffmpeg inherit it) and works only `couchtv_render` and
  `couchtv_assemble`, one job at a time. A job runner now requeues only the running jobs of
  its own types at start (the sidecar and the app never requeue each other's), and
  `Runner.Yield` keeps renders from being claimed while a `transcode_hls` runs anywhere.
- **Keys**: the sidecar publishes the bundle's template versions (`couchtvRender.versions()`)
  in settings key `couchtv_render` at start, with a heartbeat after every job; the app keys a
  clip as a hash of its template's version and its spec and composes nothing until a sidecar
  has published. Media in specs are URLs on the sidecar's localhost server that say what they
  are cut from (`/media/v1/artwork/<id>?v=<version>`, `/media/v1/frames/<file>/<start
  ms>_<count>/<n>.jpg`, `/media/v1/audio/<file>/<start ms>_<length ms>.flac`; `v1` is the
  extraction recipe), so a new image, file or recipe makes a new key and nothing else needs
  versioning. Artwork may name a size the server makes once and caches: the artwork feature's
  (`w780`) or the screens' (2026-10-13, D41: `w1920`, a backdrop at most 1920 wide, and
  `1600x800`, a logo within that box).
- **Media**: extracted once before a clip loads (promo footage as frames: 25 fps, square
  pixels, within 1080p, SDR tone-mapped through `playback.SDRFilter`, served `no-store`; beds
  as stereo FLAC of the default track, or, for a trailer's scenes, of the named stream:
  `/media/v1/audio/<file>/<stream>/<start ms>_<length ms>.flac`, a 5.1 or 7.1 stream downmixed
  with the centre channel up (weights adding up to 1, so a loud scene does not clip) and cached
  beside the default cuts as `s<stream>_<start>_<length>.flac`, so the existing beds keep their
  URLs and keys; an alternate file in the original language is named with the default form), touched on reuse, pruned after 14 days unused.
- **Frame loop**: one CDP evaluate per frame (`advance`: `isStatic` against the previous frame,
  else the frame set up without waiting for a paint) and one `captureScreenshot` (JPEG q92,
  `optimizeForSpeed`, from the surface, which forces a fresh frame of its own; the earlier
  double `requestAnimationFrame` wait drew every frame twice); a static frame reuses the
  previous JPEG. ffmpeg reads them with `image2pipe` and writes the video rendition directly:
  libx264 `veryfast`, `-tune animation`, CRF 18, High@4.0, a 2 s GOP without scene cuts (an
  IDR at the first frame), JPEG's full-range BT.601 converted to limited-range BT.709 and
  tagged so.
- **Squeeze composite**: the composer still writes `program.frames`; the sidecar loads the
  squeeze keyed instead, reads `keyRect` for every frame and has the encoder decode the program
  from its source (from the frames' start, cover-cropped, SDR, its last frame held), scale it
  to each frame's box and lay it over the picture through a matte the sidecar draws from the
  same rectangles (antialiased rounded corners times the opacity, a second pipe). Chromium
  decodes no program frame and none is extracted (FEATURES.md "couchtv").
- **Sound**: the page posts `renderAudio()`'s WAV back to the sidecar's server (too big for
  base64 over CDP); AAC 192k. A clip's render also keeps its sound as a break item
  (`renderItemAudio()`, `couchtv/sounds/<key>.wav`: from its start, faded out where the next
  item starts with its ring-out after, at the engine's level), and an assembly mixes the
  break's items' kept sounds in Go at their starts, normalizes the sum as the page does
  (BS.1770 to the spec's loudness, true peak at or under -1 dBTP) and checks its length to the
  sample; an item without a kept sound is rendered then, once. (A break's sound used to render
  in the page as one break spec, in parts of up to 120 s: see "Render throughput" below.)
- **Packages**: a clip (`couchtv/clips/<key>`) and a break (`couchtv/breaks/<id>`) are laid
  out like a Playback v2 package (`source/` video and `audio-1-aac/` English stereo, each
  with `rendition.json`, 6 s fMP4 segments on the package timeline through
  `playback.RenditionArgs`, plus a `master.m3u8` for checking), so
  `playback.ReadPackageDir` reads them and `validate-hls` checks them; the video is checked
  to the frame. Assembly joins the clips' video by stream copy (each clip's init and segments
  end to end as one fMP4, the concat demuxer at exact lengths) and segments it with the
  break's sound in one ffmpeg run.
- **For the live stream**: `Store.ReadyBreak(airing)`, `Store.Evergreen()` (the newest
  assembled evergreen break, to cut to a gap) and `Store.Standby()` (the 12 s standby card, a
  seamless loop).
- **Leaks and failures**: Chromium restarts every 25 jobs (decoded artwork is cached
  for the page's life); any failed job closes it so the next starts clean.
- **Bulletins** (2026-10-13, 6.10, D36, D38): a break with slots is assembled in sections, a
  package each, its sound mixed per section; a bulletin's clip renders as
  `couchtv_bulletin_render` (priority 1, not yielding to transcodes) and carries its sound at
  every whole LU from -23 to -16.

**Render throughput** (2026-10-13). On the Pi a break's sound, rendered in the page as one
break spec, ran at 0.15-0.2x real time (120 s parts of 13-14 clips took 11-14 min), so a
5-minute break spent 30-50 min on its sound after 6-10 min of clips, ~48 breaks a day asked
for more than 24 h of the sidecar, and through most of 2026-10-12 breaks aired the evergreen
break or the standby card.
- **Why** (the S2 break on an M1 Pro): one render builds every item's voices before it starts
  (S2: 3806 nodes: 1544 gains, 1154 oscillators, 439 biquads, 327 panners, 189 buffer
  sources, 108 2x-oversampled waveshapers, 38 delays, one convolver, two compressors; the
  short ident alone has 1655) and Chromium pulls every connected node every 128 frames from
  the render's start, so the cost grows with the break's length times all its voices: S2's
  60 s rendered in 15.6 s, its seven items alone in 5.7 s; a composed 300 s break of 20 items
  took 599 s, slower than real time even here. Of S2's render the hall (a 3.2 s stereo
  convolver) took 29 %, the compressors 12 %, the oversampled waveshapers 12 %; building the
  hidden timelines took 0.1 s.
- **Now**: offline renders schedule each cue 0.5-1 s before it plays (`suspend`; the samples
  are the same within 4e-6; S2's one render 18.9 s to 10.2 s, a clip's own sound up to 1.9x
  faster) and the hall, noise and projector buffers are made once per rate. Each item's sound
  renders once, with its clip, and is kept (`couchtv/sounds`); an assembly mixes the kept
  sounds in Go and opens Chromium only for an item without one.
- **Measured** on the M1 Pro's two efficiency cores, the Pi proxy above: the S2 break
  assembles in **5.4 s instead of 127 s** (its sound 2 min 1 s before, the mix 1 s now; the
  rest is joining the video and encoding AAC); 47 s when none of its seven items has a kept
  sound yet (each 4.4-8.8 s), as right after this change, when clips rendered before it make
  their sounds as their breaks assemble. A 300 s break of 35 clips (S2's seven, five times)
  assembles in **23 s instead of 22 min** (its mix 4.9 s; before, three parts of 9 min 31 s,
  9 min 20 s and 2 min 49 s, much as the Pi's 120 s parts of 13-14 clips took 11-14 min).
  Keeping an item's sound adds 2.5-11 s to its clip's render (48 s over nine clips) while the
  clip's own sound got 14 s faster (53 to 39 s): a few percent of a clip's 1.5-2.5 min. In
  the page alone on a performance core, composed breaks of 190, 300 and 320 s (14-21 items)
  took 315, 599 and 800 s in one render and 21, 37 and 34 s with every item rendered alone.
- **Sound**: mixed items against one render of the same break (S2 and the three composed
  breaks): integrated loudness within 0.001 LU, true peak within 0.01 dB, each item within
  0.01 LU, every 400 ms window within 0.15 LU (95 % within 0.03 LU) after the first second,
  where one render opened its compressors cold (0.39 LU). Samples differ 31-33 dB under the
  signal: Chromium quantizes automation times to samples, so a voice whose pitch moves lands
  in another phase depending on where its item sits, as it did from break to break before
  (one render put the same short promo 5 s or 7.5 s into a break 40-44 dB apart). Items
  rendered where the one render had them differ from it by 70 dB under the signal (57 dBFS
  at worst, at a boundary): the glue and the limiter running per item instead of over the
  break, inaudible.
- **Trailers** (2026-10-14, D43): a trailer body is about 1,900 frames (a 75 s body) at the sidecar's 2.0 to 2.5 fps, 13 to 16 minutes on the Pi, plus extracting its scenes' frames before it loads (on the M1 140 frames of 1080p took 1.85 CPU s, 75 fps a core, so 1,000 to 1,400 frames are 15 to 20 CPU s there and 1.5 to 2.5 minutes on the Pi; a scene's sound is cut in 0.1 s) and rendering its sound as a break item (a minute or two): 17 to 20 minutes a trailer, and about 80 MB of extracted JPEGs (pruned after 14 days unused). They are asked for only for candidates a break could use (premieres, prime-time movies and sagas: 7 or 8 titles in a week of today's library; a strip or marathon only in a break of 8 minutes or more) and at most three at a time (those waiting, rendering or rendered in the last 18 hours), so the first night after the deploy renders three (about an hour of the 8 quiet hours, beside the day's other clips' 3.5 to 4 h) and the week's trailers are done within about three nights; afterwards a night renders the one or two that a changed schedule needs. A trailer's end cards are ordinary 4 s end-card renders, one per airing phrase.
- **Daily budget** (Pi, the measured clip speeds: squeeze 2.41 fps, up next 2.47, lineup
  2.66, short promo 1.98; ~25 s to assemble a 5-minute break): a week planned over a library
  shaped like production's composes ~31 breaks a day whose new clips are ~30 squeezes and up
  nexts, 13-31 end cards, 6-11 short promos and 3-5 lineups (stings, idents, strip promos and
  bodies come from the cache): 2.1-2.6 h of video, ~4 min of items' sounds and ~13 min of
  assembly. At ~48 breaks a day that is ~3.5-4 h + ~6 min + ~20 min, inside the 8 h of quiet
  hours; before, assembly alone was 24-40 h a day. So the policy stays (the quiet hours, 3 h
  before airing at the latest), and render health's time to clear, which counts the clips'
  video at their measured speed (each clip's sounds included) and leaves assemblies out, holds
  again now that an assembly takes seconds.
- **The full idents** (D35, 2026-10-13) carry no per-break data, so each design renders once per
  template version: four clips, needed one day apart, each requested a day ahead and rendered
  in that night's quiet hours. They cost far more than other clips, because every frame is
  drawn on a canvas several times over (motion blur, up to 64 sub-frames). Measured on the M1
  Pro through the sidecar's pipeline (capture, libx264 veryfast, both sounds), on a machine
  shared with other work, so CPU time is the measure; the Pi is taken as 4.5x to 8x an M1 core
  (the up next and squeeze renders, 2.47 and 2.41 fps on the Pi, ran 4.5 and 5.5 times their
  total CPU time on the Mac):

  | Design | Frames drawn | Mac CPU | Pi estimate | Page heap and buffers | Renderer |
  |---|---|---|---|---|---|
  | assemble | 201 of 263 | 47 s | 4-6 min | 12 MB | 354 MB |
  | morph | 224 | 73 s | 6-10 min | 135 MB | 301 MB |
  | journey | 192 | 123 s | 9-16 min | 119 MB | 269 MB |
  | cinematic | 263 | 870 s | 65-116 min | 136 MB | 387 MB |

  (A promo's renderer peaks at 280 MB the same way.) Before, a render read its canvas back
  once per sub-frame and V8 left the buffers to pile up: journey's page held 1 GB and
  cinematic's renderer 1.8 GB within a few frames, past the sidecar's 1 GB container; now every
  64 MB of reads collects them (`--expose-gc`). Cinematic also counted the dusk hills' motion
  after the camera had gone through the window, where they are not drawn, and drew 64
  sub-frames where its room needed 13 to 59: 5184 sub-frames instead of 6435, its changed
  frames within 62 dB PSNR of the approved render. Its slowest frame is about 10 s of Mac CPU,
  a minute or two on the Pi, inside the 3 minutes a page call gets. At the estimate's middle
  (~6x, 87 minutes) cinematic fits a night beside the day's other clips, but it is the one
  that may not: if its first render on the Pi (render health shows the speed) runs past ~90
  minutes, ship it pre-rendered (render it with the same bundle elsewhere and register the clip
  package and its item sound on the Pi) or split its sub-frames across two workers, which the
  sidecar's two CPUs would run in parallel.

### 6.8 The broadcast package
- `clients/broadcast/`: a small TypeScript bundle (Vite library build, no framework), using
  the generated design tokens and Plus Jakarta Sans. It is the productionized samples:
  `core` (timeline, curves and springs from the tokens, accent palette from the core's
  derivation), `audio` (engine, synthesized SFX kit, the house instruments, beds, ducking,
  voice bus reserved), `templates` (one module per template, data in, timeline out).
- **Determinism rule**: every frame and every sample is a pure function of t and the data
  (seeded PRNG, no wall clock, no accumulated `requestAnimationFrame` state); a CI test
  renders golden frames and an audio hash.
- The same bundle powers the admin **Broadcast preview** (live in the browser) and the
  sidecar's renders, so what the admin approves is what airs.
- Templates (from the samples, D27; the brand since D33 and D34): `ident` (full and short; the
  full cut in four designs since D35), `sting`, `upnext`, `promo` (moods `cinematic`, `grit`,
  `bright`; premiere variant; body plus end card), `strip`, `shortPromo`, `lineup`, `squeeze`,
  `standby` ("Technical difficulties"), `evergreenEnd`, `trailer` (the long promo, D43).

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
- **Durations**: full ident 10.52 s and short ident 5.2 s (the reel's 10.5 s grid cut rounded
  to whole frames; the short one was 4.5 s, lengthened on 2026-10-14 so its chord can ring
  out); the expand squeeze 11.52 s, the ident squeeze 10.6 s; stings any whole number of
  frames from 5 to 10 s (one design scales to it: the popcorn drops onto the couch, later the
  longer the sting, and the hold takes the rest), so a gap can be closed by one sting; up
  next 8 s by default (6 to 12); a promo body ends on a frame-aligned cut snapped to the bed
  (cinematic about 16 s, premiere about 17.6 s, grit and bright about
  12.8 s, as the screens' design times them) and its end card is 4 s; the body is
  airing-independent by construction (a test proves it), and the break audio renders the promo
  `full` so its bed runs across the cut.
- **The promos in the new brand (2026-10-14)**: the promo and the evergreen end card follow the
  screens' design (2.3, 2.4) made sharp and flat at the owner's word (right angles only, no
  shadows or glows), with the title's own scenes in every picture (its footage frame by frame,
  scope footage whole in grit's letterbox, cover-cropped toward its focus elsewhere) and its
  logo wherever it shows big (else its name). The end card closes in as two panels, a night
  column with the title and the airing and the title's scenes beside it, and lands the channel
  tag on the sonic logo; with a footage end card every frame moves (100 of 100 captured; a
  stills-only one holds still once the tag has landed). Without a bed each mood has its own
  house score; over a bed the house only punctuates. A logo never sits on a box (the owner,
  of the bright promo's Simpsons logo on a black one): bright's title lands on the field its
  logo reads on, red first (the sheet that has been sliding across stays), paper for dark
  logos (the sheet leaves and uncovers it), else over the picture under the night grade, and
  the night screens draw a logo too dark for night with its colours lifted, hue kept.
  FEATURES.md "Promo look" and "A title shown big" have the detail.
- **Audio**: 48 kHz; a clip's or a break's render is normalized to its `loudness` (BS.1770
  integrated, default -18 LUFS) with the true peak capped at -1 dBTP (quieter rather than
  clipped), delivered as a float WAV; a break spec lists clips and their starts, each ringing
  out under the next (`crossfade`, 0.3 s). Two renders differ only by float noise (< 1e-5).
- **Squeeze**: the program panel's motion is data that drives both the animation and
  `keyRect(t)`. The simplest path is `program.frames` (the page draws the program's last
  seconds as JPEG frames; no compositing), which the golden tests use. For an ffmpeg
  composite, what the sidecar does (6.7), the panel is left transparent (`program.kind = key`)
  and a `matte` layer carries the alpha; a colour key was rejected (JPEG and 4:2:0 fringe it).
- **Static holds**: `isStatic` skips the holds between a whole-piece animation's keyframes
  (linear `motion`/`windows`) and steady animations, but today's templates keep subtle
  motion in nearly every frame (slow pushes, grain): only the brand's still stretches (the
  short ident's couch alone and its held lockup, a sting's holds; the bug's LIVE marker no
  longer breathes) are static, so the 6.7 budget should not count on skipped frames. Freezing the
  rest would change the approved motion, so it stays.
- **The screens around the programs (2026-10-14)**: up next, the squeeze, the lineup and
  standby were rebuilt from the screens' design spec in the new brand, as the owner directed
  that night: sharp (no rounded corners anywhere), flat (no shadows, glows or glassy blurs), the
  red as structural blocks and rules, type rising into place from behind its own edge; a title
  shows big as its own logo and in pictures as scenes from itself (D41), its artwork only as
  the fallback. Their parts are one module (`templates/onair.ts`); the reference is FEATURES.md
  "Broadcast package". The squeeze keeps the brand's `ident` handover as built and restyles
  around it; its `expand` push ends on up next's first frame, scenes and all. Scenes cost what
  their frames cost to decode on every frame they show (accepted for real scenes); nothing
  else moves on held frames: the lineup between its steps and the squeeze's hold are still
  when their titles have no footage.
- **Brand isolation**: the lockup, corner bug, channel tag, sonic logo, idents and stings
  live in `src/brand`, so a redesign replaces that folder only.
- **The full idents (D35)**: the four idents approved on 2026-10-13 are the full cut's
  `design`s, each in `src/brand/idents/<design>` with its picture and score, moved from the
  planning harness. Every frame renders as the harness renders it (263 of 263 pixel-equal per
  design) and the sound within float noise of the approved renders (-131 dB under the signal),
  after two fixes the production pipeline needed: journey and assemble score as one cue built
  before a render starts (their voices start slow oscillators up to a period early, which a
  cue scheduled shortly before it plays started late), and journey's echo enters its feedback
  loop through one delay (Chromium entered it from either side, so its right-hand repeats
  landed a quantum apart from render to render). Journey's and cinematic's motifs were re-scored
  to the sonic logo, A4 E5 D5; the old variant B full cut and its `posters` are gone. Cost and
  memory: 6.7.
- **The old brand (D27, ported from the reel; superseded by D33 and deleted once the stings, its
  last users, moved to the new one: the bracket wordmark, the brass theme, Saira and the kit's
  bracket knock and key click)**: C()UCH TV with Saira bundled (no CDN), ident variant B (full and
  short) and theme 1 as the sonic logo, its tag closing every promo and stings of 2-6 s derived from
  it. The brackets are drawn as superellipses fitted to Saira's O and scale whole, as the reel
  scaled the glyph. (The port first kept their stroke at the lockup's weight when they hug a poster;
  on air that read as a thinner, different font beside the words they frame, and the owner asked for
  the reel's brackets back on 2026-10-12: ident and sting version 3.) The short ident shows a
  stand-in poster card (the spec's `posters` went with the old full cut, D35). Cuts sit on the frame
  nearest their beat (at 25 fps a 120 bpm grid falls between frames). The squeeze's `cursor`
  handover (replacing `line`) ends on the short ident's first frame pixel for pixel. Measured
  against the reel at 48 kHz, the theme matches within 0.1 dB in every band and 0.1 LU in loudness.
  The engine's reverb is now the reel's generated hall (every template's version moved to 2).
  `lockupSvg` gives the clients the lockup for their live overlays (FEATURES.md).
- **The brand (D33, D34, built 2026-10-13)**, `src/brand` (FEATURES.md "Broadcast package",
  "Brand"); the notes the screens built on it follow.
  - **Colours**: `BRAND` in `wordmark.ts`: red #e50914 (the token accent; the logo and `tv`),
    night #050505 (the field, the knockout), paper #f4ede4 (all type on air, as the screens'
    design spec has it: morph's warm white), white #ffffff (the lockup's `couch`, as all four
    approved endings draw it).
  - **Type**: Plus Jakarta Sans only (`FONT_FAMILY`), the wordmark at 800 and -0.035 em as
    outlines (`PARTS`); the bug's LIVE at 19 px 800 with 0.12 em tracking, uppercase.
  - **The logo** (`glyph.ts`): the couch and the popcorn; the popcorn knocked out round its
    `SILHOUETTE` by `LOGO_GAP` (28.5 logo units) is the app's logo.svg exactly (the short
    ident, the stings, the tag, the static lockup). The full idents halo the whole popcorn path
    by `GAP` (30), as their concepts did: under a pixel wider at the lockup's 540 px.
  - **Lockups** (`geometry(size, layout)`, `size` the word's px): `stack` as the idents end
    (on the stage: `STAGE` in `ending.ts`, the logo 540 px wide at (690, 222), the word at
    184 px with its line box's top at 650, its advance centred); `row` 1.06 sizes tall and 5.63
    wide (the logo 36/34 of the word's size, the word's line box sharing its top, the c 0.32
    sizes from the logo).
  - **The corner bug** (`bug`): its right edge on x 1824, its top on y 64, 40 px tall: the row
    lockup at 34 (the logo 36 px tall), 18 px, a 2 x 26 px rule (paper 24), 18 px, a steady
    10 px square red marker and LIVE (19 px 800, +0.12 em), 18 px, the clock (28 px 700,
    -0.01 em, tabular), in paper, no plate; nothing in it moves, so a screen's held frames stay
    still. A screen's grade shades the corner (`scrim` adds the shade where it does not: night
    0.6 to 0.33 at 45 % to none, 760 x 300 from the corner). The web's live overlay draws the
    same on the stream's 16:9 frame over the same shade.
  - **Sharp and flat** (D42): no border radius but the logo's curves and the type (the squeeze's
    program panel, its key rectangle and matte are square since squeeze version 6), no shadows,
    glows or bevels; night corner shades are the one legibility device.
  - **The channel tag** (`channelTag`): the row lockup 80 px tall in the tag slot (its right
    edge on x 1824, its foot on y 1008); the couch fades in 0.7 s before `at` (0.4 s, sine), the
    popcorn drops from rest 1.1 logo heights up over 0.42 s and lands at `at`, couch, t and v
    type in at `at` + 0.4, 0.8, 1.2; still from `at` + 1.5.
  - **Motion**: the approved ending's (`ending.ts`): the popcorn's fall is gravity from rest
    (0.72 s, 760 px, tilting 14 degrees to 0), a 26 px rebound over 0.3 s, the frame's give
    (7 px down at 0.18 of 0.36 s, -3, 1, 0, eased between), the couch pressed 7 px over 0.22 s;
    type rises 40 px (couch, `out`) or 34 px from 1.2 times its size (t and v, `snap`) over
    0.34 or 0.26 s, fading in with it: the lockup's type-in everywhere (idents, stings, the
    tag). Everything else is on the screens' curves (`EASE` in `core/motion.ts`): `glide`
    (0.16, 1, 0.3, 1) for entrances, `move` (0.65, 0, 0.35, 1) for a change of place, `dolly`
    (0.6, 0, 0.2, 1), `leave` (0.5, 0, 0.75, 0) for exits, `sine` (0.37, 0, 0.63, 1) for light
    and crossfades; never an overshoot. A half-frame shutter's motion blur on what the ending
    moves.
  - **Sound**: `src/audio/voices.ts`, one `house(kit, bus)` per piece and voices
    `voice(house, when, ...)` with a seed: `piano` (felt), `pad` (detuned soft saws; `padSweep`
    for the bus filter), `glass` (struck, or blooming with a longer `attack`), `bowed`, `sub`,
    `impact`, `pulse` (a felted heartbeat), `air`, `inhale` and `riser` (into a hit),
    `shimmer`, `ringOut` (a clip's end, below). The sonic logo (`sonic.ts`): `SONIC` (notes,
    offsets, chord), `BEAT` 0.8; `sonicLogo(house, first note)` (the full statement),
    `sonicSting(kit, bus, first note)` (a little fuller: the root on the piano, a sub),
    `sonicTag(kit, bus, first note, { gain })` (its popcorn lands half a beat before the first
    note, its fall breathing in 0.42 s before that, so its cue goes 1 s early; `gain` 0.5 is
    its level over a ducked bed) and `sonicChime(kit, bus, first note)`; `popcornBuild`,
    `popcornFall` and `popcornLanding` for the moment before.
- **The strip and short promos, sharp (2026-10-14, D41)**: rebuilt from the screens' design (its
  2.5 and 2.6) in the owner's sharp, flat direction (no rounded corners anywhere: square bars,
  markers and label blocks; no shadows or glows; red as structural blocks and bars) on the
  title's own logo and scenes (FEATURES.md "Strip and short promos" and "A title shown big").
  Where they leave the design: the strip shows the series' footage beside a flat panel (paper by
  day, night by night) rather than one backdrop under a gradient, so the scenes keep their
  colour and the type never sits on a moving picture; the panel's red bar sweeps on across the
  scenes before the tag, so the tag lands on a clean field and the strip's last seconds hold
  still; the short promo's hit is a red swipe wiping the title card in rather than a hard cut,
  and its airing sits in a red block at 58 px (44 in the design) to read beside a logo. A logo
  never sits on a box (the owner, 2026-10-14: "I don't want that", of The Simpsons' yellow logo
  on a night plate): one that does not read on the strip's paper puts the panel itself in night,
  and one too dark for night has its colours lifted once, hue kept (Dexter's dark red on night),
  greys and outlines as drawn. Cost, with the screens' probe (frames the sidecar captures;
  median ms of an advance and a JPEG capture on the M1, loaded by other work, so indicative)
  on the same real-media
  specs (the kit's 1080p footage and 4K backdrops), today's templates against these: the strip
  captures 343 of 400 frames instead of 400 (its last 2.3 s hold) at 73 and 99 ms instead of 66
  and 90 (bright, dark; the footage decodes each frame), about 25 and 34 s a clip instead of 26
  and 36; the short promo captures all 225 frames as before at 110 and 76 ms instead of 131 and
  129 (the old one's rays, halftones and shake cost more than a footage frame), about 25 and
  17 s instead of 30 and 29. So neither adds to the Pi's daily budget. Taking the logo plates
  away costs nothing (the same probe on the real-media specs, before and after): the bright
  promo with a logo captures 356 and 347 of 420 frames (The Simpsons on red, Dexter on paper)
  against 344, at 38 and 39 ms against 41 and 45, the bright strip 343 of 400 as before, the
  short promo all 225, at 38 and 52 ms against 38 and 53.
- **Every short clip's sound ends inside it (2026-10-14, the owner: "some jingles (the short
  ones) are too short and they have their sounds cut off at the end")**. A clip's sound used to
  ring on past its last frame: the owner's reel joined standalone renders, which cut every tail, and
  on air a bulletin slot's edge and the break's end cut it too (a break mixes its items, each
  ringing out under the next, but nothing runs past the last item or a slot). The last note of the
  sonic logo needs about two seconds to die away: at 4 s a sting's chord would have to fall 35 dB a
  second to be silent in time, at 5 s about 20, which is what a hall tail does by itself. So the
  stings are 5 to 10 s (their longest twice their shortest, so stings alone fill any gap from 5 s)
  and the short ident 5.2 s (its last note had 1.7 s, now 2.4); the sonic logo's notes, the 75 bpm
  timing and every picture are as they were. A statement is given the time its sound has to be over
  (`until`: the clip's end less 0.12 s): its voices' decays shorten to the room they have, and
  `ringOut` lets the whole mix, hall included, fall at a steady rate from just after the last note
  (6 to 40 dB over the room, 9 dB for the short ident, 12 for a 5 s sting) closing over the last 0.2
  s, so the chord dies away instead of being faded at the end (the old `fadeOut` stopped at -20 dB
  and left the rest to the cut). In a long sting the settled chord (pad, bowed swell, the root's
  piano and sub under the last note) is held through the hold and released later, so the sound
  reaches within about two seconds of the end instead of leaving the lockup in silence (first
  version: the 10 s sting's loudness fell under -47 LUFS from 7.8 s; now it holds about -20 LUFS to
  8.1 s, falls through -35 at 9 s and ends at -55). The channel tag does the same at the end of
  its clip (the promo end card, the strip, the short promo, the evergreen end card); up next's logo moved one beat earlier
  so its D rings out under the type lifting away (it used to ring on into the program, which the
  break's mix cut), the lineup's open fifth comes after the last row's note and rings out under the
  rows lifting away, the squeeze's push air settles before its last frame; `ringOn` (the hall
  ringing past a clip's end) is gone. A title's bed has its own fade (1.2 to 1.6 s, linear to zero
  at the end). Because the `ident` template's version covers both cuts, the new short ident bumps it
  and the four full idents render again when next needed. Measured, standalone renders (normalized
  to -18 LUFS; dBFS RMS of the last 50 ms and of the last 250 ms, then when the level last rose
  above -45 dBFS): stings of 5, 6 and 10 s -166 / -63 (4.56 s), -173 / -64 (5.32 s) and -173 / -65
  (9.39 s); short ident -169 / -65 (4.56 s); up next -75 / -46 over a bed, -195 / -59 without;
  lineup -171 / -56 (11.64 s); squeeze into up next -61 / -56, into the short ident -123 (its sound
  ends 0.8 s after the handover starts); end cards and promos -72 to -162 / -43 to -70 (a bed's fade
  is linear to zero); before, up to -19 dBFS (up next without a bed), -26 (lineup), -31 to -41
  (promos) and -36 to -43 (stings). The level only falls through the last second (steepest 100 ms
  drop under 4 dB), and the break seams in the sample break (a squeeze over a Pulp Fiction shot into
  the short ident, an Iron Man promo, a sting, a Simpsons strip, a sting, the lineup, up next) are
  all under -45 dBFS before the cut except the promo body into its end card, which is one continuous
  sound. Cost, with the screens' probe (frames captured of the clip, median ms of an advance and a
  JPEG capture on the loaded M1): the short ident 53 of 130 frames (60 of 113 before), a 5 s sting
  62 of 125 and a 10 s one 62 of 250 (the hold is static, so a longer hold costs nothing), at 74, 57
  and 50 ms. The concepts tools `audio.mjs` (the endings), `break.mjs` (a break as it airs, with a
  seam report) and `mixcheck.mjs` are local (excluded from the repository); the unit and golden
  tests hold the endings (`audio.spec.ts`) and every break length from the shortest to twenty
  minutes (`TestComposeEveryBreakLength`).
- **The trailer (D43, 2026-10-14)**: the long promo the owner asked for after watching the reel
  ("promos can be LONGER (can be like a minute, minute and a half... and can include some part of
  the movie - or its trailer? - with sounds and stuff)"). Official trailers are not in the library
  and are not downloaded, so the channel cuts its own from the title itself: template `trailer`
  (version 1, FEATURES.md "Trailer"), `TrailerSpec` in the spec, whose scenes (each its 25 fps
  frames, its own sound and the times its lines are spoken) the composer picks from the title's
  subtitles, picture cuts and audio. What the template decides: the body is 60 to 90 s in whole
  frames (about 70 to 80 s from 45 to 55 s of scenes; scenes beyond 88 s go, those with the least
  speech first, and a thin trailer gives the montage and the cards the time); the cuts follow the
  soundtrack's beats where it leads and never touch a line (every scene plays whole lines, with
  fades short of every word, and the picture alone is trimmed for a J- or an L-cut); the end card
  is the promo's own (`buildPromo`, built on the trailer's timeline), so the airing, the tag and
  the ring-out are one implementation and the body stays timeless; the scenes' sound is levelled
  by its lines to -22 LUFS and the soundtrack ridden 13 dB under speech by sample-accurate
  automation; and a take holds an image only around its own time (`Timeline.footage`'s window),
  because the 40 or more takes of a 75 s body would otherwise each be decoded again on every
  frame. Measured on the library's Iron Man 3 (eleven hand-picked scenes, 1080p frames, the
  soundtrack, cinematic): 73.6 s, 1839 frames of which 1815 are captured (the scenes and the
  montage never hold still), a median 46.9 ms for an advance and a JPEG capture on the M1 loaded by
  other work (the promo body: 401 frames at 50.8 ms, so the same cost a frame and four and a half
  times the frames), 85 s there. At the Pi's 2.4 captures a second that is about 12.6 min a
  trailer (the promo body about 3 min) plus its sound (about 13 s on the Mac, one to two minutes
  there) and the encode; the body is rendered once per title, in the quiet hours, so a library of
  25 titles is five to six hours of render over a night or two, and the frame loop's and the
  sound's timeouts must allow about 13 minutes of frames. The page's memory: the renderer peaked
  between 750 MB and 1.05 GB on the Mac over six renders (the whole browser 1.0 to 1.3 GB; it varies with when the
  engine collects), in the sound render, and between 600 and 700 MB in the frame loop, settling to
  200-450 MB (the promo body in the same harness: 563 and 380 MB), so a trailer needs one and a
  half to two times a promo's memory; the sound render is the peak (a 74 s render of the scenes,
  the soundtrack and every house voice) and could run in a page of its own if it ever crowds the
  sidecar's 1 GB, where a promo's renderer is 280 MB on the Pi (a trailer's would be 450 to 600 MB
  by the same ratio: inside the container, and the closest of any clip).

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

**Settled while building it (Phase 5, backend and core)**; the full reference is FEATURES.md
"couchtv" and "Shared client core".
- **Guests live in the database**, not the hub: `couchtv_guests` (a random token, hashed, the
  name and the link they came through), so a restart keeps them. The cookie `couchverse_tv` is
  scoped to `/api/v1/tv` for 30 days; a native guest gets the token with `delivery=body` and
  sends `X-Couch-Tv-Token`. A guest tunes in with their own operation, `GET /tv/tune`
  (`tuneCouchTvGuest`), under the cookie's path; `tuneCouchTv` stays signed-in only. Rotating
  the link (`adminRotateCouchTvLink`, the code shown once) or turning it off
  (`adminSetCouchTvLink`) deletes every guest and closes their sockets, and the channel grant
  middleware asks `Guests.Allows` for a grant naming a guest (kept 30 s), so their stream ends
  too. `adminUpdateCouchTv` keeps the stored link. The guide stays signed-in only: a guest sees
  now, next and later from tuning in.
- **The room**: viewers are `user-<id>` or `guest-<id>`, each counted once however many
  devices; the presence names the first 24 to tune in; three reactions a second per viewer;
  `schedule_changed` follows a plan run, an admin's regenerate and a prepare run's swaps
  (`Scheduler.Changed`, `Preparer.Changed`; the admin's EPG edits should call it too), gathered
  over 2 s.
- **Reports** count a stretch when the picture moved on about as much as the clock (reports at
  most 30 s apart); they are mapped onto what aired every 20 s, off the socket's read path.
  Breaks and fillers count as channel time with a null title; a program watched to within 30 s
  of where it left the air is saved as finished. `TvStatsRecorder` (stretches with the airing,
  its kind, fill and time of day; reactions; the largest audience) is ranks', below.
- **The contract**: tuning in names the stream's `languages` (the menus' order) and lists now
  and next as plain listings, absent rather than null; the guide names the broadcast `days`
  its stretch overlaps, which the core asks for from a day back to a week ahead.
- **The core**: the clock is the server's Unix time minus the shell's monotonic time, from the
  tune-in's midpoint and refreshed by the socket's `hello`; the hold acts only on a report that
  carries a fresh program date, from a playing, unbuffered player 3 s after a load or a seek.
  The overlays: the Now caption for a program's first 8 s, the Next caption for 12 s from two
  minutes before it leaves the air, the bug over programs only (since 2026-10-12 the bug stays
  up through a program and the captions come with the controls, see D14). A failing player is tuned in
  again (a fresh grant) and reloaded three times in a row before the view fails with
  `couchtv_playback_failed`. The new code took the wasm core from about 395 KB to 411 KB
  gzipped, so its budget went from 400 KB to 440 KB.

**Ranks, as built (Phase 6)**; the full reference is FEATURES.md "ranks".
- **The hook**: `TvStatsRecorder` is `*ranks.Store`, wired in `internal/app`. Its calls take
  plain values (`RecordTvViewing(userID, airingID, kind, at, seconds)`), since a `couchtv` type
  in the signature would make `ranks` import `couchtv`; ranks reads what an airing was (fill,
  series, time on air) from `couchtv_airings` by join. The room reads the config through
  `HubDeps.Settings` and states `at` in the channel's time zone, so "after two at night" is the
  channel's night whatever zone the server runs in.
- **Storage** (migration `0029_ranks_couchtv`): `user_tv_viewing`, seconds per member, airing,
  day and hour on the channel's clock, upserted by the 20 s accrual (one statement per
  stretch, nothing on the socket's read path); programs and breaks only, the standby card and
  the slate earn nothing. Reactions and the largest audience are `user_counters`
  (`tv_reactions`, `tv_audience_max`). Like the couch counters it records while rankings are off.
- **XP**: `RankConfig.Rates.tvMinute`, default 1 (video is 2): the channel needs no choosing and
  never stops, so a set left on earns all day; its minutes include the breaks; and its programs
  already pay the movie and episode bonus through the progress the room saves. Rates are whole
  XP, so "a little less" than video is half. The profile's breakdown shows it as `tv`. A config
  saved before the rate existed keeps its default.
- **Achievements** (category `tv`, absent while `couchTvEnabled` is off; a program is seen at
  half its time on air, at most 30 minutes, and watched whole at 80%):

  | Code | Name (en / cs) | Medal | Rule |
  |---|---|---|---|
  | `tv_prime_time_5` | Prime-Time Regular / Večerní štamgast | silver | the prime-time movie (`movie`, `premiere`, `saga`) seen on 5 days |
  | `tv_night_owl` | Graveyard Shift / Noční směna | bronze | 15 minutes of the channel between 02:00 and 06:00 on a day |
  | `tv_morning_5` | Morning Cartoons / Kreslené ke snídani | bronze | 15 minutes between 06:00 and 09:00 on 5 days |
  | `tv_full_house` | Standing Room Only / Plný obývák | silver | watched with 3 others (the presence count, guests included) |
  | `tv_reactions_100` | Crowd Reactor / Hlas publika | bronze | 100 reactions |
  | `tv_strip_5` | Appointment TV / Den co den | silver | one strip seen on 5 days in a row |
  | `tv_marathon` | Weekend Marathoner / Víkendový maratonec | gold | every marathon episode of a weekend watched whole |
  | `tv_premiere` | Premiere Night / Premiérový večer | bronze | a premiere watched whole |

  Three of the owner's working names were taken (Night Owl and Full House by badges,
  Marathoner by a rank tier beside the Marathon badge), so those got their own. A weekend is the channel's marathon airings
  without a two-day gap between them, planned ones included, so it completes with its last
  episode whatever count the template gives the slot.

### 6.10 Bulletins (data)
The owner wants a weather overview of Czechia and the day's top headlines in the breaks. The
data layer is built (2026-10-13; FEATURES.md "couchtv", Bulletins) and the bulletins air in
slots of the breaks ("On air" below), in their designed templates (DESIGN 2.10 and 2.11, made
sharp and a minute and half a minute long, designed for their length, 2026-10-14). The `couchtv_bulletins` job (app) fetches the news
every 20 minutes and the weather hourly from 05:30 to 00:30 in the channel's zone and at
startup, politely (a User-Agent naming the app, 10 s timeouts, two retries, conditional GET),
and keeps the last good bulletin of each kind in `couchtv_bulletins`, served by
`adminGetCouchTvBulletins`; config `bulletins` (`enabled`, `cities`).
- **Sources and terms**:
  - Weather: Open-Meteo `/v1/forecast`, one request for every city (free for non-commercial
    use, CC BY 4.0, credit "Weather data by Open-Meteo.com"). A day's symbol is its daytime's
    (09-18), since the daily code is the day's worst hour.
  - Warnings: ČHMÚ's open data, the newest SIVS CAP message by the directory's date (free
    public-sector information; credit "Source: ČHMÚ" on air (D39), no logo, no implied
    endorsement).
  - Map: geoBoundaries' CZE ADM0 outline (ČÚZK, CC BY 4.0, 643 points), embedded; credit
    "Borders: ČÚZK / geoBoundaries (CC BY 4.0)" on air (D39).
  - News: the RSS of ČT24 and its editors' selection, iROZHLAS, Seznam Zprávy, Novinky and
    iDNES (shown), CNN Prima and Deník (ranking only; Deník allows personal use only) and
    ČT24's unofficial trending list (optional). ČTK and Economia (Aktuálně, HN) are left out:
    ČTK forbids passing its texts on, even inside a private network, and Economia reserves
    automated analysis (§ 39c).
  - Headlines are stored whole, never shortened or reworded (Český rozhlas), text only, each
    with its outlet's name as plain text (never a logo), and kept a day at most (iROZHLAS's
    limit without a contract).
- **Ranking**: the research's proxy: the last 12 hours, sport, culture, lifestyle, opinion and
  shows left out, average-link clusters of word stems with one item per outlet, scored by
  outlets (+1.5 editors' selection, +1 trending, halving over a day), the headline from ČT24,
  then iROZHLAS, Seznam Zprávy, Novinky, iDNES. Items are remembered across runs (in memory):
  Novinky's and iDNES's feeds reach back only two or three hours, so on 13 Oct at 17:40 a fresh
  process already missed outlets that had carried the top stories an hour or two before.
**On air (2026-10-13; FEATURES.md "couchtv", D36-D40).** The slice was open on five points;
how each was settled:
- **When to render (D36, D38).** Breaks are composed a day ahead and rendered at night, so a
  bulletin cannot be part of a composed break. A break that carries one keeps a slot of the
  bulletin's length instead, after the promos and the lineup, before up next, holding a
  fallback of exactly that length: promos of titles the break does not promote yet, closed
  with stings, renderable ahead like any part. The break is assembled in sections (the clips
  around its slots and each slot's fallback, a package each) and its sound mixed per section:
  what rings past a section's end is cut, faded out over 20 ms at a slot's edge, the break
  around its slots normalized as one and each slot on its own at the break's loudness in
  whole LU. The bulletin binds late: when the live stream puts the break on air (pins its
  feed), a slot that airs whole takes the newest rendered bulletin of its kind still valid
  when the slot ends, else its fallback, and every slot edge is a discontinuity with a
  program date, the bulletin's audio cut at its edge with `?keep=` like any item's. The
  clip carries its sound at every whole LU a break is mixed at (-23 to -16), so it airs as
  loud as the fallback it replaces. The bulletins job asks the sidecar for the clip on its
  kind's last run in time for the next slot (the slot's break on air 15 minutes later at the
  latest), so a bulletin renders once and only when it is the one that airs: the news,
  fetched every 20 minutes, once an hour. Its own job type (`couchtv_bulletin_render`) puts
  it ahead of every other clip and lets it run while a transcode does, outside the quiet
  hours. Nothing renders while the channel is off.
- **Language (D39).** The owner's decision: English chrome everywhere the data allows (labels,
  weekdays, times, ČHMÚ's English event and kraj names, Prague by its English name, the
  credits), the headlines Czech, whole and unaltered, each with its outlet's name.
- **Which breaks, how long, the credits (D37, D39).** The news (54.4 s) in the breaks that end
  on the hour every two hours from 06:00 to midnight (some of the breaks with the lineup; the
  owner chose every two hours over every hour), the weather (38.4 s) in the
  first break ending within the hour after 07:00, 12:00 and 17:00 and in the break before the
  evening movie, both where the break keeps the squeeze, the short ident, a short promo and
  the shortest up next beside them: 30.1 s, so a break needs 84.5 s for a news slot, 68.5 s
  for a weather slot and 120.5 s for both (the later, the weather, goes first). Measured over
  the week the Pi had planned on 2026-10-14 (9 days, 319 breaks): the schedule asks for 32
  news and 36 weather slots, and at the old lengths (20 s, 15 s) every one had its room; at
  the new ones 31 of the news slots do (the one lost, 14:00 on a Friday, is a 78 s break)
  and all of the weather's. Over the test library's planned week (`TestBulletinSlotsOverAWeek`)
  every one does (22 and 28). So the rules stay as they were: dropping the short promo from
  what a break keeps would win back that one slot and make a break of a squeeze, an ident,
  the news and up next. What the schedule asks for is the limit (few breaks end exactly on an
  even hour: about 3.5 news slots a day), not the room. On screen: "Weather data by Open-Meteo.com" always,
  "Source: ČHMÚ" with the warnings (or "No weather warnings"), "Borders: ČÚZK / geoBoundaries
  (CC BY 4.0)" with the map, the outlet's name with every headline and "Headlines from the
  outlets’ RSS feeds, as of 17:40".
- **Nights.** A slot is only made where a bulletin fetched on the grid at least 15 minutes
  before its break goes on air is still valid when the break ends: the 00:00 news airs the
  23:30 fetch, the 06:00 news needs a break on air from 05:45, and no news is scheduled from
  01:00 to 05:00. A forecast airs until midnight at the latest (D40), since its days are
  named "Today" and "Tomorrow" from the day it was fetched; its slots are all by day. Still
  open: an evening forecast's "tonight" from hourly temperatures.
- **The map (D40).** Czechia's outline is a module of the broadcast package generated from the
  server's geojson (`npm run outline`), with a projection (`czechiaMap`) that places the
  cities; the spec carries their positions. The kraje the warnings fill are a module like it
  (2026-10-14: geoBoundaries CZE ADM1, same licence and credit, `couchtv/kraje.geojson`
  simplified by `scripts/kraje.mjs` with each border between two kraje simplified once, so
  neighbours share it; `src/generated/kraje.ts`), keyed by the ISO 3166-2 codes the spec
  names. The weather icons are the brand's own, drawn in code from the WMO codes and daylight
  (`weather-icons.ts`).
- Development and e2e servers fetch too (a dozen requests at startup); switch
  `bulletins.enabled` off where that is unwanted. They render bulletins only with a sidecar
  and the channel on.

**Render budget (the Pi).** A day has at most 10 news slots and 4 weather slots, 14 renders
(the news every hour would be 23). A bulletin that moves in every frame renders at the Pi's
measured 2-2.5 fps; these hold still between their moves and a held frame is not captured
again. The designed bulletins of 2026-10-14 (20 s and 15 s) captured 213 of a five-story news'
500 frames and 154 of the weather's 375, about 25 minutes a day for the 14. The longer ones
(the news 54.4 s, 1360 frames; the weather 38.4 s, 960; designed for their length, see
FEATURES.md, Broadcast package) capture **504** of a five-story news' frames (376 with three
stories, 475 with the five longest headlines of the recorded feeds) and **225-335** of the
weather's (225 with two cities and no warnings, 242 on a quiet day, 265 with three cities and
a warning, 335 with twelve cities and four warnings), at 47-53 ms a capture on the loaded M1
(`concepts/variants/screens/tools/cost.mjs`). At the Pi's 2.4 captures a second that is
about 210 s a news and 95-140 s a weather, plus about 20 s each to load the page and encode
the eight slot loudnesses and the sound's render, which grew with the clip (13 s for the
news and 9 s for the weather on the M1 under load, the chords built when their turn comes
rather than up front; the Pi is several times slower, say 50 and 35 s): **about 4.8
minutes a news and 2.8-3.3 a weather**, one render before each slot, well inside the 15
minutes a slot leaves it. At D37's schedule met in full (10 news and 4 weather slots a day)
that is **about an hour a day**, all of it in the daytime; over the Pi's real week, where
breaks end on an even hour only about 3.5 times a day, **about half an hour**. Estimates for
the Pi, measured on the M1; the first slots on air will say. Through the sidecar's own pipeline
(`TestCouchTvRender` on the loaded M1: the page's load, the sound, three loudness renditions
and the HLS packaging included) a three-story news took 48 s and a two-city weather 83 s (the
first render of the run, with Chromium's start), so the estimate stands on the safe side.

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

**As built (Phase 5)**; the full reference is FEATURES.md "couchtv" (Web viewer).
- **`/tv` is public** (a guest watches signed out) and outside the app's shell (no top nav, full
  bleed); a member with the flag off gets a 404, a guest learns it from tuning in. The page
  tunes in on mount and out on leaving; `VideoPlayer`'s `live` mode keeps the player's chrome
  (volume, fullscreen, picture in picture, the subtitle style, the TV remote) and swaps the
  seek bar for a progress line, play/pause for the channel's controls (presence and the
  reaction button on the left, Watch from the start and the Guide on the right) and the
  playback events for the channel's audio and subtitle choices. Live never pauses: a picture
  the browser stopped (a blocked autoplay) shows one Play button that resumes at the live
  edge.
- **The overlays** follow the core's cues on the picture's clock. The corner bug stays up
  through a program; the Now and Next captions show with the controls (D14; until 2026-10-12
  the bug came with the controls and the captions for a few seconds each). Since D33 the bug is
  the broadcast's own, drawn where a break burns its own into the picture, so a cut never moves
  it, and the captions are in the brand's look (a red label, the title in 800 paper, the meta in
  paper 72 %), above the bar and aside for the quick reactions, with the live player's subtitles
  lifted over them. Sharp and flat (D42): the marks are square and nothing casts a shadow; the
  bug and the captions sit on night corner shades, as the broadcast's bug and tag do. The
  lockup comes from the package's brand module, in paths (no font to load), so pages with a
  lockup (home's card) never load the templates or the audio engine.
- **Reactions** reuse couch's emoji button and bundled picker, with five quick picks for one
  press (shown with focus as well as hover, for remotes); everyone's reactions rise up the
  left side with the sender's name.
- **The guide** is a row per broadcast day on one time axis (12 rem an hour, scrolling sideways
  to now on open); the first row is the broadcast day on air ("Today", which runs past
  midnight), the next "Tomorrow", then weekdays. The program on air is the TV's autofocus.
- **The home card** waits for the core's answer and takes no room until then; during a break it
  leads with what is next.
- **Guests**: the join page takes a name of up to 32 characters and goes to `/tv`; a member
  opening it goes straight there. A link turned off or rotated ends a guest's channel at once
  (the room's socket closes, the next tune-in answers `no_channel_guest`) and `/tv` asks them
  to sign in. The admin's panel confirms a new link (it ends the old one and its guests) but
  not the first, which ends nothing.
- **E2E content**: `e2e/setup.sql` turns the flag on and airs the series' first episode every
  32 s (to its credits at 24 s, the slate in the breaks) as locked admin rows, so the admin
  spec's replan airs around them; the e2e server makes the episode's package with the real
  job before it reports ready, and its clips have a keyframe every 2 s so the copied source
  fits the target duration.
- **Problem codes** with messages: `couchtv_playback_failed`, `invalid_name`,
  `no_channel_guest`, `no_channel_link` and `feature_disabled` (a guest of a channel that is
  off), worded on Apple and Android too.

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

### Spike results: S2 (render pipeline), 2026-10-12, local

**What ran** (an M1 Pro, 8 cores; the pipeline of 6.7 as built). Speeds are whole jobs (media
extraction, load, every frame captured and encoded, the sound rendered and encoded, the
package written), 1080p25, no static frame in any of these clips:

| Run | Clip (frames) | fps |
|---|---|---|
| `TestCouchTvRender`, macOS, Playwright's headless shell, no GPU flags | squeeze over a program's frames (275) | 8.8 |
| | short ident (125), sting (52) | 10.3, 9.7 |
| `scripts/couchtv-s2.sql` + `couchverse render`, macOS, same browser | squeeze over a file's frames (275) | 7.8 |
| | short ident (125), sting 6 s (150), up next (200) | 9.0, 10.0, 9.4 |
| | short promo (225, twice), lineup (300) | 5.6, 6.0, 8.8 |
| the `render` image (Debian trixie `chromium`, software GL), Docker Desktop arm64, `--memory 1g --cpus 2` | standby (300), full ident (288), short ident (125) | 8.4, 8.7, 7.7 |

- **The 60 s break** (`couchtv-s2.sql`: squeeze, short ident, short promo, lineup, 6 s sting,
  short promo, up next; 1500 frames): rendered in 197 s (7.6 fps on average, with the backend
  test suite running alongside), assembled in 9 s (its 60 s of sound rendered in the page,
  the clips joined by stream copy, AAC encoded, segmented); exactly 1500 frames, -20.0 LUFS
  integrated as the spec asked, `validate-hls` 0 errors (one warning: no I-frame playlist).
- **The image end to end**: the sidecar container against an app on a throwaway database
  published its versions, the app composed the evergreen break and standby card, the
  container rendered them and assembled the evergreen break (413 frames, 3.6 s); peak
  container memory **833 MiB** of the 1 GiB cap (Chromium, ffmpeg and the Go process).
- **Encoding** is never the bottleneck: capture and paint are (two CDP round trips per
  frame). Static holds were not exercised; today's templates keep motion in every frame
  (6.8).

**To measure on the Pi** (the exit criterion; nothing here touches the production database or
caches: a throwaway database, a scratch `DATA_DIR` with the movies mounted read-only):

```sh
# On the Mac, from this branch: both images for the Pi
docker buildx build --platform linux/arm64 -t couchverse:s2 --load .
docker buildx build --platform linux/arm64 --target render -t couchverse-render:s2 --load .
docker save couchverse:s2 couchverse-render:s2 | gzip > /tmp/couchverse-s2.tar.gz
scp /tmp/couchverse-s2.tar.gz scripts/couchtv-s2.sql pi:/tmp/

# On the Pi, in the compose directory (its .env sets DB_PASSWORD, MEDIA_ROOT, PUID, PGID)
set -a; . ./.env; set +a
gunzip -c /tmp/couchverse-s2.tar.gz | docker load
NET=$(docker inspect -f '{{range $k, $v := .NetworkSettings.Networks}}{{$k}}{{end}}' $(docker compose ps -q db))
S2=$MEDIA_ROOT/../couchtv-s2
mkdir -p $S2/media/movies $S2/media/series && sudo chown -R ${PUID:-1000}:${PGID:-1000} $S2
RUN="docker run --network $NET --user ${PUID:-1000}:${PGID:-1000} -e DATA_DIR=/s2
  -e DATABASE_URL=postgres://couchverse:$DB_PASSWORD@db:5432/couchtv_s2
  -v $S2:/s2 -v $MEDIA_ROOT/media/movies:/s2/media/movies:ro"
docker compose exec db psql -U couchverse -c 'CREATE DATABASE couchtv_s2'
# the new app migrates the throwaway database, then goes
$RUN -d --name s2-app -e ADMIN_USERNAME=admin -e ADMIN_PASSWORD=s2 couchverse:s2
until docker logs s2-app 2>&1 | grep -q listening; do sleep 1; done; docker rm -f s2-app
# a 60 s break cut from one movie (its path under media/movies; 20 minutes in)
docker compose exec -T db psql -U couchverse -d couchtv_s2 \
  -v file='Iron Man (2008)/Iron Man (2008).mkv' -v start_ms=1200000 < /tmp/couchtv-s2.sql
# the sidecar as the compose service runs it: GPU, V4L2 encoder, 1 GB, two CPUs
$RUN -d --name s2-render --memory 1g --cpus 2 --device /dev/dri --device /dev/video11 \
  --group-add video --group-add "$(getent group render | cut -d: -f3)" couchverse-render:s2
until docker logs s2-render 2>&1 | grep -qE 'assembled a break|ERROR'; do
  docker stats --no-stream --format '{{.MemUsage}}' s2-render >> /tmp/s2-mem.txt; sleep 2; done
docker logs s2-render 2>&1 | grep -E 'rendered a clip|assembled|WARN|ERROR'   # fps per clip
sort -h /tmp/s2-mem.txt | tail -1                                             # peak memory
docker run --rm -v $S2:/s2 --entrypoint couchverse couchverse:s2 \
  validate-hls /s2/couchtv/breaks/00000000-0000-4000-8000-00000000e2ff/master.m3u8
# clean up
docker rm -f s2-render
docker compose exec db psql -U couchverse -c 'DROP DATABASE couchtv_s2 WITH (FORCE)'
sudo rm -rf $S2; rm -f /tmp/couchverse-s2.tar.gz /tmp/couchtv-s2.sql /tmp/s2-mem.txt
docker image rm couchverse:s2 couchverse-render:s2
```

Record per clip fps (the plan budgets ~2.5 fps with the GPU), the break's assembly time, the
peak memory (under 1 GiB) and the validator's verdict here. The script's media file row is
not probed, so pick an SDR source (an HDR one would be cut without tone mapping), and one
whose source is still on disk (a file removed after transcoding cannot be cut from yet).

**Measured on the Pi 4** (2026-10-12, the commands above with the scratch data in the home
folder instead of next to the media; Iron Man, 720p H.264, 20 minutes in; production idle):

| Clip (frames) | fps | Clip (frames) | fps |
|---|---|---|---|
| squeeze over the program's frames (275) | 0.81 | short ident (125) | 2.30 |
| short promo with footage (225, twice) | 0.84, 0.83 | lineup (300) | 2.34 |
| up next (200) | 1.86 | sting 6 s (150) | 2.36 |

The 60 s break (1500 frames) took **23 minutes** to render and 1 m 45 s to assemble;
`validate-hls` 0 errors (the I-frame warning); peak container memory **907 MiB** of the 1 GiB
cap; host load average up to 14 while it ran. Graphics-only clips meet the benchmark
(~2.3 fps); clips that draw video frames in the page (the squeeze's program, promo footage)
run at a third of that, because Chromium decodes a full-size JPEG per frame on the Pi's CPU.
At these speeds a day of breaks costs about 5-6 hours of rendering, mostly squeezes. (The short
promo draws no footage, the template takes none: what makes it slow is its film grain, see
below.) The follow-up is the next section.

**Render speedups** (2026-10-12, after the Pi measurement; 6.7 and FEATURES.md "couchtv"):
- **One paint per frame**: the renderer steps with `advance(t)`, which sets the frame up
  without the double `requestAnimationFrame` wait; `captureScreenshot` forces a fresh frame of
  its own, so every frame used to be drawn twice. The captures are byte-identical for the
  short ident, the sting, up next and a grit promo body, and within 55 dB (raster noise)
  elsewhere.
- **The squeeze's program is composited by ffmpeg**: the page renders the squeeze keyed, the
  encoder decodes the program from its source and lays it over the picture at each frame's
  `keyRect` through a matte the sidecar draws; Chromium decodes no program frame and nothing
  is extracted for it. Against the in-page path: 38 dB on average, 40-41 dB through the hold
  and 32 dB at worst in the squeeze's motion, where the program is placed on even pixels and
  scaled once from the source instead of upscaled to 1080p, saved as JPEG and scaled again: it
  comes out sharper, nearer the program as it aired. The panel's corners and the cursor
  handover look as before; the frame's 1.5 px rim over the panel edge is covered by the
  program. Composited in 4:2:0 without a full-frame canvas it costs half of the 4:4:4 graph
  tried first.
- **Static frames**: `isStatic` skips the holds between a whole-piece animation's keyframes, so
  the idents' and stings' held lockups are captured once (26 of the short ident's 113 frames,
  16 of a 6 s sting's 150; since D34 60 of the short ident's 113 and 63 of a 5 s sting's 125:
  the couch alone, the logo before its type-in and the held lockup). Everything else keeps
  moving by design (slow pushes, grain; the bug's LIVE marker no longer breathes), so freezing
  it was not done.
- **The short promo's rays** (a 2600 px masked conic gradient, rotating and scaling) were
  painted again at every frame's scale; `will-change` keeps their first raster (52 dB, the
  soft rays look the same). The hint everywhere scale animates was tried and dropped: it
  softens the lockup's edges.
- Promo footage frames are served `no-store` (Chromium's cache kept every one), and an empty
  artwork URL (a title without a backdrop) no longer counts as missing media (the
  `missing=[]` warning).
- Tried and dropped: `--disable-frame-rate-limit` (light clips 2-3x faster, but the
  compositor spins and heavy ones ran 2.5-5x slower on two cores); footage extracted at the
  source's resolution (Chromium scales a lone image layer bilinearly, softer than ffmpeg's
  bicubic upscale; promo bodies render once per title).

**Measured locally, before and after**, the S2 clips (whole render jobs, a 720p `testsrc2`
program with temporal noise as the squeeze's and the footage's source, no artwork) on two
Pi-like proxies: the whole render (Chromium, ffmpeg, the Go process) on the M1 Pro's two
efficiency cores (`taskpolicy -c background`, cores of about a Cortex-A72's speed; two runs,
averaged), and on the performance cores with Chromium's main thread throttled 4x
(`Emulation.setCPUThrottlingRate`; two runs, within 2 % of each other). Playwright's headless
shell, compositing in software (fps):

| Clip (frames) | 2 E-cores before | after | | 4x throttle before | after | |
|---|---|---|---|---|---|---|
| squeeze over the program (265) | 1.18 | 1.45 | 1.2x | 7.7 | 13.3 | 1.7x |
| short ident (113) | 1.10 | 2.10 | 1.9x | 9.9 | 17.9 | 1.8x |
| short promo (225) | 0.62 | 0.94 | 1.5x | 5.9 | 7.7 | 1.3x |
| lineup (300) | 1.18 | 1.50 | 1.3x | 9.2 | 13.7 | 1.5x |
| sting 6 s (150) | 1.29 | 2.02 | 1.6x | 10.7 | 18.2 | 1.7x |
| up next (200) | 0.97 | 1.42 | 1.5x | 9.3 | 13.5 | 1.5x |
| promo body with footage (415) | 0.93 | 1.05 | 1.1x | 7.0 | 9.0 | 1.3x |
| **the S2 break** (1500, from the rows) | 27.5 min | 19.1 min | 1.44x | 193 s | 131 s | 1.47x |

The proxies have no GPU, so graphics cost more there than on the Pi (the short ident ran at
1.1 fps on the efficiency cores against the Pi's 2.3), and the Pi's in-page video penalty does
not show (before, the squeeze ran as fast as the ident here, at a third of it on the Pi). On the
efficiency cores the keyed squeeze's page alone runs at 2.6 fps and the encoder alone (the
program decoded, composited and encoded) at 6.5-8.8 fps; sharing two cores they make 1.45. On
the Pi the squeeze should therefore gain at least what the idents gain (it is now a graphics
clip plus the encoding it already had), while the short promo stays slow because of its grain
(below). Peak resident memory of the render's process tree (macOS, which counts shared pages
per process) was the same or lower after: 0.82-0.86 GiB against 0.86-1.22 GiB on the
efficiency cores, 1.50-1.57 against 1.58-1.59 GiB on the performance cores; on the Pi the
squeeze no longer writes and reads its 288 extracted frames either. Unthrottled on the M1 Pro,
`TestCouchTvRender` now renders its squeeze at 14.4 fps, the short ident and the sting at 19.3
(8.8, 10.3 and 9.7 in the first S2 table), the composited panel 37.6 dB from its program.

**Still open** (owner decisions, not done here):
- **Grain**: a grain layer draws new noise every frame (5 % on the short promo, up to 13 % in
  grit promos), so its JPEG captures are 0.5-0.6 MB and libx264 at CRF 18 makes 44 Mb/s of the
  short promo, encoding it at a quarter of the speed it manages at 20 Mb/s (2 against 8 fps on
  two efficiency cores, the encoder alone). On the Pi the encoder shares the sidecar's two CPUs
  with Chromium, so this is most of the short promo's 0.84 fps, and a 44 Mb/s stretch is heavy
  for the live stream too. A rate cap keeps the encoder fast but smooths the grain away (at 20
  Mb/s already); a grain that changes every other frame would let x264 predict half of
  them. Either changes the look.
- The squeeze's frame rim (above).

**To re-measure on the Pi**: the commands above, unchanged (`scripts/couchtv-s2.sql` now has
the current brand's lengths: the ident squeeze of 265 frames, the short ident of 113, up next
of 222 to keep the break at 1500 frames). Record the per-clip fps from the `rendered a clip`
lines, the break's render and assembly time, the peak memory and the validator's verdict here.

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
Built (2026-10-12): the sidecar, clip cache, compose and assemble jobs, render health and
pruning (6.5 "Settled in Phase 3", 6.7 "As built", S2 results); `couchtv_analyze` fills
`couchtv_titles` separately. The Pi measurement (S2's commands) is still to run.

### Phase 4: The live stream
Channel grants, playlist builder (video, audio, subtitles), `couchtv_prepare`, fallbacks,
guest link. **Exit**: a 24 h soak locally and on the Pi with no gaps; validator clean.

**Built** (2026-10-12; 6.6 "Settled while building it"): everything but the guest link, which
moves to Phase 5 with the guest cookie, presence and reactions. `make couchtv-live` (M1 Pro)
readies three generated programs with the real jobs (a copied 720p source with subtitles; HDR10
HEVC in Czech and English from its rung; a 1080p source with 10.5 s keyframes from its 1080p
rung, cut at its credits), airs them with an assembled break, two breaks that are not ready
(20 s: the evergreen break, then the standby card looped; 8 s: the evergreen break cut short)
and a 5 s hole (the standby card), with a 4 s stand-in standby card so both loop it, looped
around now, and plays it for 130 s (after the reconciliation with the render sidecar):

| Player | Stalls | Item change vs program date (picture / sound) | Subtitles | Notes |
|---|---|---|---|---|
| hls.js, Chromium (4 viewers) | 0 | 0 of 1559 picture and 1559 sound samples off; -0.02..0.24 s / 0.09..0.33 s after it | 92/92 cue samples, none out of place | the program-date pair stays within 0.03 s median, 0.08 s max; the live-edge pair drifts (S1) |
| AVFoundation (macOS) | 0 | 7 boundaries; the program date falls 1.19 s behind in 129 s, 0.33 s of it the audio switch | 2 + 2 options, selection works | see below |
| `validate-hls` | - | 0 errors at 4 boundaries (evergreen to standby, standby to program, program to evergreen, evergreen to standby) | - | the `AVERAGE-BANDWIDTH` warning of S1 |

Runs while the host was loaded (load average 15-20 on 8 cores, an image build beside it) saw
every Chromium viewer wait 0.3-0.4 s at the same wall-clock instant at different points of the
stream, and AVFoundation none; a stall that hits every player at once is the host's.

What the run found and how it was settled:
- **A hole aired its slate twice.** The airing after a hole was only read once its row started,
  so the slate filling the hole was first listed as the open-ended tail (whole copies) and later
  trimmed; hls.js kept the untrimmed copy and ran 1 s late. Rows are now read up to 6 h ahead,
  so that airing goes on air when its slate does, and the numbering test checks that a listed
  segment never changes.
- **An audio switch in hls.js waits 0.04-0.12 s** while it flushes the old track (S1 measured
  0.35 s in AVFoundation); the harness counts it apart from stalls.
- **Open**: hls.js in WebKit missed the first subtitle segment of the program after the slate
  (cues in its first 6 s) in both runs that played WebKit; Chromium and AVFoundation showed every
  cue. `TestCouchTvLive` plays Chromium unless `VIEWERS` adds `webkit`; check it with the web
  player in Phase 5. AVFoundation plays whole segments, so what a package's last `EXTINF`
  under-reports (64 ms for the copied source here) and audio overruns within the trim slack
  (50 ms) add up to about 0.1 s per boundary; the native players' program-date hold (later)
  absorbs it.

### Phase 5: Web viewer
The core module, `/tv`, overlays, Guide, home card, presence and reactions, guest join.
**Exit**: e2e green; the owner watches an evening on the TV.

**Built so far** (2026-10-12; 6.9 "Settled while building it"): the room (presence,
reactions, schedule changes, reports), the guest link, progress and TV watch time (with an
admin dashboard card), the `TvStatsRecorder` hook for ranks, the core's `couchtv` module with
its scenarios, and `ElementPlayer`'s live mode (hls.js live settings, the program date, the
rate); the native players follow the rate.

**Built** (2026-10-12; 7.2 "As built"): the web pages on the core's views (`/tv`, the guide,
the home card, the join page) and the admin's guest link panel; `e2e/couchtv.spec.ts` (a member
and a guest) and the `/tv` accessibility audit pass in Chromium and WebKit with the rest of the
suite. The owner's evening on the TV remains.

### Phase 6: Integration and admin tools
Progress, analytics, ranks XP and achievements; the EPG editor; per-title tools (promo audio
picker, markers, shots, "Never on Couch TV"). **Exit**: conformance + scenarios green.

**Admin tools built** (2026-10-12; 6.4 "As built"): the EPG editor and its two operations, the
template editor, render health and the title tools, with conformance cases,
`TestCouchTvAiringEdits` and `e2e/couchtv-admin.spec.ts` (Chromium and WebKit). Progress and
analytics came with Phase 5.

**Ranks built** (2026-10-12; 6.9 "Ranks, as built"): the room's `TvStatsRecorder` wired to
ranks, XP for channel minutes, the eight achievements in every client (web, Apple, Android)
in English and Czech, unit tests for every rule and `TestRanksCouchTv` against a database.

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
