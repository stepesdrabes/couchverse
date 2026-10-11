# Couch TV sample reel

Eleven interstitials for the "Couch TV" channel, plus a chained break. They use vanilla HTML/CSS/JS
(Web Animations and Web Audio) and no libraries. Run `python3 -m http.server <port>` in this folder,
then open it in a browser. Your first click starts the audio.

This is the planning reference for `docs/couch-tv-plan.md` (Phase 2 ports it to
`clients/broadcast`). The media is not committed: `COUCHVERSE_SSH=user@host ./fetch-media.sh`
cuts the soundtrack excerpts and copies the artwork from a server whose library has the titles.

## Files

`index.html` (stage and controls), `css/tv.css` (broadcast package), `js/core.js` (Timeline,
curves, palette, framing), `js/audio.js` (engine, onsets, sonic logo, SFX), `js/templates.js`,
`js/reel.js` (EN/CS data, player).

## Template contract

`template(stage, data, audio) -> Timeline { duration, cues[] }`. Animations are paused WAAPI
effects, each delayed to its cue time. Seeking sets `currentTime`. Playing gives every effect one
`startTime`, taken from `AudioContext.getOutputTimestamp()`, so picture and sound share a clock.
Sound cues are `(when, bus, skip)` callbacks. A new title is only new data.

## Cue structure (seconds)

1. **ident** (full, 11.5 s; `ident-short`, 5 s): one design, one score, two cuts. Each cut is a cue table (`IDENT_CUTS`) that drives both the picture and the music. There is one idea per beat, and light is used once (a soft red field behind the mark).
   - **Full:** horizon hairline 0.3; the grid drops in on two parallax planes 1.1; the grid crosses 2.6; the lines converge so the centre cell becomes the mark's frame 3.45-4.55.
   - **The reveal:** the hit at 4.6 irises the red square open (spring) and the couch wipes up. The lockup then forms on the mnemonic: the mark steps aside at 5.25, "Couch" slides out from behind it at 5.47, and a flat red "TV" lands on the final chord at 6.15. Tagline and hold from 7.0.
   - **Short:** grid 0-0.5, converge 0.5-0.9, hit 0.95, mark steps aside 1.42, "Couch" 1.64, TV 2.32, hold from 2.75. The break uses the short cut; the squeeze collapses into the same horizon line it opens on.
2. **upNext** (10): 0.45 rail, 0.8 title, 1.05 countdown ring, 1.35 episode, ticks at 7/8/9, 9.1 handover.
3/4. **moviePromo** (`cinematic` 28, `grit` 22.5): sections whose starts snap to the strongest onset within 0.7 s. Fast sections cut on every onset. Iron Man: 0, 3.96 taglines; 8.85 TONIGHT (cuts at 10.40, 11.05, 11.73); 12.39 time slam; 14.59 title; 19.03 end card; 21.78 tag. Pulp (grit) uses the same system with hard cuts, typewriter, chapter cards and a tape stop.
5. **stripPromo** (16, bright or dark): time ~1.8, day chips light on beats from ~3.4, then the episode counter or the season line, tag at 12.
6. **shortPromo** (9): hit = the sting's detected onset (1.10), schedule 1.95, tag 4.7.
7. **lineup** (12): rows from 1.0 (0.22 apart), spotlight from 3.3 (1.75 per row). LIVE or "Next, in N min" is derived from `now`.
8. **overlays** (15): bug 0.4, Now lower third 1.3-6.9, time jump 7.4, Next lower third 8.8-14. No SFX.
9. **creditsSqueeze** (11.5): squeeze 2.0-3.3, up-next panel 2.75. Handover at 9.2: `expand` grows into piece 2's first frame; `mark` collapses into the ident's red square.

The full break plays 9, 1, 3, 6, 7, 2. Each next piece is built 1.4 s ahead, underneath the current one; the outgoing layer and its bus fade out over the overlap.

## Sound

- Piece buses feed a glue compressor and a limiter; the reverb is a generated impulse. Beds are decoded buffers with automated fades and ducks, and the grit promo ends with a tape stop.
- **Theme** (`audio.themeScore(cut)`): a clean sine sub swell; two low-brass statements in fifths and octaves (D, then a B-flat lift), each with one drum; a 16th-note pulse that drops out a bar before the hit; a Shepard riser and an accelerating drum roll; 60 ms of silence. The hit is D major brass, one big drum and a clean sub drop, held by a single warm pad. The mnemonic (D, A, F#, then E) is stated plainly on brass with an octave-up bell, over a D add9 final chord with a sub and one drum. There are no reversed cymbals, choirs, crashes or noise risers.
- **Tag** (`themeTag`): every promo tag is the same theme condensed.
- **SFX**: whooshes, risers, impacts, ticks, shimmer, typewriter, projector, ambient bed.
- **Beat sync: yes.** Onsets are detected in the browser and drive cuts.
- **Offline render** (`await reel.mixReport(id[, solo, raw])`): the ident peaks at -1.2 dBFS going into the dynamics and -3.7 dBFS out, at the promos' level. Two renders differ by at most 1e-6 (float noise).

## Pre-rendering

Every frame is a function of t: animations seek, and audio noise is seeded. The ident is about 80 hairlines plus flat shapes, with no canvas, particles or blur.

## To review

- The channel name on screen is "Couch TV". English gets the most polish; Czech copy stays complete.
- Accent is the more chromatic of backdrop and poster. Ink keeps theme.ts's 4.5:1 target but lightens in HSL, so navy becomes arc-reactor blue instead of grey.
- TV rule: only transform, opacity and visibility animate (plus the ring's stroke). Blur is pre-rendered once.

## Limitations

I couldn't listen (levels are measured, timbre needs ears). Crops are hand-picked; poster crops
are soft. `southpark.m4a` is silent from 4 to 10.7 s. Wordmarks are typographic, credits use
placeholder names, and nothing was tested on Titan OS.

## Freeze

`?piece=<id>&t=<s>&lang=en|cs&mute=1`, where id is ident, ident-short, upnext, ironman, pulp, simpsons, dexter, invincible, lineup, overlays or squeeze. `still=1` hides the bar. Space plays and pauses; arrow keys step while frozen.
