# Couch TV sample reel

Eleven interstitials for the "Couch TV" channel, plus a chained break. They use vanilla HTML/CSS/JS
(Web Animations and Web Audio) and no libraries. Run `python3 -m http.server <port>` in this folder,
then open it in a browser. Your first click starts the audio. The identity is a translation of
Prima COOL's typographic language (Oficina, 2009) into Couch TV's own: see "The brand".
The owner picked ident variant B ("frame") with theme 1 (hybrid epic), the reel's default.

This is the planning reference for `docs/couch-tv-plan.md` (`clients/broadcast` ports it). The
media is not committed: `COUCHVERSE_SSH=user@host ./fetch-media.sh` cuts the soundtrack
excerpts and copies the artwork from a server whose library has the titles.

## Files

`index.html` (stage, controls, the Saira link), `css/tv.css` (broadcast package), `js/core.js`
(Timeline, curves, palette, framing, cut helpers), `js/brand.js` (the wordmark), `js/ident.js`
(both ident variants), `js/theme.js` (the hook and the three arrangements), `js/synth.js` (their
instruments), `js/audio.js` (engine, onsets, SFX), `js/templates.js` (pieces 2-9),
`js/analyze.js` (debug metering), `js/reel.js` (EN/CS data, player, pickers).

## Template contract

`template(stage, data, audio) -> Timeline { duration, cues[] }`. Animations are paused WAAPI
effects, each delayed to its cue time. Seeking sets `currentTime`. Playing gives every effect one
`startTime`, taken from `AudioContext.getOutputTimestamp()`, so picture and sound share a clock.
Sound cues are `(when, bus, skip)` callbacks. A new title is only new data.

## The brand: C()UCH TV

- **The brackets.** The O of COUCH is one Saira O cut down the middle. Closed (0.05 em apart) it
  reads as an O, even in the corner bug; up close it is a pair of brackets, and they are the
  couch's armrests: whatever sits between them is "on the couch". They open to frame a symbol, a
  word, a time, a tiny poster or the couch itself, and snap shut back into the word. Everything
  on screen comes out of the wordmark and goes back into it.
- **The cursor.** TV sits in a red block: the terminal cursor that typed the name. Text is
  edited by typing (cursor, selection, characters typed over), never animated letter by letter.
- **Type.** Saira 700 (OFL), a rounded-rectangle face in Conduit's spirit, loaded from Google
  Fonts for the sample (the product bundles it). Small text stays Plus Jakarta Sans.
- **Colour.** Pure Couchverse red #e50914, black and white, unmixed. Pure RGB appears once per
  ident (three test-card pixels). Fields cut between black, red and white on the beat.
- **Package.** The promo end-card tag, the corner bug and the lineup header use the same lockup.
  The tag lands on the condensed hook: closed on note 1, the brackets open a little on notes 2-4,
  and TV drops into its block on the long note.

## The ident: two variants, two cuts each

Both variants share the grid of the score (120 bpm; `GRID` in theme.js): the full cut hits at
6.0 and lands its final chord at 8.0 (10.5 s); the short cut hits at 2.0 and ends on the hook's
last note at 3.25 (4.5 s). From the hit on, both variants end the same way.

**Variant A, "typed"** (`?piece=ident&variant=a`):

| t | picture | music |
| --- | --- | --- |
| 0.00 | black; the red cursor blinks on the beat | the downbeat |
| 0.50-2.25 | types COUCH TV on eighths, the cursor a cell ahead | keys are the hi-hats |
| 2.75 | selects the O (red selection, the O goes black) | bar 2 |
| 3.00, 3.125 | types ( and ) over it: the O is now brackets | |
| 3.50 | the brackets open in the line: ▶ | |
| 4.00, 4.50, 5.00 | ON, NOW, 20:15 (CS: TEĎ, ŽIVĚ) on quarters | bar 3: the build |
| 5.125-5.375 | Iron Man poster, star, Pulp poster on sixteenths | the roll accelerates |
| 5.50 | the couch sits between its armrests; wind-up at 5.86 | 100 ms of silence |
| 6.00 | **hit**: the brackets snap shut (a hair too far, and back); one beat of pure red | hit, hook note 1 |
| 6.25, 6.50, 7.00 | the brackets open a little on hook notes 2-4, widest on the top note | the hook |
| 7.25 | the cursor stretches over TV: TV in its block | the long note |
| 8.00 | the tagline types itself under the lockup; the cursor blinks on the beat | final chord |

Short cut (`piece=ident-short`): 0.00 the cursor (the squeeze collapses into it); 0.125-0.75
types C ( ) U C H on sixteenths; 1.00 ON, 1.25 20:15, 1.50 Iron Man, 1.625 star, 1.75 couch;
2.00 hit and red beat; 2.25-3.00 hook pulses; 3.25 TV typed inverted into the block; hold.

**Variant B, "frame"** (`?piece=ident&variant=b`): the brackets leave the word and frame the
whole screen, hugging each item, scaling with it, and cutting on the beat.

| t | field | inside the brackets |
| --- | --- | --- |
| 0.00 | black | the lockup, still |
| 1.00 | black | the letters cut away; the brackets burst out around ON |
| 1.50 | red (brackets go black) | 20:15 |
| 2.00 | white | Iron Man poster |
| 2.50, 3.00 | black | star, NOW |
| 3.50 | red | ▶ |
| 4.00 | white | Pulp poster |
| 4.50, 4.75 | black | MOVIES, SERIES (CS: FILMY, SERIÁLY) on eighths |
| 5.00-5.375 | black | RGB pixels, Simpsons, 20:15, Dexter on sixteenths (hard cuts) |
| 5.50 | black | the couch |
| 5.875 | black | the brackets slam back into the O |
| 6.00 | red beat | the letters slide out from behind the brackets; then as A |

Short cut: 0.00 the cursor at the centre (the squeeze collapses into it); 0.25 it becomes the
closed O; 0.50 the brackets take the screen around ON; 1.00 red field, 20:15; 1.25 Iron Man;
1.50 star, 1.625 RGB, 1.75 couch; 1.875 slam; 2.00 hit as A; 3.25 TV; hold.

Flash safety: full-field colour changes at most twice in any second (Ofcom/Harding allow three
flashes a second); the sixteenth-note sections keep the field black and change only the
framed item.

## Picks and URL params

The control bar has **Ident A/B** and **Theme 1/2/3**. Both cuts follow the picks, the promo
tags play the picked theme, and "Play full break" uses the short cut with the squeeze collapsing
into that variant's opening cursor. The default is the recommendation, **A + 1**. Picks persist
in the URL: `?variant=a|b&theme=1|2|3`.

Freeze: `?piece=<id>&t=<s>&variant=a|b&theme=1|2|3&lang=en|cs&mute=1&still=1`, where id is
ident, ident-short, upnext, ironman, pulp, simpsons, dexter, invincible, lineup, overlays or
squeeze. `still=1` hides the bar. Space plays and pauses; arrow keys step while frozen.
Useful stills: `piece=ident&variant=a&t=2.8` (the selected O), `t=4.1` (C(ON)UCH),
`t=5.6` (the couch), `t=6.1` (the red hit), `t=8.9` (lockup and tagline);
`piece=ident&variant=b&t=1.7` (20:15 on red), `t=2.2` (poster on white), `t=5.6`;
`piece=ident-short&variant=a&t=1.3`, `piece=ident-short&variant=b&t=1.1`.

## Other pieces (seconds)

2. **upNext** (10): 0.45 rail, 0.8 title, 1.05 countdown ring, 1.35 episode, ticks at 7/8/9, 9.1 handover.
3/4. **moviePromo** (`cinematic` 28, `grit` 22.5): sections whose starts snap to the strongest onset within 0.7 s. Fast sections cut on every onset. Iron Man: 0, 3.96 taglines; 8.85 TONIGHT (cuts at 10.40, 11.05, 11.73); 12.39 time slam; 14.59 title; 19.03 end card; 21.78 tag. Pulp (grit) uses the same system with hard cuts, typewriter, chapter cards and a tape stop.
5. **stripPromo** (16, bright or dark): time ~1.8, day chips light on beats from ~3.4, then the episode counter or the season line, tag at 12.
6. **shortPromo** (9): hit = the sting's detected onset (1.10), schedule 1.95, tag 4.7.
7. **lineup** (12): rows from 1.0 (0.22 apart), spotlight from 3.3 (1.75 per row). LIVE or "Next, in N min" is derived from `now`.
8. **overlays** (15): bug 0.4, Now lower third 1.3-6.9, time jump 7.4, Next lower third 8.8-14. No SFX.
9. **creditsSqueeze** (11.5): squeeze 2.0-3.3, up-next panel 2.75. Handover at 9.2: `expand` grows into piece 2's first frame; `cursor` (10.6 s, used by the break) collapses the programme into the red cursor block the short ident opens on, over pure black (its last frame matches the ident's first at 47 dB PSNR; only the block's corner radius differs).

The full break plays 9, 1b, 3, 6, 7, 2. Each next piece is built 1.4 s ahead, underneath the current one; the outgoing layer and its bus fade out over the overlap.

## Sound

**The hook** (the sonic logo): A4 D5 A5 G5 F#5 in D, "da-da DAA da DAAA": two eighths, a
quarter, an eighth, then the long note pushed onto the "and" of beat three. Up a fourth and a
fifth (the brackets opening), down two steps to the major third (closing). It lands on the hit in
every arrangement and closes every promo (the tag states it slightly quicker, 0/0.2/0.4/0.8/1.0).

**Theme 1, hybrid epic** (default). D minor builds through i, bVI, bVII to a D major lift on the
hit. Bar 1: a taiko boom and a low Dm braam, low strings swelling under the typing, a spiccato
pulse on eighths. Bar 2: B-flat braam, taiko pattern, the pulse in sixteenths. Bar 3: C braam,
horn stabs, tremolo strings crescendo, an accelerating tom roll and snare roll, a reversed-cymbal
swell and a Shepard riser, 100 ms of silence. Hit: taiko ensemble, crash, D major braam, choir,
sub. The hook on trumpets with horns an octave below and a glockenspiel above, over strings
moving D, C over D, D (mixolydian, each voice a step); big drums on 1 and 3, snare and clap on 2
and 4, a tom fill. Final: D add9 on strings, brass, choir and braam, taikos, glockenspiel F#/A.

**Theme 2, electronic anthem.** vi IV I V (Bm G D A) under filtered supersaws that open bar by
bar and pump on the quarter; a sixteenth pluck arp; kick, clap and offbeat hats from bar 2; the
build drops the kick for a pitched snare roll (eighths to 32nds), a noise riser and a swell. The
drop: I V vi IV (D A Bm G) with the hook on a gliding saw lead (G5 is an appoggiatura over Bm,
resolving to F#5, held into G as a maj7), four-on-the-floor, rolling bass, crash and sub drop,
ping-pong delay. Final: a wide D add9 supersaw with a closing filter.

**Theme 3, warm orchestral.** Felt keys arpeggiate D add9 over a pad; strings enter on Bm7, G,
Em7, Asus4 to A (the suspension resolving), a heartbeat kick becoming a pulse, a timpani roll and
a harp glissando into the hit. The hook on horns (octaves) with celesta and keys, over D, G over
D, D (plagal), a sidechained pluck pulse, choir and strings. Final: D add9 on strings, horns,
choir and pad, timpani, keys rolling up, a harp glissando into it.

**Instruments** (synth.js): seeded-phase supersaws (7 voices, JP-8000 spread, 24 dB filter),
brass with a pitch scoop, filter bloom, breath and late vibrato, strings (4 detuned saws a note,
own vibrato, de-harsh dip, chorus), formant choir, felt keys, glockenspiel, harp, kick with a
pitch envelope, layered clap, snare, hats, crash, taikos with shell and slap, timpani, tuned rolls.
Saturation is tanh with headroom (soft past full scale instead of the WaveShaper's hard clamp).
Each arrangement has its own bus: rumble filter, a 300 Hz dip, an air shelf, the sidechain pump
and a ping-pong delay. The hall reverb is generated: 18 seeded early reflections in 90 ms, then a
tail that builds in density and decays per band (2.5 s lows, 2.0 s mids, 0.85 s highs).

**Metrics** (offline render of the full mix, `await reel.analyze(id, {variant, theme}, {raw})`;
loudness is BS.1770 K-weighted and gated; "raw" is the peak going into the master dynamics):

| | integrated | max short-term | peak out | raw peak in | sub / bass / 200-400 / mid / 2-5k / air (dB of total) |
| --- | --- | --- | --- | --- | --- |
| Theme 1 full | -16.5 LUFS | -14.4 | -3.7 dBFS | -1.8 dBFS | -5.7 / -3.5 / -10.2 / -7.5 / -19.4 / -24.3 |
| Theme 1 short | -15.4 | -14.4 | -3.5 | -1.3 | -6.1 / -3.8 / -9.5 / -6.9 / -18.4 / -22.1 |
| Theme 2 full | -16.7 | -14.4 | -3.4 | -1.7 | -5.8 / -3.8 / -9.2 / -7.8 / -15.7 / -23.5 |
| Theme 2 short | -16.7 | -15.6 | -3.4 | -1.5 | -6.5 / -3.6 / -9.3 / -7.3 / -14.8 / -20.6 |
| Theme 3 full | -17.2 | -15.2 | -3.6 | -2.5 | -4.9 / -4.1 / -8.9 / -8.3 / -22.2 / -25.9 |
| Theme 3 short | -16.4 | -15.6 | -3.6 | -2.4 | -4.3 / -4.4 / -10.1 / -8.2 / -21.6 / -23.5 |
| Promos (5) | -14.8 to -17.1 | -13.0 to -15.0 | -3.1 to -4.3 | | e.g. Iron Man -8.4 / -1.7 / -10.7 / -12.6 / -15.4 / -23.1 |

- Every raw peak is under -1 dBFS; the themes sit inside the promos' loudness range.
- No harsh 2-5 kHz build-up (at or below the soundtracks), no 200-400 Hz excess (the bus dips
  it; the band sits 0.6-2.7 dB under the mids).
- Stem balance (soloed): the hook voice is the loudest melodic part in every arrangement
  (about -21 LUFS alone), chords about -23 to -26, drums -19 to -23, sub -26 to -29.
- The master glue moves the same on the themes as on the promos (median 4.5 dB, 5-95% within
  about 4 dB), so the only audible pump is Theme 2's intentional sidechain.
- `reel.keyReport()`: all 33 charted chords and the hook are in their key (D minor for Theme 1's
  build, D major, D mixolydian for its bVII); every part reads its notes from those charts.
- Determinism: two offline renders differ by at most 7e-7 (float noise in the convolver).

**SFX**: key clicks (A types on the grid), bracket clacks pitched to the D major pentatonic,
whooshes, risers, impacts, ticks, shimmer, typewriter, projector, ambient bed. **Beat sync:
yes.** Onsets are detected in the browser and drive the promo cuts.

## Pre-rendering

Every frame is a function of t: animations seek, audio phases and noise offsets are seeded per
voice (never by call order), and the score is derived from the cue grid. Only transform and
opacity animate (plus the ring's stroke): cuts are opacity steps, so they stay on the compositor
with the transforms they belong to. No canvas animation, blur or backdrop-filter; the brackets'
copies (red, black) swap by opacity when the field turns red.

## To review

- The channel name on screen is "Couch TV". English gets the most polish; Czech copy stays complete.
- Accent is the more chromatic of backdrop and poster. Ink keeps theme.ts's 4.5:1 target but lightens in HSL, so navy becomes arc-reactor blue instead of grey.
- Variant B's brackets get heavy when they hug a poster (about three times the lockup's stroke).

## Limitations

I couldn't listen: levels, balance and spectrum are measured, timbre needs ears. Crops are
hand-picked; poster crops are soft. `southpark.m4a` is silent from 4 to 10.7 s. Saira comes from
Google Fonts here. Credits use placeholder names, and nothing was tested on Titan OS.
