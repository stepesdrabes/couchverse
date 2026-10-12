// Couch TV interstitial templates. Each is template(stage, data, audio) -> Timeline: it builds
// its DOM into `stage`, records every animation and sound cue on one Timeline and returns it
// (duration + named cues). Data is already localized by the caller (strings, not {en, cs});
// `data.lang` picks the template's own UI copy. A new title is new data, never new code.

import {
	Timeline,
	W,
	H,
	el,
	css,
	chars,
	words,
	rise,
	fadeUp,
	fadeOut,
	glint,
	framed,
	inset,
	palette,
	applyPalette,
	pickAccent,
	rng,
	addMinutes,
	motion,
	windows
} from './core.js';
import { SPARKLE } from './audio.js';
import { HOOK } from './theme.js';
import { wordmark, lockupStatic, geometry, cursorAt } from './brand.js';
import { ident, identCursorRect } from './ident.js';

export { ident };

// --- UI copy (in the product these are contract/i18n keys) ---------------------------------

const STR = {
	en: {
		upNext: 'Up next',
		startsIn: 'Starts in',
		now: 'Now',
		live: 'Live',
		next: 'Next',
		laterOn: 'Later on Couch TV',
		taglineA: 'One channel',
		taglineB: 'Every couch',
		episodes: (n) => `<b>${n}</b> episodes, in order`,
		afterBreak: 'after the break',
		inMin: (n) => `in ${n} min`
	},
	cs: {
		upNext: 'Následuje',
		startsIn: 'Začíná za',
		now: 'Právě běží',
		live: 'Živě',
		next: 'Následuje',
		laterOn: 'Dále na Couch TV',
		taglineA: 'Jeden kanál',
		taglineB: 'Každý gauč',
		episodes: (n) => `<b>${n}</b> dílů, pěkně popořadě`,
		afterBreak: 'po přestávce',
		inMin: (n) => `za ${n} min`
	}
};

const DAYS = {
	en: {
		short: ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'],
		name: ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday']
	},
	cs: {
		short: ['Po', 'Út', 'St', 'Čt', 'Pá', 'So', 'Ne'],
		on: ['v pondělí', 'v úterý', 've středu', 've čtvrtek', 'v pátek', 'v sobotu', 'v neděli'],
		from: ['od pondělí', 'od úterý', 'od středy', 'od čtvrtka', 'od pátku', 'od soboty', 'od neděle']
	}
};

const cap = (s) => s.charAt(0).toUpperCase() + s.slice(1);

// Czech takes "ve" before hours that start with a consonant cluster (dvě, tři, čtyři, dvanáct,
// třináct, čtrnáct, dvacet...): "v 18:00" but "ve 20:15".
function csAt(time) {
	const h = +time.split(':')[0];
	return ([2, 3, 4, 12, 13, 14, 20, 21, 22, 23].includes(h) ? 've ' : 'v ') + time;
}

/**
 * The schedule copy for a promo's `when`: eyebrow (small caps), word (the big kinetic word),
 * day (the end card's day, if any) and line (a full sentence). 24-hour times in both languages.
 */
export function phrase(w, lang) {
	const cs = lang === 'cs';
	const t = w.time;
	switch (w.kind) {
		case 'tonight':
			return cs
				? { eyebrow: 'Dnes večer', word: 'Dnes večer', day: '', line: `Dnes ${csAt(t)}` }
				: { eyebrow: 'Tonight', word: 'Tonight', day: '', line: `Tonight at ${t}` };
		case 'tomorrow':
			return cs
				? { eyebrow: 'Zítra', word: 'Zítra', day: '', line: `Zítra ${csAt(t)}` }
				: { eyebrow: 'Tomorrow', word: 'Tomorrow', day: '', line: `Tomorrow at ${t}` };
		case 'premiere': {
			const d = w.day;
			return cs
				? { eyebrow: 'Premiéra', word: cap(DAYS.cs.on[d]), day: cap(DAYS.cs.on[d]), line: `Premiéra ${DAYS.cs.on[d]} ${csAt(t)}` }
				: { eyebrow: 'Premiere', word: DAYS.en.name[d], day: DAYS.en.name[d], line: `Premiere ${DAYS.en.name[d]} at ${t}` };
		}
		case 'weekdays':
			return cs
				? { eyebrow: 'Každý všední den', word: 'Každý všední den', day: '', line: `Každý všední den ${csAt(t)}` }
				: { eyebrow: 'Every weekday', word: 'Every weekday', day: '', line: `Every weekday at ${t}` };
		case 'nightly':
			return cs
				? { eyebrow: 'Každý večer', word: 'Každý večer', day: '', line: `Každý večer ${csAt(t)}` }
				: { eyebrow: 'Every night', word: 'Every night', day: '', line: `Every night at ${t}` };
	}
	return { eyebrow: '', word: '', day: '', line: t };
}

function fromLine(from, lang) {
	return lang === 'cs'
		? { pre: `${from.season}. řada `, day: DAYS.cs.from[from.day] }
		: { pre: `Season ${from.season} from `, day: DAYS.en.name[from.day] };
}

// --- shared building blocks ------------------------------------------------------------------

function titlePalette(title) {
	return palette(pickAccent(title.accents.backdrop, title.accents.poster));
}

function glow(parent, kind, x, y, r) {
	return css(el('div', `glow ${kind}`, parent), {
		left: x - r + 'px',
		top: y - r + 'px',
		width: 2 * r + 'px',
		height: 2 * r + 'px'
	});
}

/** Corner bug: lockup, LIVE dot, clock. `clock(t)` returns the text to show at time t. */
function bug(tl, parent, clock, lang) {
	const b = el('div', 'bug', parent);
	lockupStatic(b, 27, 'bug-wm');
	el('span', 'bug-sep', b);
	const live = el('span', 'live', b);
	const dot = el('span', 'live-dot', live);
	el('span', null, live, STR[lang].live);
	el('span', 'bug-sep', b);
	const c = el('span', 'clock', b, clock(0));
	let last = '';
	tl.tick((t) => {
		const v = clock(t);
		if (v !== last) c.textContent = last = v;
	});
	// the dot breathes for the whole piece (iterations, so seeking still lands right)
	tl.add(dot, [{ opacity: 1 }, { opacity: 0.35 }, { opacity: 1 }], 0, 1.6, 'sine', { iterations: Math.ceil(tl.duration / 1.6) });
	return b;
}

/** Fit a single line into maxWidth by scaling its font size down (long titles, Czech). */
function fit(e, maxWidth) {
	const w = e.scrollWidth;
	if (w > maxWidth) {
		const fs = parseFloat(getComputedStyle(e).fontSize);
		e.style.fontSize = Math.floor((fs * maxWidth) / w) + 'px';
	}
	return e;
}

function setHTML(e, html) {
	e.innerHTML = html;
	return e;
}

/** Strongest detected onset near t (weighted by distance), or t when nothing qualifies. */
function snapper(onsets) {
	return (t, win = 0.6, min = 0.4) => {
		let best = t;
		let score = -1;
		for (const o of onsets) {
			const d = Math.abs(o.t - t);
			if (d > win || o.s < min) continue;
			const sc = o.s * (1 - (0.6 * d) / win);
			if (sc > score) {
				score = sc;
				best = o.t;
			}
		}
		return best;
	};
}

/** Onsets in (a, b) at least `gap` apart and `min` strong: cut points that follow the music. */
function beatsIn(onsets, a, b, gap, min, tail = gap * 0.6) {
	const out = [];
	let last = a;
	for (const o of onsets) {
		if (o.t <= last + gap || o.t >= b - tail || o.s < min) continue;
		out.push(o.t);
		last = o.t;
	}
	return out;
}

/** Shake: a short decaying jitter on a wrapper (for hits). */
function shake(tl, target, at, { dur = 0.35, amp = 14, seed = 1 } = {}) {
	const r = rng(seed);
	const n = 9;
	const frames = [];
	for (let i = 0; i <= n; i++) {
		const k = 1 - i / n;
		const x = i === n ? 0 : (r() * 2 - 1) * amp * k;
		const y = i === n ? 0 : (r() * 2 - 1) * amp * k;
		frames.push({ transform: `translate(${x.toFixed(1)}px, ${y.toFixed(1)}px)` });
	}
	tl.add(target, frames, at, dur, 'linear');
}

/** Film grain: a noise tile jumped to a new offset every frame-ish (steps, composited). */
function grain(tl, parent, opacity, seed = 5) {
	const g = css(el('div', 'grain', parent), { opacity });
	const r = rng(seed);
	const n = 12;
	const frames = [];
	for (let i = 0; i < n; i++) frames.push({ transform: `translate(${Math.round(r() * 256)}px, ${Math.round(r() * 256)}px)` });
	tl.add(g, frames, 0, 0.5, `steps(${n}, jump-none)`, { iterations: Math.ceil(tl.duration / 0.5) + 1 });
	return g;
}

/**
 * The end-of-promo channel tag: the wordmark lands on the theme's condensed hook. It appears
 * closed on the first note, the brackets open a little on the next three (wider on the top
 * note) and TV drops into its block on the long last note, as the ident does.
 */
function channelTag(tl, parent, at, audio, { size = 62, cls = 'tag-lockup', gain = 0.5 } = {}) {
	const wrap = el('div', cls, parent);
	const wm = wordmark(wrap, size, { cls: 'wm-tag' });
	const m = wm.m;
	const R = HOOK.tagAt.map((r) => at + r);
	const D = tl.duration;
	tl.add(wrap, [{ opacity: 0, transform: 'scale(0.92)' }, { opacity: 1, transform: 'scale(1)' }], at - 0.02, 0.2, 'snap');
	const moves = [];
	[[R[1], 0.2], [R[2], 0.42], [R[3], 0.22]].forEach(([t, w]) => {
		moves.push({ at: t, dur: 0.05, to: m.em * w + m.split, ease: 'out' });
		moves.push({ at: t + 0.08, dur: 0.1, to: m.split, ease: 'inOut' });
	});
	const geo = (g) => geometry(m, g);
	motion(tl, wm.halves.red.l, D, m.split, moves, (g) => ({ transform: geo(g).l }));
	motion(tl, wm.halves.red.r, D, m.split, moves, (g) => ({ transform: geo(g).r }));
	motion(tl, wm.c, D, m.split, moves, (g) => ({ transform: geo(g).c }));
	motion(tl, wm.rest, D, m.split, moves, (g) => ({ transform: geo(g).rest }));
	motion(tl, wm.cur, D, cursorAt(m, m.tv.x), [{ at: R[4], dur: 0.12, to: cursorAt(m, m.tv.x, m.tv.w), ease: 'snap' }], (v) => ({ transform: v }));
	windows(tl, wm.cur, [[0, R[4] + 0.12]]);
	windows(tl, wm.tvBg, [[R[4] + 0.12, D + 1]]);
	windows(tl, wm.tvT, [[R[4], D + 1]]);
	windows(tl, wm.tvV, [[R[4] + 0.06, D + 1]]);
	// the event starts a beat early: the tag's swell leads into the first note
	tl.sound(at - 0.4, (w, b) => audio.themeTag(b, w + 0.4, { gain }));
	tl.cue('channel tag (theme)', at);
	return wrap;
}

// =========================================================================================
// 2. "Up next" bumper
// =========================================================================================

export function upNext(stage, data, audio) {
	const S = STR[data.lang];
	const t = data.title;
	const D = data.duration || 10;
	const tl = new Timeline(stage, D);
	applyPalette(stage, titlePalette(t));
	stage.classList.add('upnext');
	const continuing = data.enter === 'continue';

	const shot = framed(stage, t.art.b, t.art.dims.b);
	const fr = t.frames.upnext;
	tl.add(shot.img, [{ transform: shot.tf(fr[0], continuing ? 1.06 : 1) }, { transform: shot.tf(fr[1]) }], 0, D, 'cubic-bezier(0.3, 0.2, 0.6, 1)');
	const scrimL = el('div', 'fill scrim-left', stage);
	const scrimB = el('div', 'fill scrim-bottom', stage);
	if (!continuing) tl.add(shot.el, [{ opacity: 0 }, { opacity: 1 }], 0, 1.0, 'sine');
	tl.add(scrimL, [{ opacity: 0 }, { opacity: 0.85 }], 0.1, 1.0, 'sine');
	tl.add(scrimB, [{ opacity: 0 }, { opacity: 1 }], 0.1, 1.0, 'sine');
	const b = bug(tl, stage, () => data.clock, data.lang);
	fadeUp(tl, b, 0.5, { y: -10 });

	const info = el('div', 'un-info', stage);
	const ebRow = el('div', 'un-eyebrow', info);
	const rail = el('div', 'rail', ebRow);
	const eb = el('div', 'eyebrow', ebRow, S.upNext);
	const title = words(el('div', 'hero un-title', info), t.name);
	fit(title.root.parentElement, 1100);
	const epRow = el('div', 'un-ep', info);
	const chip = el('span', 'chip solid', epRow, data.episode.code);
	const epName = el('span', null, epRow, data.episode.name);
	const meta = el('div', 'un-meta', info);
	[data.time, ...t.genres, `${data.episode.runtime} min`].forEach((x, i) => el('span', i ? 'dot-sep' : 'tnum', meta, x));

	const cd = el('div', 'countdown', stage);
	const lbl = el('div', 'cd-label', cd);
	el('div', 'eyebrow', lbl, S.startsIn);
	const ringBox = css(el('div', null, cd), { position: 'relative', width: '168px', height: '168px' });
	const C = 2 * Math.PI * 78;
	setHTML(ringBox, `<svg class="ring-svg" viewBox="0 0 168 168"><circle class="track" cx="84" cy="84" r="78"/><circle class="prog" cx="84" cy="84" r="78" stroke-dasharray="${C.toFixed(1)}"/></svg>`);
	cd.prepend(el('div', 'cd-disc'));
	const prog = ringBox.querySelector('.prog');
	const num = el('div', 'ring-num tnum', ringBox, '0:10');
	const line = el('div', 'progress-line', stage);

	const T0 = continuing ? 0.25 : 0.45;
	tl.cue('eyebrow + rail', T0);
	tl.add(rail, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], T0, 0.75, 'enter');
	tl.add(eb, [{ opacity: 0, transform: 'translateX(-14px)' }, { opacity: 1, transform: 'translateX(0)' }], T0 + 0.15, 0.8, 'enter');
	tl.cue('title', T0 + 0.35);
	rise(tl, title.items, T0 + 0.35, { stagger: 0.09, dur: 1.0 });
	tl.cue('episode', T0 + 0.9);
	tl.add(chip, [{ opacity: 0, transform: 'scale(0.7)' }, { opacity: 1, transform: 'scale(1)' }], T0 + 0.9, 0.5, 'pop');
	tl.add(epName, [{ opacity: 0, transform: 'translateX(-20px)' }, { opacity: 1, transform: 'translateX(0)' }], T0 + 1.0, 0.8, 'enter');
	fadeUp(tl, meta, T0 + 1.2, { y: 12 });

	tl.cue('countdown ring', T0 + 0.6);
	tl.add(cd, [{ opacity: 0, transform: 'scale(0.86)' }, { opacity: 1, transform: 'scale(1)' }], T0 + 0.6, 0.8, 'enter');
	tl.add(prog, [{ strokeDashoffset: C }, { strokeDashoffset: 0 }], T0 + 0.6, 0.7, 'enter');
	tl.add(prog, [{ strokeDashoffset: 0 }, { strokeDashoffset: C }], T0 + 1.3, D - T0 - 1.3, 'linear');
	tl.add(line, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], 0, D, 'linear');
	tl.add(num, [{ transform: 'none', opacity: 1 }, { transform: 'none', opacity: 1 }], 0, 0.01, 'linear');
	for (let s = 1; s < D; s++) tl.add(num, [{ transform: 'translateY(12px)', opacity: 0.25 }, { transform: 'translateY(0)', opacity: 1 }], s, 0.4, 'enter');
	let lastN = -1;
	tl.tick((tt) => {
		const n = Math.max(0, Math.ceil(D - tt - 1e-3));
		if (n !== lastN) num.textContent = `0:${String((lastN = n)).padStart(2, '0')}`;
	});

	const out = D - 0.9;
	tl.cue('handover', out);
	fadeOut(tl, info, out, { dur: 0.6, x: -40, y: 0 });
	fadeOut(tl, cd, out + 0.05, { dur: 0.55, y: 0, scale: 0.9 });
	tl.add(scrimL, [{ opacity: 0.85 }, { opacity: 0.2 }], out, 0.9, 'sine');
	tl.add(scrimB, [{ opacity: 1 }, { opacity: 0.4 }], out, 0.9, 'sine');

	tl.sound(0, (w, bus, skip) => audio.bed(bus, w, t.bed, skip, { fadeIn: 0.35, fadeOut: 1.3, end: D, gain: 0.95 }), D);
	tl.sound(T0 - 0.05, (w, bus) => audio.whoosh(bus, w, { dur: 0.75, pan: [-0.7, -0.2], gain: 0.05, lo: 600, hi: 3200, body: 0.2 }));
	tl.sound(T0 + 0.35, (w, bus) => audio.impact(bus, w, { gain: 0.16, size: 0.6, tone: 52, click: 0.3 }));
	tl.sound(T0 + 0.9, (w, bus) => audio.tick(bus, w, { freq: SPARKLE[0], gain: 0.045, pan: -0.5 }));
	for (const s of [D - 3, D - 2, D - 1]) tl.sound(s, (w, bus) => audio.tick(bus, w, { freq: s === D - 1 ? SPARKLE[4] : SPARKLE[2], gain: 0.04, pan: 0.6, decay: 0.09 }));
	tl.sound(D - 1.6, (w, bus) => audio.riser(bus, w, { dur: 1.5, gain: 0.035, tone: 0.3 }));
	return tl;
}

// =========================================================================================
// 3/4. Movie promo ("Tonight at 20:15", "Premiere Saturday 20:15")
// =========================================================================================

// A promo is a sequence of sections over the title's own soundtrack. Section starts are
// nominal times snapped to the bed's detected onsets; shots inside fast sections cut on beats.
// The mood decides how each section moves and sounds, not what it says.
const MOODS = {
	cinematic: {
		duration: 28,
		dissolve: 0.45,
		minShot: 1.7,
		fastShot: 0.62,
		cutMin: 0.55,
		grain: 0.045,
		sequence: () => [
			{ kind: 'line', at: 0, line: 0, snap: false },
			{ kind: 'line', at: 4.0, line: 1 },
			{ kind: 'word', at: 8.8, fast: true },
			{ kind: 'time', at: 12.4 },
			{ kind: 'title', at: 14.6 },
			{ kind: 'end', at: 19.0 }
		]
	},
	grit: {
		duration: 22.5,
		dissolve: 0,
		minShot: 0.6,
		fastShot: 0.6,
		cutMin: 0.25,
		grain: 0.13,
		sequence: () => [
			{ kind: 'type', at: 0, line: 0, snap: false },
			{ kind: 'shots', at: 1.6, fast: true },
			{ kind: 'card', at: 3.6 },
			{ kind: 'title', at: 5.5 },
			{ kind: 'word', at: 8.3, fast: true },
			{ kind: 'time', at: 10.4 },
			{ kind: 'type', at: 12.6, line: 1, shot: true },
			{ kind: 'end', at: 14.6 }
		]
	}
};

export function moviePromo(stage, data, audio) {
	const t = data.title;
	const mood = data.mood || 'cinematic';
	const P = MOODS[mood];
	const D = data.duration || P.duration;
	const grit = mood === 'grit';
	const ph = phrase(data.when, data.lang);
	const tl = new Timeline(stage, D);
	const pal = titlePalette(t);
	applyPalette(stage, pal);
	stage.classList.add('promo', `mood-${mood}`);

	const onsets = audio.onsets(t.bed);
	const snap = snapper(onsets);
	const seq = P.sequence(data).map((s) => ({ ...s, at: s.snap === false ? s.at : snap(s.at, 0.7, 0.5) }));
	seq.forEach((s, i) => (s.end = i + 1 < seq.length ? seq[i + 1].at : D));

	const world = el('div', 'fill', stage);
	const shots = el('div', 'fill', world);
	// a constant grade over every shot (text always sits on a darker, accent-tinted picture),
	// plus a heavier dim for the sections where big type sits mid-frame
	const grade = css(el('div', 'fill', world), {
		background: `linear-gradient(0deg, rgb(0 0 0 / 0.62) 0%, rgb(0 0 0 / 0.08) 45%, rgb(0 0 0 / 0.25) 100%), radial-gradient(ellipse 80% 70% at 50% 45%, transparent 40%, rgb(${pal.dark.match(/\w\w/g).map((h) => parseInt(h, 16)).join(' ')} / 0.55) 100%)`
	});
	const dim = css(el('div', 'fill', world), { background: 'rgb(0 0 0 / 0.42)', opacity: 0 });
	const blackA = css(el('div', 'fill', world), { background: '#000' });
	const glowA = glow(world, 'accent', 960, 560, 760);
	const text = el('div', 'fill', world);
	const endLayer = el('div', 'fill', stage);
	const vig = el('div', 'vignette', stage);
	if (grit) {
		vig.style.background = 'radial-gradient(ellipse 70% 65% at 50% 50%, transparent 45%, rgb(0 0 0 / 0.75) 100%)';
	}
	grain(tl, stage, P.grain, grit ? 9 : 4);
	const flash = el('div', 'flash', stage);
	let lbT, lbB;
	if (grit) {
		lbT = el('div', 'letterbox top', stage);
		lbB = el('div', 'letterbox bottom', stage);
		// gate weave: the picture breathes a pixel or two, like film in a projector
		const r = rng(31);
		const frames = Array.from({ length: 10 }, () => ({ transform: `translate(${(r() * 2.4 - 1.2).toFixed(2)}px, ${(r() * 3 - 1.5).toFixed(2)}px)` }));
		tl.add(shots, frames, 0, 0.42, 'steps(10, jump-none)', { iterations: Math.ceil(D / 0.42) });
	}
	const black = (a, b) => tl.show(blackA, a, b);
	tl.add(glowA, [{ opacity: 0 }, { opacity: 0 }], 0, 0.01, 'linear');

	const pool = t.shots;
	let shotIndex = 0;
	const nextShot = () => pool[shotIndex++ % pool.length];
	const dims = t.art.dims;

	function addShot(a, b, spec = nextShot(), { dissolve = P.dissolve, punch = grit } = {}) {
		const s = framed(shots, t.art[spec.src], dims[spec.src]);
		const inT = a > 0 && dissolve ? a - dissolve / 2 : a;
		tl.show(s.el, inT, b + (dissolve ? dissolve / 2 : 0) + 0.001);
		if (dissolve && a > 0) tl.add(s.el, [{ opacity: 0 }, { opacity: 1 }], inT, dissolve, 'sine');
		const to = spec.to || inset(spec.from, 0.9);
		if (punch) {
			tl.add(s.img, [{ transform: s.tf(spec.from, 1.16) }, { transform: s.tf(spec.from) }], a, 0.24, 'snap');
			tl.add(s.img, [{ transform: s.tf(spec.from) }, { transform: s.tf(to) }], a + 0.24, Math.max(0.3, b - a - 0.24), 'linear');
		} else {
			tl.add(s.img, [{ transform: s.tf(spec.from) }, { transform: s.tf(to) }], inT, b - inT + (dissolve || 0), 'linear');
		}
		return s;
	}

	function shotRun(a, b, fast) {
		const cuts = [a, ...beatsIn(onsets, a, b, fast ? P.fastShot : P.minShot, P.cutMin)];
		tl.beatCuts = (tl.beatCuts || []).concat(cuts.slice(1));
		cuts.forEach((c, i) => {
			const e = i + 1 < cuts.length ? cuts[i + 1] : b;
			addShot(c, e, nextShot(), { dissolve: fast && !grit ? 0 : P.dissolve });
			if (i > 0) {
				if (grit) tl.sound(c, (w, bus) => audio.punch(bus, w, { gain: 0.22, pan: i % 2 ? 0.3 : -0.3 }));
				else tl.sound(c - 0.12, (w, bus) => audio.whoosh(bus, w, { dur: 0.5, pan: i % 2 ? [0.5, -0.2] : [-0.5, 0.2], gain: 0.03, body: 0.2 }));
			}
		});
		return cuts;
	}

	function flashAt(a, peak = 0.85, dur = grit ? 0.09 : 0.35) {
		tl.add(flash, [{ opacity: peak }, { opacity: 0 }], a, dur, grit ? 'linear' : 'out', { fill: 'forwards' });
	}

	// each section's text exists only inside its section
	const centerBox = (a, b) => {
		const box = el('div', 'center', text);
		tl.show(box, a, b + 0.001);
		return box;
	};
	const dimFor = (a, b) => {
		tl.add(dim, [{ opacity: 0 }, { opacity: 1 }], a, 0.25, 'sine', { fill: 'forwards' });
		tl.add(dim, [{ opacity: 1 }, { opacity: 0 }], b - 0.2, 0.2, 'sine', { fill: 'forwards' });
	};
	void grade;

	const CUE = {
		line: 'tagline line (long take)',
		word: 'kinetic word, cuts on beats',
		time: 'time slam',
		title: 'title wordmark',
		type: 'typewriter line',
		card: 'chapter card',
		shots: 'beat-cut shots',
		end: 'end card'
	};
	for (const s of seq) {
		const { at: a, end: b } = s;
		tl.cue(CUE[s.kind], a);
		switch (s.kind) {
			case 'line': {
				addShot(a, b, nextShot());
				const box = el('div', 'promo-line', text);
				tl.show(box, a, b + 0.001);
				const ws = words(box, t.tagline[s.line]);
				ws.root.style.display = 'inline-block';
				rise(tl, ws.items, a + (a === 0 ? 1.0 : 0.35), { stagger: 0.12, dur: 1.1 });
				// trailer type never sits still: a slow push for the life of the line
				tl.add(ws.root, [{ transform: 'scale(1)' }, { transform: 'scale(1.045)' }], a, b - a, 'linear');
				if (a === 0) tl.add(shots, [{ opacity: 0 }, { opacity: 1 }], 0, 1.4, 'sine');
				fadeOut(tl, box, b - 0.45, { dur: 0.45, y: -10 });
				if (a > 0) tl.sound(a, (w, bus) => audio.impact(bus, w, { gain: 0.16, size: 1.1, tone: 44, click: 0.2, verb: 0.35 }));
				break;
			}
			case 'word': {
				shotRun(a, b, s.fast);
				dimFor(a, b);
				const box = centerBox(a, b);
				const word = grit ? ph.word : ph.eyebrow;
				const cw = chars(el('div', 'promo-word', box), word);
				fit(cw.root.parentElement, 1640);
				if (grit) {
					tl.add(cw.root.parentElement, [{ opacity: 0, transform: 'scale(1.3)' }, { opacity: 1, transform: 'scale(1)' }], a, 0.18, 'snap');
					shake(tl, world, a, { amp: 12, seed: 4 });
					flashAt(a, 0.5);
					tl.sound(a, (w, bus) => audio.punch(bus, w, { gain: 0.42 }));
				} else {
					const mid = (cw.items.length - 1) / 2;
					cw.items.forEach((c, i) => tl.add(c, [{ opacity: 0, transform: `translateX(${(i - mid) * 46}px)` }, { opacity: 1, transform: 'translateX(0)' }], a + 0.05, 1.5, 'enter'));
					cw.root.style.display = 'inline-block';
					tl.add(cw.root, [{ transform: 'scale(1)' }, { transform: 'scale(1.035)' }], a, b - a, 'linear');
					tl.sound(a, (w, bus) => audio.impact(bus, w, { gain: 0.32, size: 1.2, tone: 46, click: 0.5 }));
					tl.sound(a - 0.1, (w, bus) => audio.whoosh(bus, w, { dur: 0.9, pan: [-0.6, 0.6], gain: 0.06 }));
				}
				tl.add(cw.root.parentElement, [{ opacity: 1, transform: 'scale(1)' }, { opacity: 0, transform: 'scale(1.06)' }], b - 0.22, 0.22, 'exit');
				break;
			}
			case 'time': {
				black(a, b);
				const box = centerBox(a, b);
				const eb = el('div', 'eyebrow promo-time-eyebrow', box, ph.eyebrow);
				const timeEl = el('div', 'promo-time tnum', box);
				const tc = chars(timeEl, data.when.time);
				timeEl.style.display = 'inline-block';
				tl.add(glowA, [{ opacity: 0, transform: 'scale(0.7)' }, { opacity: 0.9, transform: 'scale(1)' }], a, 0.9, 'out');
				tl.add(glowA, [{ opacity: 0.9 }, { opacity: 0 }], b - 0.3, 0.3, 'sine');
				if (grit) {
					tl.add(timeEl, [{ opacity: 0, transform: 'scale(1.5)' }, { opacity: 1, transform: 'scale(1)' }], a, 0.16, 'snap');
					shake(tl, world, a, { amp: 16, seed: 9 });
					flashAt(a);
					tl.sound(a, (w, bus) => audio.punch(bus, w, { gain: 0.5 }));
					tl.sound(a, (w, bus) => audio.impact(bus, w, { gain: 0.3, size: 0.8 }));
				} else {
					flashAt(a, 0.55);
					tl.add(timeEl, [{ opacity: 0, transform: 'scale(1.16)' }, { opacity: 1, transform: 'scale(1)' }], a, 0.7, 'enter');
					rise(tl, tc.items, a, { stagger: 0.05, dur: 0.7, from: '40%' });
					glint(tl, timeEl, a + 0.55, 1.0);
					tl.sound(a - 1.3, (w, bus) => audio.riser(bus, w, { dur: 1.3, gain: 0.06, tone: 0.4, root: 98 }));
					tl.sound(a, (w, bus) => audio.impact(bus, w, { gain: 0.55, size: 1.3, tone: 42 }));
					tl.sound(a + 0.55, (w, bus) => audio.shimmer(bus, w, { dur: 1.0, gain: 0.03, seed: 2 }));
				}
				fadeUp(tl, eb, a + 0.25, { dur: 0.7, y: 10 });
				tl.add(box, [{ opacity: 1, transform: 'scale(1)' }, { opacity: 0, transform: 'scale(1.04)' }], b - 0.3, 0.3, 'exit');
				break;
			}
			case 'title': {
				const hero = t.hero || nextShot();
				addShot(a, b, hero, { dissolve: grit ? 0 : 0.3 });
				dimFor(a, b);
				const box = centerBox(a, b);
				const tEl = el('div', 'promo-title', box);
				const tc = chars(tEl, t.name);
				tEl.style.display = 'inline-block';
				fit(tEl, 1660);
				const under = grit ? el('div', 'card-under', box) : null;
				const meta = el('div', 'promo-meta', box);
				[String(t.year), t.genres.join(' / '), `${t.runtime} min`].forEach((x, i) => el('span', i ? 'dot-sep' : '', meta, x));
				if (grit) {
					black(a, a + 0.12);
					tl.add(tEl, [{ opacity: 0, transform: 'scale(1.6)' }, { opacity: 1, transform: 'scale(1)' }], a, 0.2, 'snap');
					shake(tl, world, a, { amp: 22, seed: 13, dur: 0.45 });
					flashAt(a, 0.95, 0.12);
					css(under, { width: tEl.offsetWidth * 0.5 + 'px', marginTop: '14px' });
					tl.add(under, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], a + 0.18, 0.3, 'snap');
					fadeUp(tl, meta, a + 0.5, { y: 8, dur: 0.4 });
					tl.sound(a, (w, bus) => audio.impact(bus, w, { gain: 0.6, size: 1.1, tone: 44 }));
					tl.sound(a, (w, bus) => audio.punch(bus, w, { gain: 0.4 }));
				} else {
					rise(tl, tc.items, a + 0.15, { stagger: 0.045, dur: 1.1, from: '60%' });
					glint(tl, tEl, a + 1.0, 1.2);
					fadeUp(tl, meta, a + 1.0, { y: 12, dur: 1.0 });
					tl.sound(a - 0.15, (w, bus) => audio.whoosh(bus, w, { dur: 0.8, pan: [0.5, -0.5], gain: 0.05 }));
					tl.sound(a + 1.0, (w, bus) => audio.shimmer(bus, w, { dur: 1.2, gain: 0.03, seed: 9 }));
				}
				tl.add(box, [{ opacity: 1, transform: 'scale(1)' }, { opacity: 0, transform: 'scale(1.03)' }], b - 0.35, 0.35, 'exit');
				break;
			}
			case 'type': {
				if (s.shot) {
					const sp = addShot(a, b, nextShot(), { dissolve: 0, punch: false });
					sp.el.style.opacity = '0.42';
				} else black(a, b);
				const box = centerBox(a, b);
				const line = el('div', 'type-line', box);
				const tc = chars(line, t.tagline[s.line]);
				const caret = el('span', 'caret', line);
				const step = 0.055;
				const startT = a + 0.25;
				// the caret rides just after the last typed character
				const cx = (x) => [{ transform: `translateX(${x}px)` }, { transform: `translateX(${x}px)` }];
				tl.add(caret, cx(0), a, 0.01, 'linear');
				tc.items.forEach((c, i) => {
					const at = startT + i * step + (/[.,]/.test(tc.items[i - 1]?.textContent || '') ? 0.18 : 0);
					tl.add(c, [{ opacity: 0 }, { opacity: 1 }], at, 0.01, 'linear');
					tl.add(caret, cx(c.offsetLeft + c.offsetWidth + 4), at, 0.01, 'linear');
					tl.sound(at, (w, bus) => audio.key(bus, w, { gain: 0.07, pan: (i / tc.items.length) * 0.8 - 0.4, seed: i + 3 }));
				});
				tl.add(caret, [{ opacity: 1 }, { opacity: 1, offset: 0.5 }, { opacity: 0, offset: 0.5 }, { opacity: 0 }], a, 0.6, 'linear', { iterations: Math.ceil((b - a) / 0.6) });
				tl.sound(a, (w, bus, skip) => audio.projector(bus, w, b - a, skip, { gain: 0.05 }), b - a);
				tl.add(box, [{ opacity: 1 }, { opacity: 0 }], b - 0.08, 0.08, 'linear');
				break;
			}
			case 'card': {
				black(a, b);
				const box = centerBox(a, b);
				const inner = el('div', null, box);
				const w = el('div', 'card-word', inner, ph.eyebrow);
				fit(w, 1640);
				const under = el('div', 'card-under', inner);
				tl.add(w, [{ opacity: 0, transform: 'scale(1.35)' }, { opacity: 1, transform: 'scale(1)' }], a, 0.16, 'snap');
				tl.add(under, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], a + 0.12, 0.35, 'snap');
				shake(tl, world, a, { amp: 14, seed: 21 });
				flashAt(a, 0.7);
				tl.sound(a, (w2, bus) => audio.punch(bus, w2, { gain: 0.5 }));
				tl.add(box, [{ opacity: 1 }, { opacity: 0 }], b - 0.06, 0.06, 'linear');
				break;
			}
			case 'shots': {
				shotRun(a, b, true);
				flashAt(a, 0.6);
				tl.sound(a, (w, bus) => audio.punch(bus, w, { gain: 0.45 }));
				break;
			}
			case 'end':
				endCard(a, b);
				break;
		}
	}

	function endCard(a, b) {
		const layer = endLayer;
		tl.show(layer, a - (grit ? 0 : 0.4), D + 1);
		if (!grit) tl.add(layer, [{ opacity: 0 }, { opacity: 1 }], a - 0.4, 0.8, 'sine');
		const bgWrap = el('div', 'fill', layer);
		const bg = css(el('img', 'fill', bgWrap), { width: '100%', height: '100%', objectFit: 'cover', opacity: 0.5 });
		bg.src = t.art.blur;
		css(el('div', 'fill', layer), { background: 'linear-gradient(90deg, rgb(7 8 13 / 0.5), rgb(7 8 13 / 0.86) 55%, rgb(7 8 13 / 0.94))' });
		const gA = glow(layer, 'accent', 450, 540, 640);
		const posterCard = el('div', 'card ec-poster', layer);
		const pImg = el('img', null, posterCard);
		pImg.src = t.art.p;
		const info = el('div', 'ec-info', layer);
		const ebRow = el('div', 'un-eyebrow', info);
		const rail = el('div', 'rail', ebRow);
		rail.style.width = '56px';
		const eb = el('div', 'eyebrow', ebRow, ph.eyebrow);
		const title = words(el('div', 'hero ec-title', info), t.name);
		fit(title.root.parentElement, 980);
		const whenRow = el('div', 'ec-when', info);
		const day = ph.day ? el('span', 'ec-day', whenRow, ph.day) : null;
		const timeEl = el('span', 'ec-time tnum', whenRow, data.when.time);
		timeEl.style.display = 'inline-block';
		const metaRow = el('div', 'ec-meta', info);
		const chipsEl = [String(t.year), ...t.genres, `${t.runtime} min`].map((x) => el('span', 'chip', metaRow, x));

		tl.add(bgWrap, [{ transform: 'scale(1.12)' }, { transform: 'scale(1)' }], a - 0.4, b - a + 0.4, 'out');
		tl.add(gA, [{ opacity: 0 }, { opacity: 0.8 }], a, 1.4, 'sine');
		if (grit) {
			tl.add(lbT, [{ transform: 'translateY(0)' }, { transform: 'translateY(-100%)' }], a + 0.1, 0.7, 'inOut');
			tl.add(lbB, [{ transform: 'translateY(0)' }, { transform: 'translateY(100%)' }], a + 0.1, 0.7, 'inOut');
			tl.add(posterCard, [{ opacity: 0, transform: 'translateX(-60px) rotate(-2deg) scale(1.04)' }, { opacity: 1, transform: 'translateX(0) rotate(0deg) scale(1)' }], a, 0.32, 'snap');
			flashAt(a, 0.6);
			tl.sound(a, (w, bus) => audio.punch(bus, w, { gain: 0.42 }));
		} else {
			tl.add(posterCard, [{ opacity: 0, transform: 'translateY(50px) scale(0.96)' }, { opacity: 1, transform: 'translateY(0) scale(1)' }], a, 1.1, 'enter');
			tl.sound(a - 0.2, (w, bus) => audio.whoosh(bus, w, { dur: 0.9, pan: [-0.7, -0.2], gain: 0.05 }));
		}
		tl.add(pImg, [{ transform: 'scale(1.08)' }, { transform: 'scale(1)' }], a, b - a, 'out');
		const i0 = a + (grit ? 0.25 : 0.45);
		tl.add(rail, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], i0, 0.6, 'enter');
		tl.add(eb, [{ opacity: 0, transform: 'translateX(-14px)' }, { opacity: 1, transform: 'translateX(0)' }], i0 + 0.1, 0.7, 'enter');
		rise(tl, title.items, i0 + 0.25, { stagger: 0.08, dur: grit ? 0.5 : 1.0, ease: grit ? 'snap' : 'enter' });
		const wAt = i0 + (grit ? 0.6 : 0.85);
		if (day) fadeUp(tl, day, wAt, { y: 14 });
		tl.add(timeEl, [{ opacity: 0, transform: 'translateY(24px)' }, { opacity: 1, transform: 'translateY(0)' }], wAt + 0.12, 0.8, 'enter');
		tl.sound(wAt + 0.12, (w, bus) => audio.tick(bus, w, { freq: SPARKLE[1], gain: 0.04, pan: 0.2 }));
		chipsEl.forEach((c, i) => {
			fadeUp(tl, c, wAt + 0.45 + i * 0.07, { y: 10, dur: 0.6 });
			tl.sound(wAt + 0.45 + i * 0.07, (w, bus) => audio.tick(bus, w, { freq: SPARKLE[2 + (i % 4)], gain: 0.018, pan: 0.1 + i * 0.12, decay: 0.04 }));
		});
		if (!grit) glint(tl, timeEl, wAt + 1.1, 0.9);

		const tagAt = a + (grit ? 4.45 : 2.75);
		// after the tape stop the tag plays alone, so it carries more of the weight
		channelTag(tl, layer, tagAt, audio, { gain: grit ? 0.72 : 0.5 });

		// out: everything settles, then black
		const blk = css(el('div', 'fill', stage), { background: '#000' });
		tl.add(blk, [{ opacity: 0 }, { opacity: 1 }], D - 0.75, 0.75, 'sine');

		// the bed: the title's own soundtrack, ducked under the tag (cinematic) or wound
		// down like a stopped deck right before it (grit)
		const bedOpts = grit
			? { fadeIn: 0.05, fadeOut: 0.6, end: D, gain: 1, tapeStop: { at: tagAt - 0.62, dur: 0.55 } }
			: { fadeIn: 0.2, fadeOut: 2.4, end: D, gain: 1, ducks: [{ at: tagAt, depth: 0.42, hold: 1.4, attack: 0.25, release: 1.0 }] };
		tl.sound(0, (w, bus, skip) => audio.bed(bus, w, t.bed, skip, bedOpts), D);
		if (grit) tl.cue('tape stop', tagAt - 0.62);
	}

	return tl;
}

// =========================================================================================
// 5. Strip promo ("Every weekday at 18:00", "Every night at 22:30")
// =========================================================================================

export function stripPromo(stage, data, audio) {
	const S = STR[data.lang];
	const t = data.title;
	const D = data.duration || 16;
	const dark = data.mood === 'dark';
	const ph = phrase(data.when, data.lang);
	const tl = new Timeline(stage, D);
	applyPalette(stage, titlePalette(t));
	stage.classList.add('strip', dark ? 'mood-dark' : 'mood-bright');
	const onsets = audio.onsets(t.bed);

	const gA = glow(stage, 'accent', 380, 300, 760);
	const gV = glow(stage, dark ? 'red' : 'violet', 300, 1000, 640);
	const panel = el('div', 'sp-panel', stage);
	const view = { w: 1180, h: 1080 };
	const shot = framed(panel, t.art.b, t.art.dims.b, view);
	const fr = t.frames.strip;
	tl.add(shot.img, [{ transform: shot.tf(fr[0]) }, { transform: shot.tf(fr[1]) }], 0, D, 'cubic-bezier(0.3, 0.2, 0.6, 1)');
	if (dark) {
		shot.img.style.filter = 'grayscale(0.45) contrast(1.15) brightness(0.62)';
		css(el('div', 'fill', panel), { background: 'radial-gradient(ellipse 70% 80% at 65% 45%, transparent 30%, rgb(10 3 5 / 0.8) 100%), linear-gradient(0deg, rgb(70 4 8 / 0.35), rgb(70 4 8 / 0.35))' });
	} else {
		css(el('div', 'fill', panel), { background: 'linear-gradient(0deg, rgb(7 8 13 / 0.6), transparent 42%), linear-gradient(90deg, rgb(7 8 13 / 0.35), transparent 60%)' });
	}

	const col = el('div', 'sp-col', stage);
	const ebRow = el('div', 'un-eyebrow', col);
	const rail = el('div', 'rail', ebRow);
	rail.style.width = '56px';
	const eb = el('div', 'eyebrow', ebRow, ph.eyebrow);
	const title = words(el('div', 'hero sp-title', col), t.name);
	fit(title.root.parentElement, 960);
	const timeWrap = el('div', 'sp-time tnum', col);
	const tc = chars(el('span', 'mask', timeWrap), data.when.time);
	const daysRow = el('div', 'sp-days', col);
	const lang = data.lang;
	const days = data.when.days;
	const chips = [0, 1, 2, 3, 4, 5, 6].map((d) => {
		const c = el('div', 'day', daysRow);
		const on = el('div', 'on', c);
		el('span', null, c, DAYS[lang].short[d]);
		const lo = el('span', 'lbl-on', c, DAYS[lang].short[d]);
		if (dark) on.style.transformOrigin = '50% 0%';
		return { c, on, lo, d, lit: days.includes(d) };
	});
	let foot;
	if (data.count) {
		foot = setHTML(el('div', 'sp-count', col), S.episodes(0));
	} else if (data.when.from) {
		const fl = fromLine(data.when.from, lang);
		foot = el('div', 'sp-count', col);
		foot.append(fl.pre);
		el('b', 'ink', foot, fl.day);
	}

	// open
	tl.add(gA, [{ opacity: 0 }, { opacity: 1 }], 0, 1.6, 'sine');
	tl.add(gV, [{ opacity: 0 }, { opacity: 0.8 }], 0.2, 1.6, 'sine');
	tl.add(panel, [{ opacity: 0, transform: 'translateX(120px)' }, { opacity: 1, transform: 'translateX(0)' }], 0, 1.4, 'enter');
	tl.cue('eyebrow', 0.45);
	tl.add(rail, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], 0.45, 0.7, 'enter');
	tl.add(eb, [{ opacity: 0, transform: 'translateX(-14px)' }, { opacity: 1, transform: 'translateX(0)' }], 0.55, 0.8, 'enter');
	tl.cue('title', 0.75);
	rise(tl, title.items, 0.75, { stagger: 0.09, dur: 1.0 });
	const timeAt = snapper(onsets)(1.6, 0.4, 0.3);
	tl.cue('time', timeAt);
	rise(tl, tc.items, timeAt, { stagger: 0.06, dur: 0.85, from: '100%', fade: false });
	tl.sound(timeAt, (w, b) => audio.impact(b, w, { gain: dark ? 0.3 : 0.2, size: 0.8, tone: dark ? 40 : 50, click: 0.4 }));

	// days: appear, then light up on the music's beats
	const daysAt = timeAt + 0.9;
	chips.forEach((ch, i) => fadeUp(tl, ch.c, daysAt + i * 0.05, { y: 14, dur: 0.6 }));
	const lit = chips.filter((c) => c.lit);
	const beats = beatsIn(onsets, daysAt + 0.5, D, dark ? 0.42 : 0.25, 0.3, 0);
	let prev = daysAt + 0.4;
	const litTimes = lit.map((_, i) => {
		const tt = beats.find((x) => x > prev + (dark ? 0.4 : 0.22) && x < prev + 0.9) ?? prev + (dark ? 0.5 : 0.32);
		prev = tt;
		return tt;
	});
	tl.cue('days light up', litTimes[0]);
	lit.forEach((ch, i) => {
		const at = litTimes[i];
		tl.add(ch.on, [{ transform: 'scaleY(0)' }, { transform: 'scaleY(1)' }], at, dark ? 0.5 : 0.32, dark ? 'cubic-bezier(0.5, 0, 0.2, 1)' : 'pop');
		tl.add(ch.lo, [{ opacity: 0 }, { opacity: 1 }], at + 0.04, 0.2, 'linear');
		tl.add(ch.c, [{ transform: 'scale(1)' }, { transform: 'scale(1.1)' }, { transform: 'scale(1)' }], at, 0.36, 'out');
		tl.sound(at, (w, b) => audio.tick(b, w, { freq: dark ? SPARKLE[[0, 2, 1, 3, 2, 4, 0][i]] / 2 : SPARKLE[i], gain: dark ? 0.06 : 0.05, pan: -0.6 + i * 0.12, decay: dark ? 0.16 : 0.07, type: dark ? 'triangle' : 'sine' }));
	});
	const afterDays = litTimes[litTimes.length - 1] + 0.55;

	if (data.when.from) {
		const startChip = chips.find((c) => c.d === data.when.from.day);
		const ring = el('div', 'start-ring', startChip.c);
		startChip.c.style.overflow = 'visible';
		startChip.on.style.borderRadius = '18px';
		tl.add(ring, [{ opacity: 0, transform: 'scale(1.3)' }, { opacity: 1, transform: 'scale(1)' }], afterDays, 0.6, 'enter');
		tl.add(ring, [{ opacity: 1 }, { opacity: 0.35 }, { opacity: 1 }], afterDays + 0.6, 1.4, 'sine', { iterations: Math.ceil((D - afterDays) / 1.4) });
		tl.sound(afterDays, (w, b) => audio.tick(b, w, { freq: SPARKLE[0] / 2, gain: 0.06, decay: 0.25, type: 'triangle' }));
	}
	if (foot) {
		tl.cue(data.count ? 'episode counter' : 'season line', afterDays + 0.15);
		fadeUp(tl, foot, afterDays + 0.15, { y: 14 });
		if (data.count) {
			const n = data.count;
			const c0 = afterDays + 0.2;
			const cd = 1.5;
			let lastV = -1;
			tl.tick((tt) => {
				const p = Math.min(1, Math.max(0, (tt - c0) / cd));
				const v = Math.round(n * (1 - Math.pow(1 - p, 3)));
				if (v !== lastV) setHTML(foot, S.episodes((lastV = v)));
			});
			// a rolling counter: ticks that slow down with the ease-out
			for (let i = 0; i < 12; i++) {
				const p = i / 12;
				const at = c0 + cd * (1 - Math.pow(1 - p, 1 / 3)) * 0.98;
				tl.sound(at, (w, b) => audio.tick(b, w, { freq: 3200 - i * 90, gain: 0.018, decay: 0.02, pan: 0.3 }));
			}
		}
	}

	// tag and out
	const tagAt = D - 4.0;
	channelTag(tl, stage, tagAt, audio);
	const blk = css(el('div', 'fill', stage), { background: '#000' });
	tl.add(blk, [{ opacity: 0 }, { opacity: 1 }], D - 0.8, 0.8, 'sine');
	tl.sound(0, (w, b, skip) => audio.bed(b, w, t.bed, skip, { from: data.bedFrom || 0, fadeIn: 0.3, fadeOut: 2.2, end: D, gain: 1, ducks: [{ at: tagAt, depth: 0.45, hold: 1.4, attack: 0.25 }] }), D);
	tl.sound(0, (w, b) => audio.whoosh(b, w, { dur: 1.2, pan: [0.8, 0.2], gain: 0.045 }));
	return tl;
}

// =========================================================================================
// 6. Short promo ("Tomorrow 21:00 INVINCIBLE")
// =========================================================================================

export function shortPromo(stage, data, audio) {
	const t = data.title;
	const D = data.duration || 9;
	const ph = phrase(data.when, data.lang);
	const tl = new Timeline(stage, D);
	const pal = titlePalette(t);
	applyPalette(stage, pal);
	const deep = pal.base === t.accents.backdrop ? pal.dark : t.accents.backdrop;
	stage.style.setProperty('--shadow-rgb', deep.match(/\w\w/g).map((h) => parseInt(h, 16)).join(' '));
	stage.classList.add('short');

	// the sting's hit: the strongest onset in its first two seconds
	const hit = audio.onsets(t.bed).filter((o) => o.t > 0.5 && o.t < 2.2).sort((a, b) => b.s - a.s)[0]?.t ?? 1.1;

	const world = el('div', 'fill', stage);
	const shot = framed(world, t.art.b, t.art.dims.b);
	const fr = t.frames.short;
	css(el('div', 'fill', world), { background: `linear-gradient(0deg, rgb(7 8 13 / 0.75), rgb(7 8 13 / 0.35) 50%, rgb(7 8 13 / 0.6))` });
	const rays = el('div', 'rays', world);
	const htA = el('div', 'halftone', world);
	const htB = el('div', 'halftone tl', world);
	const burst = el('div', 'burst', world);
	const c = el('div', 'center', world);
	const stack = el('div', null, c);
	css(stack, { display: 'flex', flexDirection: 'column', alignItems: 'center' });
	const whenRow = el('div', 'sh-when', stack);
	const ebMask = el('span', 'mask', whenRow);
	const eb = el('span', 'eyebrow', ebMask, ph.eyebrow);
	const timeP = el('span', 'sh-time tnum', whenRow, data.when.time);
	const word = el('div', 'sh-word', stack, t.name);
	fit(word, 1680);
	const flash = el('div', 'flash', stage);
	grain(tl, stage, 0.05, 17);

	tl.add(shot.img, [{ transform: shot.tf(fr[0], 1.5) }, { transform: shot.tf(fr[0], 1.12) }], 0, hit, 'cubic-bezier(0.55, 0, 1, 0.45)');
	tl.add(shot.img, [{ transform: shot.tf(fr[0], 1.12) }, { transform: shot.tf(fr[0], 1.04) }], hit, 0.25, 'snap');
	tl.add(shot.img, [{ transform: shot.tf(fr[0], 1.04) }, { transform: shot.tf(fr[1]) }], hit + 0.25, D - hit - 0.25, 'out');
	tl.add(shot.el, [{ opacity: 0 }, { opacity: 1 }], 0, 0.5, 'sine');
	tl.add(rays, [{ transform: 'rotate(0deg) scale(0.8)', opacity: 0 }, { transform: 'rotate(12deg) scale(1)', opacity: 1 }], 0, hit, 'linear');
	tl.add(rays, [{ transform: 'rotate(12deg) scale(1)', opacity: 1 }, { transform: 'rotate(40deg) scale(1.08)', opacity: 0.7 }], hit, D - hit, 'out');

	tl.cue('the hit', hit);
	tl.add(flash, [{ opacity: 0.9 }, { opacity: 0 }], hit, 0.16, 'linear', { fill: 'forwards' });
	tl.add(word, [{ opacity: 0, transform: 'scale(2.3)' }, { opacity: 1, transform: 'scale(1)' }], hit, 0.24, 'snap');
	shake(tl, world, hit, { amp: 26, dur: 0.5, seed: 77 });
	tl.add(burst, [{ opacity: 1, transform: 'scale(0.35)' }, { opacity: 0, transform: 'scale(1.7)' }], hit, 0.8, 'out');
	tl.add(htA, [{ opacity: 0 }, { opacity: 1 }], hit, 0.25, 'linear');
	tl.add(htB, [{ opacity: 0 }, { opacity: 0.8 }], hit + 0.05, 0.25, 'linear');

	const wAt = hit + 0.85;
	tl.cue('when', wAt);
	tl.add(eb, [{ transform: 'translateY(110%)' }, { transform: 'translateY(0)' }], wAt, 0.5, 'snap');
	tl.add(timeP, [{ opacity: 0, transform: 'scale(0.4) rotate(-6deg)' }, { opacity: 1, transform: 'scale(1) rotate(0deg)' }], wAt + 0.22, 0.42, 'pop');
	tl.sound(wAt, (w, b) => audio.whoosh(b, w, { dur: 0.45, pan: [-0.6, 0], gain: 0.05, lo: 700, hi: 4200, body: 0.2 }));
	tl.sound(wAt + 0.22, (w, b) => audio.punch(b, w, { gain: 0.25, pan: 0.2 }));

	const tagAt = Math.min(D - 3.3, hit + 3.6);
	tl.add(stack, [{ transform: 'translateY(0) scale(1)' }, { transform: 'translateY(-60px) scale(0.9)' }], tagAt - 0.5, 0.9, 'inOut');
	channelTag(tl, stage, tagAt, audio);
	tl.add(world, [{ opacity: 1 }, { opacity: 1 }], 0, 0.01, 'linear');
	tl.add(world, [{ opacity: 1 }, { opacity: 0 }], D - 0.55, 0.55, 'exit');
	tl.add(stack, [{ opacity: 1 }, { opacity: 0 }], D - 0.6, 0.4, 'exit');
	const tagEl = stage.querySelector('.tag-lockup');
	tl.add(tagEl, [{ opacity: 1 }, { opacity: 0 }], D - 0.45, 0.45, 'exit');

	tl.sound(0, (w, b, skip) => audio.bed(b, w, t.bed, skip, { fadeIn: 0.02, fadeOut: 1.6, end: D, gain: 1, ducks: [{ at: tagAt, depth: 0.55, hold: 1.2, attack: 0.2 }] }), D);
	tl.sound(0.1, (w, b) => audio.riser(b, w, { dur: hit - 0.1, gain: 0.05, tone: 0.3 }));
	tl.sound(hit, (w, b) => audio.impact(b, w, { gain: 0.38, size: 1.3, tone: 42 }));
	return tl;
}

// =========================================================================================
// 7. "Later on Couch TV" lineup
// =========================================================================================

export function lineup(stage, data, audio) {
	const S = STR[data.lang];
	const D = data.duration || 12;
	const tl = new Timeline(stage, D);
	stage.classList.add('lineup');
	const items = data.items;
	const now = data.now;
	const toMin = (hhmm) => {
		const [h, m] = hhmm.split(':').map(Number);
		return h * 60 + m;
	};
	const nowM = toMin(now);
	items.forEach((it, i) => {
		const start = toMin(it.time);
		const end = i + 1 < items.length ? toMin(items[i + 1].time) : start + 60;
		it.state = nowM >= start && nowM < end ? 'now' : '';
		it.progress = it.state === 'now' ? (nowM - start) / (end - start) : 0;
	});
	const firstFuture = items.find((it) => toMin(it.time) > nowM);
	if (!items.some((it) => it.state === 'now') && firstFuture) firstFuture.state = 'next';

	const gV = glow(stage, 'violet', 280, 180, 760);
	const gT = glow(stage, 'teal', 760, 1100, 700);
	const gR = glow(stage, 'red', 1500, 300, 700);
	const panel = el('div', 'lu-panel', stage);
	const view = { w: 1100, h: 1080 };
	const shots = items.map((it) => {
		const s = framed(panel, it.title.art.b, it.title.art.dims.b, view);
		s.el.style.background = 'transparent';
		return s;
	});
	el('div', 'shade', panel);
	const head = el('div', 'lu-head', stage);
	const lk = lockupStatic(head, 56, 'lu-wm');
	const title = words(el('div', 'hero lu-title', head), S.laterOn);
	const list = el('div', 'lu-list', stage);
	const hl = css(el('div', 'abs'), { left: 0, top: 0, right: 0, height: '140px', borderRadius: '22px', background: 'rgb(255 255 255 / 0.06)', border: '2px solid rgb(255 255 255 / 0.18)' });
	list.appendChild(hl);
	const rows = items.map((it) => {
		const r = el('div', 'row', list);
		const bg = el('div', 'row-bg', r);
		el('div', 'row-time tnum', r, it.time);
		const pw = el('div', 'row-poster', r);
		const pi = el('img', null, pw);
		pi.src = it.title.art.p;
		const tx = el('div', 'row-text', r);
		el('div', 'row-name', tx, it.title.name);
		el('div', 'row-sub', tx, it.sub);
		let pill = null;
		let fill = null;
		if (it.state === 'now') {
			el('div', 'row-bar', r);
			const pr = el('div', 'row-prog', tx);
			fill = el('i', null, pr);
			pill = el('div', 'pill-live', r);
			el('span', 'live-dot', pill);
			el('span', null, pill, S.live);
		} else if (it.state === 'next') {
			pill = el('div', 'pill-next', r, `${S.next} · ${S.inMin(toMin(it.time) - nowM)}`);
		}
		return { r, bg, pw, pill, fill, it };
	});
	const spot = el('div', 'lu-spot', stage);
	const captions = items.map((it) => {
		const cpt = el('div', 'abs', spot);
		css(cpt, { right: 0, bottom: 0, textAlign: 'right' });
		el('div', 'eyebrow', cpt, it.time);
		el('div', 'hero lu-spot-name', cpt, it.title.name);
		return cpt;
	});

	// open
	for (const g of [gV, gT, gR]) tl.add(g, [{ opacity: 0, transform: 'scale(0.85)' }, { opacity: 1, transform: 'scale(1)' }], 0, 1.8, 'out');
	tl.cue('header', 0.3);
	tl.add(lk.root, [{ opacity: 0, transform: 'translateY(14px)' }, { opacity: 1, transform: 'translateY(0)' }], 0.3, 0.9, 'enter');
	rise(tl, title.items, 0.5, { stagger: 0.09, dur: 1.0 });
	tl.add(panel, [{ opacity: 0 }, { opacity: 1 }], 0.4, 1.4, 'sine');

	const rowAt = 1.0;
	tl.cue('rows slide in', rowAt);
	rows.forEach((row, i) => {
		const at = rowAt + i * 0.22;
		tl.add(row.r, [{ opacity: 0, transform: 'translateX(90px)' }, { opacity: 1, transform: 'translateX(0)' }], at, 0.9, 'enter');
		tl.add(row.pw, [{ transform: 'scale(1.15)' }, { transform: 'scale(1)' }], at, 1.1, 'enter');
		tl.sound(at, (w, b) => audio.tick(b, w, { freq: SPARKLE[[0, 2, 3, 5][i % 4]], gain: 0.035, pan: 0.3 - i * 0.1, decay: 0.08 }));
		if (row.pill) {
			const pa = rowAt + 1.3 + i * 0.05;
			tl.add(row.pill, [{ opacity: 0, transform: 'scale(0.6)' }, { opacity: 1, transform: 'scale(1)' }], pa, 0.5, 'pop');
			tl.sound(pa, (w, b) => audio.tick(b, w, { freq: SPARKLE[4], gain: 0.04, pan: 0.5 }));
			const dot = row.pill.querySelector('.live-dot');
			if (dot) tl.add(dot, [{ opacity: 1 }, { opacity: 0.3 }, { opacity: 1 }], pa + 0.5, 1.4, 'sine', { iterations: Math.ceil(D / 1.4) });
		}
		if (row.fill) tl.add(row.fill, [{ transform: 'scaleX(0)' }, { transform: `scaleX(${row.it.progress.toFixed(3)})` }], rowAt + 1.5, 1.2, 'enter');
	});

	// spotlight: each programme in turn on the big panel
	const s0 = 3.3;
	const step = 1.75;
	tl.cue('spotlight', s0);
	tl.add(hl, [{ opacity: 0, transform: 'translateY(0)' }, { opacity: 1, transform: 'translateY(0)' }], s0 - 0.2, 0.5, 'sine');
	items.forEach((it, i) => {
		const at = s0 + i * step;
		const nextAt = i + 1 < items.length ? s0 + (i + 1) * step : D;
		const sh = shots[i];
		tl.show(sh.el, i === 0 ? 0 : at - 0.6, i + 1 < items.length ? nextAt + 0.2 : D + 1);
		if (i > 0) tl.add(sh.el, [{ opacity: 0 }, { opacity: 1 }], at - 0.6, 0.8, 'sine');
		const fr = it.title.frames.lineup || [0.1, 0.05, 0.85, 0.9];
		tl.add(sh.img, [{ transform: sh.tf(fr, 1.08) }, { transform: sh.tf(fr, 1) }], i === 0 ? 0 : at - 0.6, nextAt - at + 1.4, 'cubic-bezier(0.2, 0.3, 0.4, 1)');
		if (i > 0) {
			tl.add(hl, [{ transform: `translateY(${(i - 1) * 152}px)` }, { transform: `translateY(${i * 152}px)` }], at, 0.6, 'cubic-bezier(0.6, 0, 0.2, 1)');
			tl.sound(at, (w, b) => audio.tick(b, w, { freq: SPARKLE[i + 1], gain: 0.025, pan: 0.2, decay: 0.12 }));
		}
		const cp = captions[i];
		tl.add(cp, [{ opacity: 0, transform: 'translateY(20px)' }, { opacity: 1, transform: 'translateY(0)' }], at + 0.1, 0.7, 'enter');
		if (i + 1 < items.length) fadeOut(tl, cp, nextAt - 0.25, { dur: 0.3, y: -14 });
	});

	// out
	const out = D - 1.2;
	tl.cue('out', out);
	rows.forEach((row, i) => tl.add(row.r, [{ opacity: 1, transform: 'translateX(0)' }, { opacity: 0, transform: 'translateX(-50px)' }], out + i * 0.06, 0.45, 'exit'));
	fadeOut(tl, head, out, { dur: 0.5 });
	fadeOut(tl, hl, out, { dur: 0.4, y: 0 });
	fadeOut(tl, spot, out + 0.1, { dur: 0.4, y: 0 });
	tl.add(panel, [{ opacity: 1 }, { opacity: 0 }], out + 0.2, 0.9, 'sine');
	for (const g of [gV, gT, gR]) tl.add(g, [{ opacity: 1 }, { opacity: 0 }], out + 0.2, 1.0, 'sine');

	tl.sound(0, (w, b, skip) => audio.ambient(b, w, D, skip, { gain: 0.085 }), D);
	tl.sound(0.15, (w, b) => audio.shimmer(b, w, { dur: 1.4, gain: 0.018, count: 8, seed: 31 }));
	return tl;
}

// =========================================================================================
// 8. On-program overlays (corner bug, "Now" and "Next" lower thirds)
// =========================================================================================

export function overlays(stage, data, audio) {
	const S = STR[data.lang];
	const D = data.duration || 15;
	const t = data.title;
	const tl = new Timeline(stage, D);
	applyPalette(stage, titlePalette(t));
	stage.classList.add('overlays');

	// stand-in for the programme's video
	const shot = framed(stage, t.art.b, t.art.dims.b);
	const fr = t.frames.overlays;
	tl.add(shot.img, [{ transform: shot.tf(fr[0]) }, { transform: shot.tf(fr[1]) }], 0, 7.4, 'linear');
	tl.add(shot.img, [{ transform: shot.tf(fr[2]) }, { transform: shot.tf(fr[3]) }], 7.4, D - 7.4, 'linear');
	const jump = css(el('div', 'fill', stage), { background: '#000', opacity: 0 });

	const startClock = data.start;
	const jumpAt = 7.4;
	const jumpMin = data.jumpMinutes;
	const b = bug(tl, stage, (tt) => addMinutes(startClock, Math.floor((tt < jumpAt ? tt : tt + jumpMin * 60) / 60) + 1), data.lang);
	tl.cue('bug', 0.4);
	tl.add(b, [{ opacity: 0, transform: 'translateY(-10px)' }, { opacity: 0.95, transform: 'translateY(0)' }], 0.4, 0.8, 'enter');
	// a bug that sits over a film for an hour backs off after its entrance
	tl.add(b, [{ opacity: 0.95 }, { opacity: 0.78 }], 4.6, 1.5, 'sine');

	// "Now" lower third
	const nowL3 = el('div', 'l3', stage);
	const bar = el('div', 'l3-bar', nowL3);
	const clip = el('div', 'l3-clip', nowL3);
	const card = el('div', 'l3-card', clip);
	const nEb = el('div', 'eyebrow', card, S.now);
	const nTitle = el('div', 'l3-title', card, t.name);
	const nSub = el('div', 'l3-sub', card);
	nSub.append(`${data.episode.code} · ${data.episode.name}`);
	el('span', 'dot-sep tnum', nSub, `${data.slot[0]}–${data.slot[1]}`);
	const prog = el('div', 'l3-prog', card);
	const pf = el('i', null, prog);

	const n0 = 1.3;
	tl.cue('now lower third', n0);
	tl.add(bar, [{ transform: 'scaleY(0)' }, { transform: 'scaleY(1)' }], n0, 0.45, 'enter');
	tl.add(card, [{ transform: 'translateX(-104%)' }, { transform: 'translateX(0)' }], n0 + 0.12, 0.8, 'enter');
	for (const [e, d] of [[nEb, 0.3], [nTitle, 0.38], [nSub, 0.48]]) tl.add(e, [{ opacity: 0, transform: 'translateX(-16px)' }, { opacity: 1, transform: 'translateX(0)' }], n0 + d, 0.7, 'enter');
	tl.add(pf, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(0.02)' }], n0 + 0.6, 0.6, 'enter');
	const nOut = 6.4;
	tl.add(card, [{ transform: 'translateX(0)' }, { transform: 'translateX(-104%)' }], nOut, 0.55, 'cubic-bezier(0.6, 0, 0.8, 0.4)');
	tl.add(bar, [{ transform: 'scaleY(1)' }, { transform: 'scaleY(0)' }], nOut + 0.4, 0.3, 'exit');

	// time passes: a dip, the clock jumps
	tl.cue(`${jumpMin} minutes later`, jumpAt);
	tl.add(jump, [{ opacity: 0 }, { opacity: 0.85, offset: 0.5 }, { opacity: 0 }], jumpAt - 0.35, 0.7, 'sine');

	// "Next" lower third, in the next programme's colour
	const nx = data.next;
	const nextL3 = el('div', 'l3', stage);
	applyPalette(nextL3, titlePalette(nx.title));
	const bar2 = el('div', 'l3-bar', nextL3);
	const clip2 = el('div', 'l3-clip', nextL3);
	const card2 = el('div', 'l3-card l3-next', clip2);
	const pw = el('div', 'row-poster', card2);
	const pi = el('img', null, pw);
	pi.src = nx.title.art.p;
	const tx = el('div', null, card2);
	const eb2 = el('div', 'eyebrow', tx, `${S.next} · ${nx.time}`);
	const t2 = el('div', 'l3-title', tx, nx.title.name);
	const s2 = el('div', 'l3-sub', tx, nx.sub);

	const x0 = 8.8;
	tl.cue('next lower third', x0);
	tl.add(bar2, [{ transform: 'scaleY(0)' }, { transform: 'scaleY(1)' }], x0, 0.45, 'enter');
	tl.add(card2, [{ transform: 'translateX(-104%)' }, { transform: 'translateX(0)' }], x0 + 0.12, 0.8, 'enter');
	tl.add(pw, [{ opacity: 0, transform: 'scale(0.8)' }, { opacity: 1, transform: 'scale(1)' }], x0 + 0.35, 0.6, 'pop');
	for (const [e, d] of [[eb2, 0.36], [t2, 0.44], [s2, 0.54]]) tl.add(e, [{ opacity: 0, transform: 'translateX(-16px)' }, { opacity: 1, transform: 'translateX(0)' }], x0 + d, 0.7, 'enter');
	const xOut = 13.6;
	tl.add(card2, [{ transform: 'translateX(0)' }, { transform: 'translateX(-104%)' }], xOut, 0.55, 'cubic-bezier(0.6, 0, 0.8, 0.4)');
	tl.add(bar2, [{ transform: 'scaleY(1)' }, { transform: 'scaleY(0)' }], xOut + 0.4, 0.3, 'exit');

	// the programme's own sound stands in for the movie; overlays add none by design
	tl.sound(0, (w, bus, skip) => audio.bed(bus, w, t.bed, skip, { from: 4, fadeIn: 0.6, fadeOut: 1.2, end: jumpAt + 0.1, gain: 0.7 }), jumpAt);
	tl.sound(jumpAt, (w, bus, skip) => audio.bed(bus, w, t.bed, skip, { from: 20, fadeIn: 0.35, fadeOut: 1.2, end: D - jumpAt, gain: 0.7 }), D - jumpAt);
	return tl;
}

// =========================================================================================
// 9. Credits squeeze
// =========================================================================================

export function creditsSqueeze(stage, data, audio) {
	const S = STR[data.lang];
	const handover = data.handover || 'expand';
	const D = handover === 'cursor' ? 10.6 : 11.5;
	const tl = new Timeline(stage, D);
	stage.classList.add('squeeze');
	const end = data.ending;
	const nx = data.next;
	const nxPal = titlePalette(nx.title);
	applyPalette(stage, nxPal);

	const gV = glow(stage, 'violet', 400, 160, 760);
	const gT = glow(stage, 'teal', 700, 1100, 720);
	const gA = glow(stage, 'accent', 1500, 560, 720);

	// the ending programme: a still and rolling credits stand in for its video
	const prog = el('div', 'program', stage);
	const shot = framed(prog, end.art.b, end.art.dims.b);
	el('div', 'dim', prog);
	const credits = el('div', 'credits', prog);
	for (const [role, name] of data.credits) {
		el('div', 'role', credits, role);
		el('div', 'name', credits, name);
	}
	const redFill = el('div', 'red-fill', prog);
	const frame = css(el('div', 'sq-frame', stage), { left: '96px', top: '236px', width: '1008px', height: '567px' });

	const col = el('div', 'sq-col', stage);
	const ebRow = el('div', 'un-eyebrow', col);
	ebRow.style.marginBottom = '0';
	const rail = el('div', 'rail', ebRow);
	rail.style.width = '48px';
	const eb = el('div', 'eyebrow', ebRow, S.upNext);
	const slot = el('div', 'sq-card-slot', col);
	const title = words(el('div', 'hero sq-title', col), nx.title.name);
	fit(title.root.parentElement, 648);
	const epRow = el('div', 'sq-ep', col);
	const chip = el('span', 'chip solid', epRow, nx.episode.code);
	const epName = el('span', null, epRow, nx.episode.name);
	const whenEl = el('div', 'sq-when', col);
	el('b', 'tnum', whenEl, nx.time);
	whenEl.append(` · ${S.afterBreak}`);
	const b = bug(tl, stage, () => data.clock, data.lang);

	// the next programme's picture lives at stage level so it can expand to full frame
	let slotX = 0;
	let slotY = 0;
	for (let e = slot; e && e !== stage; e = e.offsetParent) {
		slotX += e.offsetLeft;
		slotY += e.offsetTop;
	}
	const k = 648 / W;
	const tvEl = el('div', 'sq-tv', stage);
	const nshot = framed(tvEl, nx.title.art.b, nx.title.art.dims.b);
	const slotTf = `translate(${slotX}px, ${slotY}px) scale(${k})`;
	tvEl.style.transform = slotTf;

	const full = 'translate(-48px, -27px) scale(1.05)';
	const sq = 'translate(96px, 236px) scale(0.525)';
	const s0 = 2.0;
	const s1 = 3.3;
	tl.cue('programme ends (credits)', 0);
	tl.add(prog, [{ transform: full }, { transform: full }], 0, 0.01, 'linear');
	tl.add(credits, [{ transform: 'translateY(420px)' }, { transform: `translateY(${420 - 70 * D}px)` }], 0, D, 'linear');
	tl.add(shot.img, [{ transform: shot.tf([0, 0, 1, 1]) }, { transform: shot.tf([0.05, 0.05, 0.9, 0.9]) }], 0, D, 'linear');
	tl.cue('squeeze', s0);
	tl.add(prog, [{ transform: full }, { transform: sq }], s0, s1 - s0, 'cubic-bezier(0.65, 0, 0.25, 1)');
	tl.add(frame, [{ opacity: 0 }, { opacity: 1 }], s1 - 0.2, 0.5, 'sine');
	for (const g of [gV, gT, gA]) tl.add(g, [{ opacity: 0 }, { opacity: 1 }], s0 + 0.3, 1.6, 'sine');
	fadeUp(tl, b, s0 + 0.6, { y: -10 });

	const c0 = s0 + 0.75;
	tl.cue('up next panel', c0);
	tl.add(rail, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], c0, 0.7, 'enter');
	tl.add(eb, [{ opacity: 0, transform: 'translateX(-14px)' }, { opacity: 1, transform: 'translateX(0)' }], c0 + 0.1, 0.7, 'enter');
	tl.add(tvEl, [{ opacity: 0 }, { opacity: 1 }], c0 + 0.2, 0.8, 'sine');
	tl.add(nshot.el, [{ transform: 'scale(1.1)' }, { transform: 'scale(1)' }], c0 + 0.2, 1.4, 'enter');
	tl.add(nshot.img, [{ transform: nshot.tf([0, 0, 1, 1]) }, { transform: nshot.tf([0.06, 0.04, 0.9, 0.9]) }], c0, D - c0, 'linear');
	rise(tl, title.items, c0 + 0.45, { stagger: 0.08, dur: 0.95 });
	tl.add(chip, [{ opacity: 0, transform: 'scale(0.7)' }, { opacity: 1, transform: 'scale(1)' }], c0 + 0.8, 0.5, 'pop');
	tl.add(epName, [{ opacity: 0, transform: 'translateX(-16px)' }, { opacity: 1, transform: 'translateX(0)' }], c0 + 0.9, 0.7, 'enter');
	fadeUp(tl, whenEl, c0 + 1.05, { y: 10 });

	const h0 = 9.2;
	tl.cue(handover === 'cursor' ? "handover: collapses into the ident's cursor" : 'handover: next programme expands', h0);
	[ebRow, title.root.parentElement, epRow, whenEl].forEach((e, i) => fadeOut(tl, e, h0 + i * 0.05, { dur: 0.4, y: -10 }));
	tl.add(frame, [{ opacity: 1 }, { opacity: 0 }], h0, 0.3, 'sine');
	if (handover === 'cursor') {
		// the programme collapses into the red cursor block the short ident opens on (its first frame)
		const r = identCursorRect(stage, data.variant);
		// the ident's field is pure black: the stage settles into it under the collapse
		const black = css(el('div', 'fill', stage), { background: '#000' });
		stage.insertBefore(black, prog);
		tl.add(black, [{ opacity: 0 }, { opacity: 1 }], h0, 1.1, 'sine');
		tl.add(tvEl, [{ opacity: 1 }, { opacity: 0 }], h0, 0.45, 'sine');
		fadeOut(tl, b, h0, { dur: 0.4, y: -10 });
		const block = `translate(${r.x.toFixed(1)}px, ${r.y.toFixed(1)}px) scale(${(r.w / W).toFixed(5)}, ${(r.h / H).toFixed(5)})`;
		tl.add(prog, [{ transform: sq }, { transform: block }], h0 + 0.2, 0.9, 'cubic-bezier(0.7, 0, 0.2, 1)');
		tl.add(redFill, [{ opacity: 0 }, { opacity: 1 }], h0 + 0.4, 0.5, 'in');
		for (const g of [gV, gT, gA]) tl.add(g, [{ opacity: 1 }, { opacity: 0 }], h0, 1.2, 'sine');
		tl.sound(h0 + 0.2, (w, bus) => audio.whoosh(bus, w, { dur: 0.9, pan: [-0.5, 0], gain: 0.05 }));
		tl.sound(h0 + 1.1, (w, bus) => audio.keyClick(bus, w, { gain: 0.05, seed: 7 }));
	} else {
		tl.add(prog, [{ transform: sq, opacity: 1 }, { transform: 'translate(-40px, 236px) scale(0.525)', opacity: 0 }], h0, 0.8, 'cubic-bezier(0.6, 0, 0.8, 0.5)');
		tl.add(tvEl, [{ transform: slotTf }, { transform: 'translate(-57.6px, -32.4px) scale(1.06)' }], h0 + 0.35, 1.25, 'cubic-bezier(0.7, 0, 0.2, 1)');
		tl.sound(h0 + 0.3, (w, bus) => audio.whoosh(bus, w, { dur: 1.2, pan: [0.6, -0.1], gain: 0.07 }));
		tl.sound(h0 + 1.5, (w, bus) => audio.impact(bus, w, { gain: 0.15, size: 0.7, click: 0.2 }));
	}

	// the credits music keeps playing, ducked while "Up next" talks over it
	tl.sound(0, (w, bus, skip) => audio.bed(bus, w, end.bed, skip, { from: data.bedFrom || 0, fadeIn: 0.4, fadeOut: 1.4, end: h0 + 1.5, gain: 1, ducks: [{ at: s0 + 0.5, depth: 0.42, hold: h0 - s0 - 0.5, attack: 0.7, release: 0.01 }] }), h0 + 1.5);
	tl.sound(s0, (w, bus) => audio.whoosh(bus, w, { dur: 1.3, pan: [0.4, -0.5], gain: 0.05, lo: 300, hi: 1800 }));
	tl.sound(c0, (w, bus) => audio.tick(bus, w, { freq: SPARKLE[1], gain: 0.04, pan: 0.5 }));
	tl.sound(c0 + 0.2, (w, bus) => audio.shimmer(bus, w, { dur: 0.9, gain: 0.018, count: 8, seed: 41 }));
	tl.sound(c0 + 0.8, (w, bus) => audio.tick(bus, w, { freq: SPARKLE[3], gain: 0.035, pan: 0.55 }));
	return tl;
}
