// The channel ident. The full cut (10.52 s) is one of four designs (idents/, D35); the short cut
// (4.52 s) is variant B "frame" (D27): the brackets leave the word and frame the whole screen,
// hugging each item and cutting on the beat over flat black and red fields, then slam back into
// the O on the hit; the letters slide out from behind them, the brackets sing the hook and its
// long note drops TV into its block. Picture and score read theme.ts's GRID (120 bpm), so the
// cue table is in seconds of the beat; cuts land on the frame nearest their beat (core/moves.ts).
//
// The brackets' outline grows with what they frame while their stroke keeps the lockup's
// weight, so their shape is computed per frame (a pure function of t) instead of scaled.

import { SPARKLE } from '../audio/kit';
import { css, div, el, img } from '../core/dom';
import { edge, motion, windows, type Move } from '../core/moves';
import { frame } from '../core/timeline';
import type { IdentSpec } from '../spec';
import { timeline, type Template, type TL } from '../templates/shared';
import { buildFullIdent } from './idents';
import { GRID, HOOK, themeScore } from './theme';
import {
	animateBrackets,
	centre,
	cursorAt,
	geometry,
	layout,
	METRICS,
	placeAt,
	wordmark,
	type Frame
} from './wordmark';

/** The lockup's font size on the 1920x1080 stage. */
const SIZE = 190;

const COPY = {
	on: 'ON',
	now: 'NOW',
	movies: 'MOVIES',
	series: 'SERIES',
	time: '20:15'
};

type Field = 'black' | 'red' | 'white';
export type Item =
	'on' | 'now' | 'movies' | 'series' | 'time' | 'star' | 'play' | 'rgb' | 'couch' | 'poster';

// [beat time, field, framed item]; the poster is a stand-in card in the brand's colours
const CUTS: [number, Field, Item][] = [
	[0.5, 'black', 'on'],
	[1.0, 'red', 'time'],
	[1.25, 'black', 'poster'],
	[1.5, 'black', 'star'],
	[1.625, 'black', 'rgb'],
	[1.75, 'black', 'couch']
];

/** The short cut opens on the cursor, which becomes the closed O. */
const CURSOR: [number, number] = [0, 0.25];

// cap heights (px) per item: the brackets grow to hug them
const CAPS: Record<Item, number> = {
	on: 320,
	now: 270,
	time: 250,
	movies: 175,
	series: 175,
	star: 270,
	play: 270,
	rgb: 200,
	couch: 270,
	poster: 300
};

const STAR =
	'polygon(50% 0%, 61.2% 34.5%, 97.6% 34.5%, 68.2% 55.9%, 79.4% 90.5%, 50% 69.1%, 20.6% 90.5%, 31.8% 55.9%, 2.4% 34.5%, 38.8% 34.5%)';

/** The red block the short cut opens on, in stage px: the squeeze collapses into it. */
export function identCursorRect() {
	const m = layout(SIZE);
	const top = placeAt(m, 960, 540).y;
	return {
		x: 960 - m.curW / 2,
		y: top + m.blockTop,
		width: m.curW,
		height: m.blockH,
		radius: m.radius
	};
}

/**
 * The short cut's first frame without its field, from `at` on: the cursor built and animated as
 * the ident does it (animated parts rasterize on layers of their own), so a piece that hands
 * over to the ident ends on exactly its pixels.
 */
export function identCursor(tl: TL, parent: Element, at: number) {
	const wm = wordmark(parent, SIZE, { colors: ['red', 'black'], cls: 'id-wm' });
	const pos = centre(wm, 960, 540);
	const m = wm.m;
	for (const e of [wm.c, ...wm.uch, wm.tvBg, wm.tvT, wm.tvV]) windows(tl, e, []);
	for (const h of Object.values(wm.halves)) for (const s of [h.l.svg, h.r.svg]) windows(tl, s, []);
	const transform = (v: string) => ({ transform: v });
	motion(tl, wm.rest, `${geometry(m, m.split).rest} translateX(0.0px)`, [], transform);
	motion(tl, wm.cur, cursorAt(m, 960 - pos.x - m.curW / 2), [], transform);
	windows(tl, wm.cur, [[at, tl.duration + 1]]);
	tl.add(wm.root, [{ transform: 'scale(1)' }, { transform: 'scale(1)' }], 0, tl.duration, 'linear');
}

export interface Framed {
	e: HTMLElement;
	w: number;
	h: number;
}

/** A framed item centred on the stage, sized from its cap height (text: the caps' height). */
export function frameItem(
	layer: Element,
	kind: Item,
	fg: string,
	poster: string | null = null
): Framed {
	const cap = CAPS[kind];
	let e: HTMLElement;
	let w: number;
	let h: number;
	if (kind === 'poster') {
		h = cap * 1.18;
		w = h * (2 / 3);
		e = div('id-poster', layer);
		if (poster) img(null, e, poster);
		// no poster to show: a card in the brand's colours, the couch on it
		else e.classList.add('id-poster-stand');
	} else if (kind === 'couch') {
		h = cap * 0.95;
		w = h * (1046 / 745);
		e = css(div('id-couch', layer), { background: fg });
	} else if (kind === 'play') {
		h = cap;
		w = h * 0.88;
		e = css(div('id-shape', layer), {
			background: fg,
			clipPath: 'polygon(8% 0%, 100% 50%, 8% 100%)'
		});
	} else if (kind === 'star') {
		h = cap * 1.12;
		w = h * 1.05;
		e = css(div('id-shape', layer), { background: fg, clipPath: STAR });
	} else if (kind === 'rgb') {
		h = cap;
		w = h * 1.62;
		e = div('id-rgb', layer);
		for (const c of ['#ff0000', '#00ff00', '#0000ff']) css(el('i', null, e), { background: c });
	} else {
		const size = cap / METRICS.cap;
		e = css(div('id-txt tnum', layer, COPY[kind]), { fontSize: `${size.toFixed(1)}px`, color: fg });
		w = e.offsetWidth;
		h = cap;
		// a text box is line-height 1: its caps' centre sits where the wordmark's does
		css(e, {
			left: `${(960 - w / 2).toFixed(1)}px`,
			top: `${(540 - (METRICS.baseline - METRICS.cap / 2) * size).toFixed(1)}px`
		});
		return { e, w, h };
	}
	css(e, {
		width: `${w}px`,
		height: `${h}px`,
		left: `${(960 - w / 2).toFixed(1)}px`,
		top: `${(540 - h / 2).toFixed(1)}px`
	});
	return { e, w, h };
}

/** Show an item from `a` until `z`, popping in. */
export function popIn(tl: TL, it: Framed, a: number, z: number) {
	windows(tl, it.e, [[a, z]]);
	tl.add(it.e, [{ transform: 'scale(0.82)' }, { transform: 'scale(1)' }], edge(a), 0.1, 'out');
}

export const ident: Template<IdentSpec> = {
	version: 4,
	build(ctx, spec) {
		if (spec.cut !== 'short') return buildFullIdent(ctx, spec);
		const { root, sound } = ctx;
		const g = GRID;
		const D = frame(g.D);
		const H = g.hit;
		const n = HOOK.at.map((r) => H + r);
		const tl = timeline(root, D);
		root.classList.add('ident');

		const red = div('id-field id-red', root);
		const white = div('id-field id-white', root);
		const wm = wordmark(root, SIZE, { colors: ['red', 'black'], cls: 'id-wm' });
		const m = wm.m;
		const pos = centre(wm, 960, 540);
		const items = div('id-items', root);
		const seq = CUTS;
		const start = seq[0][0];
		const slam = H - 0.125;

		// --- before the brackets leave the word ----------------------------------------------
		const letters = [wm.c, ...wm.uch];
		const curMoves: Move<string>[] = [];
		const curWin: [number, number][] = [];
		const brWin: Record<'red' | 'black', [number, number][]> = { red: [], black: [] };
		const fields: Record<'red' | 'white', [number, number][]> = { red: [[H, H + 0.5]], white: [] };
		// the brackets' centre on the stage's, around the items
		const centreDx = 960 - (pos.x + m.cx);
		const centreDy = 540 - (pos.y + m.cy);
		const v0: Frame = { g: m.split, k: 1, dx: centreDx, dy: centreDy };
		tl.cue('the cursor (handed over by the squeeze)', 0);
		// the block sits at the stage centre, in the wordmark's coordinates
		const cur0 = cursorAt(m, 960 - pos.x - m.curW / 2);
		curWin.push(CURSOR);
		tl.cue('the cursor becomes the O', CURSOR[1]);
		brWin.red.push([CURSOR[1], start]);
		tl.sound(CURSOR[1], (w, b) => sound.clack(b, w, { gain: 0.04, freq: SPARKLE[0] }));
		for (const e of letters) windows(tl, e, [[H, D + 1]]);

		// --- the frames: bracket geometry, field and colour per beat ---------------------------
		tl.cue('the brackets take the screen: beat cuts', start);
		const moves: Move<Frame>[] = [];
		seq.forEach(([t, field, kind], i) => {
			const z = i + 1 < seq.length ? seq[i + 1][0] : slam;
			const fg = field === 'white' ? '#000' : '#fff';
			const it = frameItem(items, kind, fg);
			popIn(tl, it, t, z);
			const k = Math.min(3.1, (it.h * 1.1) / m.capH);
			const gap = it.w + 0.16 * m.em * k;
			const beat = Math.abs((t * 2) % 1) < 1e-6;
			moves.push({
				at: t,
				dur: i === 0 ? 0.14 : beat ? 0.09 : 0,
				to: { g: gap, k, dx: centreDx, dy: centreDy },
				ease: i === 0 ? 'pop' : 'snap'
			});
			brWin[field === 'red' ? 'black' : 'red'].push([t, z]);
			if (field !== 'black') fields[field].push([t, z]);
			tl.sound(t, (w, b) =>
				sound.clack(b, w, {
					gain: i === 0 ? 0.05 : 0.026,
					freq: SPARKLE[[0, 2, 4, 1, 3, 5][i % 6]],
					pan: i % 2 ? 0.25 : -0.25,
					seed: i
				})
			);
		});
		tl.cue('slam back into the O', slam);
		brWin.red.push([slam, H], [H + 0.5, D + 1]);
		brWin.black.push([H, H + 0.5]);
		moves.push({
			at: slam,
			dur: 0.1,
			to: { g: m.split - 0.03 * m.em, k: 1, dx: 0, dy: 0 },
			ease: 'in'
		});
		moves.push({ at: H, dur: 0.14, to: { g: m.split, k: 1, dx: 0, dy: 0 }, ease: 'pop' });

		// --- the hit: a beat of pure red, the letters slide out from behind the brackets -------
		tl.cue('hit: the brackets snap shut', H);
		tl.cue('the hook: the brackets sing', n[1]);
		tl.cue('the hook, last note: TV lands', n[4]);
		windows(tl, red, fields.red);
		windows(tl, white, fields.white);
		// the brackets open a little on each note of the hook, widest on the top note
		const pulses: Move<Frame>[] = (
			[
				[n[1], 0.2],
				[n[2], 0.42],
				[n[3], 0.22]
			] as const
		).flatMap(([t, w]) => {
			tl.sound(t, (w2, b) => sound.clack(b, w2, { gain: 0.025, freq: SPARKLE[1], pan: -0.1 }));
			return [
				{ at: t, dur: 0.05, to: { g: m.em * w + m.split, k: 1, dx: 0, dy: 0 }, ease: 'out' },
				{ at: t + 0.09, dur: 0.12, to: { g: m.split, k: 1, dx: 0, dy: 0 }, ease: 'inOut' }
			];
		});
		moves.push(...pulses);
		const lv = (off: number) => ({ g: m.split, off });
		const letterMoves: Move<{ g: number; off: number }>[] = [
			{ at: H - 0.002, to: lv(0.3 * m.em) },
			{ at: H, dur: 0.2, to: lv(0), ease: 'snap' },
			...pulses.map((mv) => ({ ...mv, to: { g: mv.to.g, off: 0 } }))
		];
		motion(tl, wm.c, lv(0), letterMoves, (v) => ({
			transform: `${geometry(m, v.g).c} translateX(${v.off.toFixed(1)}px)`
		}));
		motion(tl, wm.rest, lv(0), letterMoves, (v) => ({
			transform: `${geometry(m, v.g).rest} translateX(${(-v.off).toFixed(1)}px)`
		}));
		animateBrackets(tl, wm, v0, moves, brWin);

		// --- the cursor blinks on the beat, then becomes the TV block on the long note ---------
		curMoves.push({ at: H, to: cursorAt(m, m.tv.x) });
		curMoves.push({ at: n[4], dur: 0.12, to: cursorAt(m, m.tv.x, m.tv.w), ease: 'snap' });
		curWin.push([H + 0.5, H + 0.75], [H + 1.0, n[4] + 0.12]);
		motion(tl, wm.cur, cur0, curMoves, (v) => ({ transform: v }));
		windows(tl, wm.cur, curWin);
		// the stretched cursor hands over to the real block, whose corners are not stretched
		windows(tl, wm.tvBg, [[n[4] + 0.12, D + 1]]);
		// typed inverted into the block: T, then V
		windows(tl, wm.tvT, [[n[4], D + 1]]);
		windows(tl, wm.tvV, [[n[4] + 0.06, D + 1]]);
		tl.add(wm.root, [{ transform: 'scale(1)' }, { transform: 'scale(1.025)' }], H, D - H, 'sine');

		for (const ev of themeScore(sound, g)) tl.sound(ev.at, ev.fn, ev.dur);
		return tl;
	}
};
