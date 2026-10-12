// Plays the Couch TV live stitching spike's channel (backend TestLiveSpike) in
// hls.js the way the web player will: Chromium and WebKit, plus pairs of
// Chromium viewers tuning in 20 s apart, first with hls.js's own live edge and
// then aligned on the program date. Every page samples the program date, the
// picture and the dominant audio tone four times a second, switches the audio
// language and turns subtitles on midway; the script checks the samples
// against the schedule (stalls, boundaries crossed, picture and sound changing
// item on the program date, subtitle cues, live edge, sync between viewers).
// Prints one line per check and exits non-zero on a failure. Items of kind slate
// (Couch TV's stand-in picture: dark and silent) look like programs to the
// picture check, which only tells breaks (plain blue) from the rest.
// usage: SERVER=http://127.0.0.1:port SECONDS=130 [MASTER=/path/master.m3u8] node e2e/live.mjs
import { writeFileSync } from 'node:fs';
import { chromium, webkit } from 'playwright';

const server = process.env.SERVER;
const seconds = Number(process.env.SECONDS ?? 130);
const hlsPath = new URL('../node_modules/hls.js/dist/hls.min.js', import.meta.url).pathname;
if (!server) throw new Error('SERVER is required');

const master = process.env.MASTER ?? '/live/master.m3u8';
const schedule = await (await fetch(`${server}/schedule.json`)).json();
const target = schedule.targetDuration;
// the plan's starting point: three target durations behind the live edge
const latency = 3 * target;

// The page: hls.js on a <video>, a Web Audio analyser for the tone and a tiny
// canvas for the picture. Playback starts on a click so WebKit allows sound.
const page = String.raw`<!doctype html>
<html><body style="margin:0;background:#000">
<button id="start" style="position:fixed;inset:0;opacity:0">start</button>
<video id="v" width="640" height="360" playsinline></video>
<canvas id="c" width="64" height="36" hidden></canvas>
<script src="/hls.min.js"></script>
<script>
const params = new URLSearchParams(location.search);
const lang = params.get('lang');
const latency = Number(params.get('latency'));
const target = Number(params.get('target'));
const duration = Number(params.get('duration'));
const sync = params.get('sync');
const video = document.getElementById('v');
const spike = (window.spike = { samples: [], events: [], errors: [], fatal: null, audioTracks: [], subtitleTracks: [] });
const log = (type, detail) => spike.events.push({ wall: Date.now(), type, detail });
const hls = new Hls({ liveSyncDuration: latency, liveMaxLatencyDuration: latency + 2 * target, backBufferLength: 30 });
hls.on(Hls.Events.ERROR, (_, d) => {
  spike.errors.push({ wall: Date.now(), type: d.type, details: d.details, fatal: d.fatal, reason: d.reason || (d.error && d.error.message) || '' });
  if (d.fatal) spike.fatal = d.details;
});
hls.on(Hls.Events.AUDIO_TRACKS_UPDATED, () => (spike.audioTracks = hls.audioTracks.map((t) => t.lang)));
hls.on(Hls.Events.SUBTITLE_TRACKS_UPDATED, () => (spike.subtitleTracks = hls.subtitleTracks.map((t) => t.lang)));
hls.on(Hls.Events.AUDIO_TRACK_SWITCHED, (_, d) => log('audioSwitched', hls.audioTracks[d.id].lang));
hls.on(Hls.Events.SUBTITLE_TRACK_SWITCH, (_, d) => log('subtitleSwitch', d.id));
for (const type of ['playing', 'waiting', 'seeking', 'seeked']) video.addEventListener(type, () => log(type));
hls.loadSource(params.get('master') + '?lang=' + lang);
hls.attachMedia(video);

let analyser, bins, sampleRate;
const canvas = document.getElementById('c');
const g = canvas.getContext('2d', { willReadFrequently: true });
function tone() {
  if (!analyser) return 0;
  analyser.getFloatFrequencyData(bins);
  let best = 0;
  for (let i = 1; i < bins.length; i++) if (bins[i] > bins[best]) best = i;
  return bins[best] > -70 ? Math.round((best * sampleRate) / analyser.fftSize) : 0;
}
// the breaks are plain blue below their moving box; programs are test patterns
function picture() {
  if (video.readyState < 2) return null;
  g.drawImage(video, 0, 0, 64, 36);
  const blue = [10, 32, 54].every((x) => {
    const [r, gr, b] = g.getImageData(x, 27, 1, 1).data;
    return Math.abs(r - 16) < 40 && Math.abs(gr - 64) < 40 && Math.abs(b - 192) < 40;
  });
  return blue ? 'break' : 'program';
}
function cues() {
  const out = [];
  for (const track of video.textTracks) {
    if (track.mode !== 'showing' || !track.activeCues) continue;
    for (const cue of track.activeCues) out.push(cue.text);
  }
  return out;
}
let offset = 0;
document.getElementById('start').addEventListener('click', async () => {
  const ctx = new AudioContext();
  const source = ctx.createMediaElementSource(video);
  analyser = ctx.createAnalyser();
  analyser.fftSize = 8192;
  analyser.smoothingTimeConstant = 0;
  const silent = ctx.createGain();
  silent.gain.value = 0;
  source.connect(analyser).connect(silent).connect(ctx.destination);
  bins = new Float32Array(analyser.frequencyBinCount);
  sampleRate = ctx.sampleRate;
  ctx.resume();
  const { now } = await (await fetch('/clock')).json();
  offset = now - Date.now();
  video.play();
  const started = Date.now();
  setInterval(() => {
    const date = hls.playingDate ? hls.playingDate.getTime() : null;
    spike.samples.push({
      wall: Date.now() + offset, time: video.currentTime, date, latency: hls.latency, paused: video.paused,
      tone: tone(), picture: picture(), width: video.videoWidth, cues: cues(),
      audio: hls.audioTracks[hls.audioTrack] ? hls.audioTracks[hls.audioTrack].lang : null, rate: video.playbackRate,
      dropped: video.getVideoPlaybackQuality ? video.getVideoPlaybackQuality().droppedVideoFrames : 0
    });
  }, 250);
  // a quarter in the viewer turns English subtitles on (both subtitled
  // programs have them), later picks the other audio language
  setTimeout(() => {
    hls.subtitleDisplay = true;
    hls.subtitleTrack = hls.subtitleTracks.findIndex((t) => t.lang === 'en');
  }, duration * 250);
  setTimeout(() => {
    log('switchAudio');
    hls.audioTrack = hls.audioTracks.findIndex((t) => t.lang !== lang);
  }, duration * 400);
  if (sync === 'pdt') {
    // hold the picture latency seconds behind the server clock: a small drift
    // is played away up to 10 % faster or slower, a large one is seeked
    setInterval(() => {
      if (!hls.playingDate || video.paused || video.seeking) return;
      const drift = (hls.playingDate.getTime() - (Date.now() + offset - latency * 1000)) / 1000;
      if (Math.abs(drift) > 2) {
        log('resync', drift);
        video.playbackRate = 1;
        video.currentTime -= drift;
      } else {
        video.playbackRate = Math.abs(drift) < 0.1 ? 1 : 1 - Math.max(-0.1, Math.min(0.1, drift / 5));
      }
    }, 500);
  }
});
</script></body></html>`;

let failures = 0;
function report(ok, name, detail) {
	if (!ok) failures++;
	console.log(`${ok ? 'ok  ' : 'FAIL'} ${name}${detail ? `: ${detail}` : ''}`);
}

async function view(browserType, { name, lang, delay = 0, duration, sync = 'edge' }) {
	await new Promise((r) => setTimeout(r, delay * 1000));
	const browser = await browserType.launch({
		args: browserType === chromium ? ['--autoplay-policy=no-user-gesture-required'] : []
	});
	try {
		const context = await browser.newContext({ viewport: { width: 640, height: 360 } });
		const tab = await context.newPage();
		const pageErrors = [];
		tab.on('pageerror', (e) => pageErrors.push(e.message));
		await tab.route('**/hls.min.js', (r) =>
			r.fulfill({ path: hlsPath, contentType: 'text/javascript' })
		);
		await tab.route('**/player.html*', (r) => r.fulfill({ body: page, contentType: 'text/html' }));
		const q = new URLSearchParams({ lang, latency, target, duration, sync, master });
		await tab.goto(`${server}/player.html?${q}`);
		await tab.click('#start');
		await tab.waitForTimeout(duration * 1000);
		const result = await tab.evaluate(() => window.spike);
		return { name, lang, sync, delay, ...result, pageErrors };
	} finally {
		await browser.close();
	}
}

const itemAt = (date) => schedule.items.find((i) => date >= i.start && date < i.end);
// what the picture check tells apart: breaks are plain blue, the rest is not
const looks = (item) => (item?.kind === 'break' ? 'break' : 'program');
const boundaries = schedule.items.map((i) => i.start);
const nearBoundary = (date, margin) => boundaries.some((b) => Math.abs(date - b) < margin);
const stats = (values) => {
	const sorted = [...values].sort((a, b) => a - b);
	const pick = (p) => sorted[Math.min(sorted.length - 1, Math.floor(p * sorted.length))];
	return { min: sorted[0], p50: pick(0.5), max: sorted[sorted.length - 1] };
};
const fmt = (v) => (v === undefined ? '-' : v.toFixed(2));

function check(v) {
	const label = v.name;
	report(
		!v.fatal && v.pageErrors.length === 0,
		`${label} plays without fatal errors`,
		v.fatal || v.pageErrors.join('; ')
	);
	const errors = {};
	for (const e of v.errors) errors[e.details] = (errors[e.details] ?? 0) + 1;
	if (Object.keys(errors).length)
		console.log(`     ${label} non-fatal errors ${JSON.stringify(errors)}`);
	const played = v.samples.filter((s) => s.date !== null && !s.paused);
	if (played.length < 20) {
		report(false, `${label} played`, `${played.length} samples with a program date`);
		return null;
	}
	// stalls: waiting after playback started, not caused by a seek; a wait
	// right after the viewer switched audio is hls.js flushing the old track
	const firstPlaying = v.events.find((e) => e.type === 'playing');
	let stalls = 0;
	let stalled = 0;
	let switchWaits = 0;
	let lastSeek = -Infinity;
	let lastSwitch = -Infinity;
	for (const [i, e] of v.events.entries()) {
		if (e.type === 'seeking') lastSeek = e.wall;
		if (e.type === 'switchAudio') lastSwitch = e.wall;
		if (
			e.type !== 'waiting' ||
			!firstPlaying ||
			e.wall < firstPlaying.wall ||
			e.wall - lastSeek < 1500
		)
			continue;
		const resumed = v.events.slice(i).find((n) => n.type === 'playing');
		if (e.wall - lastSwitch < 1500) {
			switchWaits += ((resumed?.wall ?? e.wall) - e.wall) / 1000;
			continue;
		}
		stalls++;
		stalled += ((resumed?.wall ?? e.wall) - e.wall) / 1000;
	}
	report(
		stalls === 0,
		`${label} never stalls`,
		`${stalls} stalls, ${stalled.toFixed(2)}s` +
			(switchWaits ? `; the audio switch waited ${switchWaits.toFixed(2)}s` : '')
	);

	const first = played[Math.floor(played.length / 10)];
	const last = played[played.length - 1];
	const crossed = boundaries.filter((b) => b > first.date && b < last.date).length;
	const drift = (last.date - first.date - (last.wall - first.wall)) / 1000;
	report(
		crossed >= 3 && (v.sync === 'pdt' || Math.abs(drift) < 1.5),
		`${label} advances across boundaries`,
		`${crossed} boundaries, program date drifts ${fmt(drift)}s from the clock`
	);
	// how fast each item plays (media seconds per second), seeks left out
	const speed = {};
	const seeks = v.events.filter((e) => e.type === 'seeking').map((e) => e.wall);
	for (let i = 1; i < played.length; i++) {
		const [a, b] = [played[i - 1], played[i]];
		const item = itemAt(a.date)?.name;
		if (item !== itemAt(b.date)?.name || seeks.some((w) => w > a.wall - 500 && w <= b.wall))
			continue;
		speed[item] ??= [0, 0];
		speed[item][0] += b.time - a.time;
		speed[item][1] += (b.wall - a.wall) / 1000;
	}
	console.log(
		`     ${label} speed ${Object.entries(speed)
			.map(([name, [media, wall]]) => `${name} ${fmt(media / wall)}`)
			.join(', ')}; ${played[played.length - 1].dropped} frames dropped`
	);
	// where the program date left the clock: a hold (stall, switch) or a jump
	const t0 = played[0].wall;
	for (let i = 1; i < played.length; i++) {
		const [a, b] = [played[i - 1], played[i]];
		const off = (b.date - a.date - (b.wall - a.wall)) / 1000;
		if (Math.abs(off) < 0.3) continue;
		const near = v.events
			.filter((e) => Math.abs(e.wall - b.wall) < 1500)
			.map(
				(e) =>
					e.type +
					(e.detail !== undefined
						? `(${typeof e.detail === 'number' ? fmt(e.detail) : e.detail})`
						: '')
			);
		console.log(
			`     ${label} at ${fmt((b.wall - t0) / 1000)}s the date moved ${fmt(off)}s off the clock ` +
				`(${itemAt(a.date)?.name} -> ${itemAt(b.date)?.name}, time ${fmt(a.time)} -> ${fmt(b.time)}) ${near.join(' ')}`
		);
	}

	// the picture and the tone change item on the program date
	const switchedAt =
		v.events.find((e) => e.type === 'audioSwitched' && e.detail !== v.lang)?.wall ?? Infinity;
	const settled = played.filter(
		(s) =>
			s.wall - first.wall > 0 && !nearBoundary(s.date, 600) && Math.abs(s.wall - switchedAt) > 1500
	);
	const pictureMiss = settled.filter(
		(s) => s.picture && s.picture !== looks(itemAt(s.date))
	).length;
	const heard = settled.filter((s) => s.tone > 0);
	const toneMiss = heard.filter((s) => {
		const item = itemAt(s.date);
		return item && Math.abs(s.tone - item.tones[s.audio]) > 15;
	}).length;
	report(
		pictureMiss <= settled.length * 0.02,
		`${label} picture matches the program date`,
		`${pictureMiss}/${settled.length} samples off`
	);
	if (heard.length < settled.length / 2) {
		console.log(
			`     ${label} audio analysis unavailable (${heard.length}/${settled.length} samples with a tone)`
		);
	} else {
		report(
			toneMiss <= heard.length * 0.02,
			`${label} sound matches the program date and language`,
			`${toneMiss}/${heard.length} samples off`
		);
	}
	// how far from the scheduled instant each boundary shows up in picture and sound
	const offsets = { picture: [], tone: [] };
	for (const b of boundaries.filter((b) => b > first.date && b < last.date)) {
		const after = itemAt(b);
		const p =
			looks(after) !== looks(itemAt(b - 1)) &&
			played.find((s) => s.date >= b - 2000 && s.picture === looks(after));
		const t = played.find(
			(s) =>
				s.date >= b - 2000 &&
				s.tone > 0 &&
				Math.abs(s.tone - after.tones[s.audio]) <= 15 &&
				s.tone !== itemAt(b - 1)?.tones[s.audio]
		);
		if (p) offsets.picture.push((p.date - b) / 1000);
		if (t) offsets.tone.push((t.date - b) / 1000);
	}
	console.log(
		`     ${label} boundary offsets: picture ${offsets.picture.map(fmt).join(' ')}, sound ${offsets.tone.map(fmt).join(' ')} (s, sampled every 0.25 s)`
	);
	report(
		[...offsets.picture, ...offsets.tone].every((o) => Math.abs(o) <= 0.75),
		`${label} changes item within 0.75 s of the program date`
	);

	report(
		v.events.some((e) => e.type === 'audioSwitched' && e.detail !== v.lang),
		`${label} switches audio`,
		`tracks ${v.audioTracks.join(',')}`
	);
	// subtitles: once on, every cue of the schedule in the played range shows up
	const subsOn = v.events.find((e) => e.type === 'subtitleSwitch' && e.detail >= 0)?.wall;
	const subLang = 'en';
	const shown = played.filter((s) => subsOn && s.wall > subsOn + 3000);
	let expected = 0;
	let hit = 0;
	let wrong = 0;
	for (const s of shown) {
		const cue = (itemAt(s.date)?.subtitles[subLang] ?? []).find(
			(c) => s.date > c.start + 400 && s.date < c.end - 400
		);
		const inGap = (itemAt(s.date)?.subtitles[subLang] ?? []).every(
			(c) => s.date < c.start - 400 || s.date > c.end + 400
		);
		if (cue) {
			expected++;
			if (s.cues.includes(cue.text)) hit++;
		} else if (inGap && s.cues.length) {
			wrong++;
		}
	}
	report(
		expected > 0 && hit >= expected * 0.9 && wrong === 0,
		`${label} shows ${subLang} subtitles on time`,
		`${hit}/${expected} cue samples, ${wrong} out of place`
	);

	const steady = played.filter((s) => s.wall - played[0].wall > 15000).map((s) => s.latency);
	const lat = stats(steady);
	console.log(
		`     ${label} latency ${fmt(lat.min)}..${fmt(lat.max)}s (median ${fmt(lat.p50)}, target ${latency}s), resyncs ${v.events.filter((e) => e.type === 'resync').length}`
	);
	if (v.sync === 'edge')
		report(
			lat.min > latency - target - 1 && lat.max < latency + target + 1,
			`${label} holds the live edge`
		);
	return played;
}

// the second viewer of a pair tunes in 20 s later; at every moment both should show the same program date
// hls.js's own live edge moves in whole segments, so viewers on it drift apart
// (S1): that pair is reported, the program-date hold is what players use
function compare(name, a, b, required = true) {
	if (!a || !b) return;
	const diffs = [];
	for (const s of b.filter((s) => s.wall - b[0].wall > 10000)) {
		const other = a.reduce((best, o) =>
			Math.abs(o.wall - s.wall) < Math.abs(best.wall - s.wall) ? o : best
		);
		if (Math.abs(other.wall - s.wall) < 200)
			diffs.push(Math.abs(s.date - other.date - (s.wall - other.wall)) / 1000);
	}
	const d = stats(diffs);
	const detail = `program dates differ by ${fmt(d.p50)}s median, ${fmt(d.max)}s max`;
	if (!required) {
		console.log(`     ${name} viewers 20 s apart: ${detail}`);
		return;
	}
	report(diffs.length > 20 && d.max <= 2, `${name} viewers 20 s apart stay in sync`, detail);
}

const viewers = [
	{ browser: chromium, name: 'chromium', lang: 'en', duration: seconds },
	{ browser: chromium, name: 'chromium +20s', lang: 'cs', delay: 20, duration: seconds - 20 },
	{ browser: chromium, name: 'chromium pdt', lang: 'cs', duration: seconds, sync: 'pdt' },
	{
		browser: chromium,
		name: 'chromium pdt +20s',
		lang: 'en',
		delay: 20,
		duration: seconds - 20,
		sync: 'pdt'
	},
	{ browser: webkit, name: 'webkit', lang: 'cs', duration: seconds }
	// VIEWERS=chromium,webkit plays only those
].filter((v) => !process.env.VIEWERS || process.env.VIEWERS.split(',').includes(v.name));
const results = await Promise.all(viewers.map((v) => view(v.browser, v)));
if (process.env.OUT) writeFileSync(process.env.OUT, JSON.stringify({ schedule, results }));
const played = {};
for (const v of results) played[v.name] = check(v);
compare('hls.js live edge:', played['chromium'], played['chromium +20s'], false);
compare('program-date aligned:', played['chromium pdt'], played['chromium pdt +20s']);
compare('chromium and webkit:', played['chromium'], played['webkit']);
process.exit(failures ? 1 : 0);
