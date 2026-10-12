// The Couch TV ident in two variants, each with a full and a short cut. Everything comes out
// of the wordmark and goes back into it: the O's brackets open, frame what is on, and snap shut
// on the hit; the hook's long note drops TV into its red block.
//
//   A "typed": a cursor types the name, selects the O and types brackets over it; they open in
//     the line and frame a beat-cut sequence (symbols, words, a time, tiny posters, the couch).
//   B "frame": the brackets leave the word and frame the whole screen, cutting on the beat over
//     flat black, red and white fields, then slam back into the O.
//
// Picture and score share theme.js's GRID (120 bpm), so the cue tables below are in beats'
// seconds and the theme is chosen independently (data.theme).

import { Timeline, el, css, chars, motion, windows, minus } from './core.js';
import { wordmark, geometry, cursorAt, centre } from './brand.js';
import { GRID, HOOK } from './theme.js';
import { SPARKLE } from './audio.js';

const SIZE = 190; // the lockup's font size on the 1920x1080 stage

const COPY = {
	en: { on: 'ON', now: 'NOW', movies: 'MOVIES', series: 'SERIES', tagline: 'One channel. Every couch.' },
	cs: { on: 'TEĎ', now: 'ŽIVĚ', movies: 'FILMY', series: 'SERIÁLY', tagline: 'Jeden kanál. Každý gauč.' }
};

// Cue tables. A sequence entry is [time, item] (A) or [time, field, item] (B).
const CUTS = {
	a: {
		full: {
			typed: 'COUCH TV',
			type: { at: 0.5, step: 0.25 },
			blink: [[0, 0.25]],
			select: 2.75,
			replace: [3.0, 3.125],
			seq: [[3.5, 'play'], [4.0, 'on'], [4.5, 'now'], [5.0, 'time'], [5.125, 'poster:ironman'], [5.25, 'star'], [5.375, 'poster:pulp'], [5.5, 'couch']]
		},
		short: {
			typed: 'C()UCH',
			type: { at: 0.125, step: 0.125 },
			blink: [],
			seq: [[1.0, 'on'], [1.25, 'time'], [1.5, 'poster:ironman'], [1.625, 'star'], [1.75, 'couch']]
		}
	},
	b: {
		full: {
			intro: [0, 1.0],
			seq: [
				[1.0, 'black', 'on'],
				[1.5, 'red', 'time'],
				[2.0, 'white', 'poster:ironman'],
				[2.5, 'black', 'star'],
				[3.0, 'black', 'now'],
				[3.5, 'red', 'play'],
				[4.0, 'white', 'poster:pulp'],
				[4.5, 'black', 'movies'],
				[4.75, 'black', 'series'],
				[5.0, 'black', 'rgb'],
				[5.125, 'black', 'poster:simpsons'],
				[5.25, 'black', 'time'],
				[5.375, 'black', 'poster:dexter'],
				[5.5, 'black', 'couch']
			]
		},
		short: {
			cursor: [0, 0.25],
			seq: [
				[0.5, 'black', 'on'],
				[1.0, 'red', 'time'],
				[1.25, 'black', 'poster:ironman'],
				[1.5, 'black', 'star'],
				[1.625, 'black', 'rgb'],
				[1.75, 'black', 'couch']
			]
		}
	}
};

/** The red block the short cut opens on, in stage px: the squeeze collapses into it. */
export function identCursorRect(parent, variant) {
	const probe = el('div', 'wm-probe', parent);
	const wm = wordmark(probe, SIZE, { cls: 'wm-abs' });
	const m = wm.m;
	const left = 960 - m.width / 2;
	const top = 540 - m.cy;
	probe.remove();
	const x = variant === 'b' ? 960 - m.curW / 2 : left + m.c.x;
	return { x, y: top + m.blockTop, w: m.curW, h: m.blockH };
}

const STAR = 'polygon(50% 0%, 61.2% 34.5%, 97.6% 34.5%, 68.2% 55.9%, 79.4% 90.5%, 50% 69.1%, 20.6% 90.5%, 31.8% 55.9%, 2.4% 34.5%, 38.8% 34.5%)';

/**
 * A framed item, built centred on (cx, cy) of `layer`. `cap` is the cap height to fill: text
 * is set so its caps are that tall, shapes are sized from it. Returns the element and its box.
 */
function item(layer, kind, cap, { lang, art, cx, cy, fg = '#fff' }) {
	const [type, arg] = kind.split(':');
	let e;
	let w;
	let h;
	if (type === 'poster') {
		h = cap * 1.18;
		w = h * (2 / 3);
		e = el('div', 'id-poster', layer);
		const img = el('img', null, e);
		img.src = art[arg].p;
		img.alt = '';
	} else if (type === 'couch') {
		h = cap * 0.95;
		w = h * (1046 / 745);
		e = css(el('div', 'id-couch', layer), { background: fg });
	} else if (type === 'play') {
		h = cap;
		w = h * 0.88;
		e = css(el('div', 'id-shape', layer), { background: fg, clipPath: 'polygon(8% 0%, 100% 50%, 8% 100%)' });
	} else if (type === 'star') {
		h = cap * 1.12;
		w = h * 1.05;
		e = css(el('div', 'id-shape', layer), { background: fg, clipPath: STAR });
	} else if (type === 'rgb') {
		h = cap;
		w = h * 1.62;
		e = el('div', 'id-rgb', layer);
		for (const c of ['#ff0000', '#00ff00', '#0000ff']) css(el('i', null, e), { background: c });
	} else {
		const text = type === 'time' ? '20:15' : COPY[lang][type];
		e = css(el('div', 'id-txt tnum', layer, text), { fontSize: (cap / 0.69).toFixed(1) + 'px', color: fg });
		w = e.offsetWidth;
		h = cap;
	}
	if (type !== 'on' && type !== 'now' && type !== 'movies' && type !== 'series' && type !== 'time') css(e, { width: w + 'px', height: h + 'px' });
	// text boxes are line-height 1 with the caps centred, so the box centre is the caps' centre
	const bh = e.classList.contains('id-txt') ? cap / 0.69 : h;
	css(e, { left: (cx - w / 2).toFixed(1) + 'px', top: (cy - bh / 2).toFixed(1) + 'px' });
	return { e, w, h };
}

/** Pop an item in at `a` and show it until `z`. */
function popIn(tl, it, a, z) {
	windows(tl, it.e, [[a, z]]);
	tl.add(it.e, [{ transform: 'scale(0.82)' }, { transform: 'scale(1)' }], a, 0.1, 'out');
}

export function ident(stage, data, audio) {
	const variant = data.variant === 'b' ? 'b' : 'a';
	const cutName = data.cut === 'short' ? 'short' : 'full';
	const g = GRID[cutName];
	const c = CUTS[variant][cutName];
	const tl = new Timeline(stage, g.D);
	stage.classList.add('ident', `ident-${variant}`, `ident-${cutName}`);
	const ctx = { stage, data, audio, tl, g, c, full: cutName === 'full', lang: data.lang, art: data.art };
	(variant === 'b' ? frameVariant : typedVariant)(ctx);
	for (const ev of audio.themeScore(data.theme || 1, g)) tl.sound(ev.at, ev.fn, ev.dur);
	return tl;
}

/** The parts both variants share: fields, the wordmark, the hit, the hook, TV and the tagline. */
function setup(x, { colors, plainO = false }) {
	const { stage } = x;
	const red = css(el('div', 'id-field id-red', stage), { opacity: 0 });
	const white = css(el('div', 'id-field id-white', stage), { opacity: 0 });
	const wm = wordmark(stage, SIZE, { colors, plainO, cls: 'wm-abs id-wm' });
	centre(wm, 960, 540);
	const layer = el('div', 'id-items', wm.root);
	return { red, white, wm, layer };
}

/** From the hit to the end: red field for a beat, the brackets sing the hook, TV lands. */
function finale(x, s, gapMoves, curMoves, curWin, { tvTyped, tvWin = [] }) {
	const { tl, g, audio, full } = x;
	const { wm } = s;
	const m = wm.m;
	const H = g.hit;
	const n = HOOK.at.map((r) => H + r);
	const D = g.D;
	tl.cue('hit: the brackets snap shut', H);
	tl.cue('the hook: the brackets sing', n[1]);
	tl.cue('hook, last note: TV lands', n[4]);
	// one beat of pure red under the lockup (the brackets go black for it)
	windows(tl, s.red, [[H, H + 0.5], ...(s.redWins || [])]);
	// the brackets open a little on each note of the hook: wider on the top note
	[[n[1], 0.2], [n[2], 0.42], [n[3], 0.22]].forEach(([t, w]) => {
		gapMoves.push({ at: t, dur: 0.05, to: { g: m.em * w + m.split, k: 1 }, ease: 'out' });
		gapMoves.push({ at: t + 0.09, dur: 0.12, to: { g: m.split, k: 1 }, ease: 'inOut' });
		audio && tl.sound(t, (w2, b) => audio.clack(b, w2, { gain: 0.025, freq: SPARKLE[1], pan: -0.1 }));
	});
	// the cursor blinks on the beat, then becomes the TV block on the long note
	const after = tvTyped ? m.v.x + m.v.w + 0.07 * m.em : m.tv.x;
	curMoves.push({ at: H, dur: 0, to: cursorAt(m, after) });
	curMoves.push({ at: n[4], dur: 0.12, to: cursorAt(m, m.tv.x, m.tv.w), ease: 'snap' });
	curWin.push([H + 0.5, H + 0.75], [H + 1.0, n[4] + 0.12]);
	// the stretched cursor hands over to the real block (whose corners are not stretched)
	windows(tl, wm.tvBg, [...tvWin, [n[4] + 0.12, D + 1]]);
	// typed inverted into the block: T, then V
	if (!tvTyped) for (const [e, d] of [[wm.tvT, 0], [wm.tvV, 0.06]]) windows(tl, e, [...tvWin, [n[4] + d, D + 1]]);
	tl.add(wm.root, [{ transform: 'scale(1)' }, { transform: 'scale(1.025)' }], H, D - H, 'sine');
	// the tagline types itself out under the lockup on the final chord (full cut)
	if (full) {
		const T = g.final;
		tl.cue('final chord: tagline types', T);
		const line = el('div', 'id-tagline', x.stage);
		css(line, { left: (parseFloat(wm.root.style.left) + m.c.x + 0.04 * m.em).toFixed(1) + 'px', top: (parseFloat(wm.root.style.top) + m.em * 1.02).toFixed(1) + 'px' });
		const tc = chars(line, COPY[x.lang].tagline);
		const cur = el('i', 'id-tagcur', line);
		const step = 0.03;
		tc.items.forEach((ch, i) => tl.add(ch, [{ opacity: 0 }, { opacity: 1 }], T + 0.1 + i * step, 0.01, 'linear'));
		const endX = (i) => (i < 0 ? 0 : tc.items[i].offsetLeft + tc.items[i].offsetWidth);
		const cm = tc.items.map((_, i) => ({ at: T + 0.1 + i * step, dur: 0, to: endX(i) }));
		motion(tl, cur, D, 0, cm, (v) => ({ transform: `translateX(${v.toFixed(1)}px)` }));
		const tEnd = T + 0.1 + tc.items.length * step;
		const wins = [[T, tEnd + 0.25]];
		for (let t = Math.ceil((tEnd + 0.25) / 0.5) * 0.5; t < D + 0.5; t += 0.5) wins.push([t, t + 0.25]);
		windows(tl, cur, wins);
		tl.sound(T + 0.1, (w, b) => {
			for (let i = 0; i < tc.items.length; i += 2) audio.keyClick(b, w + i * step, { gain: 0.016, pan: -0.3 + (i / tc.items.length) * 0.6, seed: 500 + i, pitch: 1.2 });
		});
	}
}

// =========================================================================================
// Variant A: typed
// =========================================================================================

function typedVariant(x) {
	const { tl, g, c, full, audio, lang, art } = x;
	const D = g.D;
	const H = g.hit;
	const s = setup(x, { colors: ['red', 'black'], plainO: full });
	const { wm, layer } = s;
	const m = wm.m;
	const sel = css(el('i', 'wm-block wm-sel', wm.root), { width: m.o.w + 'px', transform: `translateX(${m.o.x}px)` });

	// --- typing: letters appear on the grid, the cursor rides one cell ahead -------------------
	const cells = full
		? [
				['C', m.c.x + m.c.w, [wm.c]],
				['O', m.o.x + m.o.w, [wm.plain.white]],
				['U', m.uch[0].x + m.uch[0].w, [wm.uch[0]]],
				['C', m.uch[1].x + m.uch[1].w, [wm.uch[1]]],
				['H', m.uch[2].x + m.uch[2].w, [wm.uch[2]]],
				[' ', m.t.x, []],
				['T', m.t.x + m.t.w, [wm.tvT]],
				['V', m.v.x + m.v.w + 0.07 * m.em, [wm.tvV]]
			]
		: [
				['C', m.c.x + m.c.w, [wm.c]],
				['(', m.cx, [wm.halves.red.l]],
				[')', m.o.x + m.o.w, [wm.halves.red.r]],
				['U', m.uch[0].x + m.uch[0].w, [wm.uch[0]]],
				['C', m.uch[1].x + m.uch[1].w, [wm.uch[1]]],
				['H', m.uch[2].x + m.uch[2].w, [wm.uch[2]]]
			];
	const curMoves = [];
	const typed = new Map();
	tl.cue(full ? 'cursor; types COUCH TV' : 'cursor (handed over by the squeeze); types C()UCH', 0);
	cells.forEach(([ch, nextX, els], i) => {
		const t = c.type.at + i * c.type.step;
		for (const e of els) typed.set(e, t);
		curMoves.push({ at: t, dur: 0, to: cursorAt(m, nextX) });
		if (ch !== ' ') tl.sound(t, (w, b) => audio.keyClick(b, w, { gain: 0.05, pan: -0.4 + i * 0.1, seed: 10 + i }));
	});
	const typeEnd = c.type.at + cells.length * c.type.step;
	for (const e of [wm.c, ...wm.uch, ...(full ? [wm.tvT, wm.tvV] : [])]) tl.add(e, [{ opacity: 0 }, { opacity: 1 }], typed.get(e), 0.01, 'linear');

	// --- the edit (full): select the O, type brackets over it ----------------------------------
	let redFrom;
	const curWin = [];
	if (full) {
		const [r0, r1] = c.replace;
		curWin.push(...c.blink, [c.type.at, typeEnd], [typeEnd + 0.125, c.select]);
		windows(tl, wm.plain.white, [[typed.get(wm.plain.white), c.select]]);
		windows(tl, wm.plain.black, [[c.select, r0]]);
		windows(tl, sel, [[c.select, r0]]);
		tl.cue('select the O', c.select);
		tl.cue('type ( ) over it', r0);
		tl.sound(c.select, (w, b) => audio.clack(b, w, { gain: 0.03, freq: SPARKLE[3], pan: -0.2 }));
		tl.sound(r0, (w, b) => audio.keyClick(b, w, { gain: 0.055, seed: 31, pan: -0.25 }));
		tl.sound(r1, (w, b) => audio.keyClick(b, w, { gain: 0.055, seed: 32, pan: -0.15 }));
		redFrom = { l: r0, r: r1 };
	} else {
		curWin.push([0, typeEnd + 0.125]);
		redFrom = { l: typed.get(wm.halves.red.l), r: typed.get(wm.halves.red.r) };
		windows(tl, sel, []);
	}
	const redWin = (from) => minus([[from, D + 1]], [[H, H + 0.5]]);
	windows(tl, wm.halves.red.l, redWin(redFrom.l));
	windows(tl, wm.halves.red.r, redWin(redFrom.r));
	windows(tl, wm.halves.black.l, [[H, H + 0.5]]);
	windows(tl, wm.halves.black.r, [[H, H + 0.5]]);

	// --- the brackets open and frame a beat-cut sequence ---------------------------------------
	const cap = m.capH;
	const gapMoves = [];
	const seq = c.seq;
	tl.cue('brackets open: the sequence', seq[0][0]);
	seq.forEach(([t, kind], i) => {
		const z = i + 1 < seq.length ? seq[i + 1][0] : H - 0.02;
		const big = kind.startsWith('poster') ? 1.3 : kind === 'couch' ? 1.2 : 1;
		const it = item(layer, kind, cap * (kind.startsWith('poster') ? 1.05 : kind === 'couch' ? 1.1 : 0.9), { lang, art, cx: m.cx, cy: m.cy });
		popIn(tl, it, t, z);
		const gap = it.w + 0.2 * m.em * big;
		const first = i === 0;
		gapMoves.push({ at: t - (first ? 0.02 : 0.01), dur: first ? 0.16 : z - t > 0.2 ? 0.1 : 0.06, to: { g: gap, k: big }, ease: first ? 'pop' : 'snap' });
		tl.sound(t, (w, b) => (first ? audio.clack(b, w, { gain: 0.05, freq: SPARKLE[0], pan: -0.1 }) : audio.clack(b, w, { gain: 0.022, freq: SPARKLE[[2, 4, 1, 3, 5][i % 5]], pan: (i % 2 ? 0.2 : -0.2), seed: i })));
	});
	// wind-up, then shut on the hit (a hair too far, and back)
	const last = gapMoves[gapMoves.length - 1].to;
	gapMoves.push({ at: H - 0.14, dur: 0.1, to: { g: last.g * 1.06, k: last.k }, ease: 'out' });
	gapMoves.push({ at: H - 0.035, dur: 0.035, to: { g: m.split - 0.03 * m.em, k: 1 }, ease: 'in' });
	gapMoves.push({ at: H, dur: 0.14, to: { g: m.split, k: 1 }, ease: 'pop' });

	finale(x, s, gapMoves, curMoves, curWin, { tvTyped: full });
	const v0 = { g: m.split, k: 1 };
	motion(tl, wm.halves.red.l, D, v0, gapMoves, (v) => ({ transform: geometry(m, v.g, { k: v.k }).l }));
	motion(tl, wm.halves.red.r, D, v0, gapMoves, (v) => ({ transform: geometry(m, v.g, { k: v.k }).r }));
	motion(tl, wm.halves.black.l, D, v0, gapMoves, (v) => ({ transform: geometry(m, v.g, { k: v.k }).l }));
	motion(tl, wm.halves.black.r, D, v0, gapMoves, (v) => ({ transform: geometry(m, v.g, { k: v.k }).r }));
	motion(tl, wm.c, D, v0, gapMoves, (v) => ({ transform: geometry(m, v.g, { k: v.k }).c }));
	motion(tl, wm.rest, D, v0, gapMoves, (v) => ({ transform: geometry(m, v.g, { k: v.k }).rest }));
	motion(tl, wm.cur, D, cursorAt(m, m.c.x), curMoves, (v) => ({ transform: v }));
	windows(tl, wm.cur, curWin);
}

// =========================================================================================
// Variant B: frame
// =========================================================================================

function frameVariant(x) {
	const { tl, g, c, full, audio, lang, art, stage } = x;
	const D = g.D;
	const H = g.hit;
	const s = setup(x, { colors: ['red', 'black'] });
	const { wm } = s;
	const m = wm.m;
	const rootX = parseFloat(wm.root.style.left);
	const rootY = parseFloat(wm.root.style.top);
	const big = el('div', 'id-items id-stage-items', stage);
	const seq = c.seq;
	const start = seq[0][0];
	const last = seq[seq.length - 1][0];
	const slam = H - 0.125;

	// what is on screen before the brackets leave the word
	const curMoves = [];
	const curWin = [];
	// letters one by one: UCH TV's group carries the cursor, which must stay free to show
	const letters = [wm.c, ...wm.uch];
	if (full) {
		tl.cue('the lockup', 0);
		curMoves.push({ at: 0, dur: 0, to: cursorAt(m, m.tv.x) });
		for (const e of letters) windows(tl, e, [[0, start], [H, D + 1]]);
	} else {
		tl.cue('cursor (handed over by the squeeze)', 0);
		const [a, z] = c.cursor;
		// the block sits at the stage centre: in the lockup's coordinates
		curMoves.push({ at: 0, dur: 0, to: cursorAt(m, 960 - rootX - m.curW / 2) });
		curWin.push([a, z]);
		for (const e of letters) windows(tl, e, [[H, D + 1]]);
	}

	// the frames: bracket geometry, field and colours per beat
	const centreDx = 960 - (rootX + m.cx);
	const centreDy = 540 - (rootY + m.cy);
	const capPx = m.capH;
	const moves = [];
	const brWin = { red: [], black: [] };
	const fields = { red: [], white: [] };
	if (!full) {
		// the cursor turns into the closed O, at the centre
		moves.push({ at: 0, dur: 0, to: { g: m.split, k: 1, dx: centreDx, dy: centreDy } });
		brWin.red.push([c.cursor[1], start]);
		tl.cue('the cursor becomes the O', c.cursor[1]);
		tl.sound(c.cursor[1], (w, b) => audio.clack(b, w, { gain: 0.04, freq: SPARKLE[0] }));
	} else brWin.red.push([0, start]);
	tl.cue('brackets take the screen: beat cuts', start);
	// cap heights (px) per item: the brackets scale to hug them
	const CAPS = { on: 320, now: 270, time: 250, movies: 175, series: 175, star: 270, play: 270, rgb: 200, couch: 270, poster: 300 };
	seq.forEach(([t, field, kind], i) => {
		const z = i + 1 < seq.length ? seq[i + 1][0] : slam;
		const type = kind.split(':')[0];
		const fg = field === 'white' ? '#000' : '#fff';
		const it = item(big, kind, CAPS[type], { lang, art, cx: 960, cy: 540, fg });
		popIn(tl, it, t, z);
		const k = Math.min(3.1, (it.h * 1.1) / capPx);
		const gap = it.w + 0.16 * m.em * k;
		const beat = Math.abs((t * 2) % 1) < 1e-6;
		moves.push({ at: t, dur: i === 0 ? 0.14 : beat ? 0.09 : 0, to: { g: gap, k, dx: centreDx, dy: centreDy }, ease: i === 0 ? 'pop' : 'snap' });
		const br = field === 'red' ? 'black' : 'red';
		brWin[br].push([t, z]);
		if (field !== 'black') fields[field].push([t, z]);
		tl.sound(t, (w, b) => audio.clack(b, w, { gain: i === 0 ? 0.05 : 0.026, freq: SPARKLE[[0, 2, 4, 1, 3, 5][i % 6]], pan: i % 2 ? 0.25 : -0.25, seed: i }));
	});
	tl.cue('slam back into the O', slam);
	brWin.red.push([slam, H]);
	moves.push({ at: slam, dur: 0.1, to: { g: m.split - 0.03 * m.em, k: 1, dx: 0, dy: 0 }, ease: 'in' });
	moves.push({ at: H, dur: 0.14, to: { g: m.split, k: 1, dx: 0, dy: 0 }, ease: 'pop' });
	brWin.red.push([H + 0.5, D + 1]);
	brWin.black.push([H, H + 0.5]);
	void last;

	const gapMoves = [];
	s.redWins = fields.red;
	finale(x, s, gapMoves, curMoves, curWin, { tvTyped: false, tvWin: full ? [[0, start]] : [] });
	// the letters come back out from behind the brackets, then follow the hook's pulses
	const lv = (off) => ({ g: m.split, k: 1, off });
	const letterMoves = [
		{ at: H - 0.002, dur: 0, to: lv(0.3 * m.em) },
		{ at: H, dur: 0.2, to: lv(0), ease: 'snap' },
		...gapMoves.map((mv) => ({ ...mv, to: { ...mv.to, off: 0 } }))
	];
	motion(tl, wm.c, D, lv(0), letterMoves, (v) => ({ transform: `${geometry(m, v.g, { k: v.k }).c} translateX(${v.off.toFixed(1)}px)` }));
	motion(tl, wm.rest, D, lv(0), letterMoves, (v) => ({ transform: `${geometry(m, v.g, { k: v.k }).rest} translateX(${(-v.off).toFixed(1)}px)` }));
	// the finale's pulses are plain gaps: give them the rest geometry's other fields
	for (const mv of gapMoves) moves.push({ ...mv, to: { ...mv.to, dx: 0, dy: 0 } });
	const v0 = full ? { g: m.split, k: 1, dx: 0, dy: 0 } : { g: m.split, k: 1, dx: centreDx, dy: centreDy };
	for (const col of ['red', 'black']) {
		const h = wm.halves[col];
		motion(tl, h.l, D, v0, moves, (v) => ({ transform: geometry(m, v.g, { k: v.k, dx: v.dx, dy: v.dy }).l }));
		motion(tl, h.r, D, v0, moves, (v) => ({ transform: geometry(m, v.g, { k: v.k, dx: v.dx, dy: v.dy }).r }));
		windows(tl, h.l, brWin[col]);
		windows(tl, h.r, brWin[col]);
	}
	windows(tl, s.white, fields.white);
	motion(tl, wm.cur, D, curMoves[0].to, curMoves.slice(1), (v) => ({ transform: v }));
	windows(tl, wm.cur, curWin);
}
