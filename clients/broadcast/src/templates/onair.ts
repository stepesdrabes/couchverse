// The look of the screens around the programs (up next, the squeeze, the lineup, standby): the
// idents' night, paper type and the brand's red as sharp blocks and rules. Everything is square
// and flat (no rounded corners, shadows or glows); type rises into place from behind its own
// edge; a title shows big as its own logo and in pictures as scenes from itself. These are the
// parts the screens share; each screen places them on the grid (DESIGN.md 1.6).

import type { Assets } from '../assets';
import type { Bus } from '../audio/engine';
import type { Kit } from '../audio/kit';
import { house } from '../audio/voices';
import { css, div, img, span, words } from '../core/dom';
import { fitText, type FitBox } from '../core/fit';
import { framed } from '../core/framing';
import type { Crop, FootageShot, Title } from '../spec';
import type { TL } from './shared';

/** The screens' colours (DESIGN.md 1.2): paper's tiers are solid over night, so small type never blends. */
export const INK = {
	red: '#e50914',
	night: '#050505',
	night1: '#0c0a0a',
	night2: '#141111',
	paper: '#f4ede4',
	paper72: '#b1aca6',
	paper56: '#8b8782',
	paper40: '#65625e',
	paper24: '#3e3d3b',
	paper12: '#222120',
	ember900: '#140203',
	ember800: '#260306',
	ember700: '#3a0409',
	ember600: '#560610',
	ember500: '#7a0a14'
} as const;

const rgb = (hex: string) => [1, 3, 5].map((i) => parseInt(hex.slice(i, i + 2), 16)).join(' ');

/** Night at an alpha, for gradients. */
export const night = (a: number) => `rgb(${rgb(INK.night)} / ${a})`;

/** The grades over a picture (DESIGN.md 1.7), each a CSS background. */
export const GRADE = {
	/** The bug's corner, so it reads over the brightest picture. */
	bug: `radial-gradient(760px 300px at 100% 0%, ${night(0.6)} 0%, ${night(0.33)} 45%, transparent 100%)`,
	/** The left falling to night: where the type sits. */
	left: (to = 66, a = 0.94) =>
		`linear-gradient(90deg, ${night(a)} 0%, ${night(a * 0.82)} ${to * 0.4}%, ${night(0)} ${to}%)`,
	/** The foot falling to night. */
	foot: (to = 58, a = 0.94) =>
		`linear-gradient(0deg, ${night(a)} 0%, ${night(a * 0.7)} ${to * 0.45}%, ${night(0)} ${to}%)`,
	/** A flat dim, as a gradient so it stacks with the rest. */
	dim: (a: number) => `linear-gradient(${night(a)}, ${night(a)})`
};

/**
 * Every static grade over a picture in one element (CSS stacks the backgrounds, the first on
 * top): one full-frame blend per frame instead of one per gradient.
 */
export function grade(parent: Element, layers: string[], cls = 'fill') {
	return css(div(cls, parent), { background: layers.join(', ') });
}

/**
 * Let the house's hall ring on past the clip's end: in a break a clip's buses fade where the
 * next one starts and only the engine's hall rings on under it, so a screen whose last chord is
 * meant to ring into what follows feeds that hall from its own. Once per piece.
 */
export function ringOn(k: Kit, b: Bus, amount = 0.3) {
	k.send(b, house(k, b).hall, amount);
}

/** The way in: a fast fade and a rise into place on the glide (DESIGN.md 1.8's type-in). */
export function typeIn(tl: TL, els: Element[], at: number, { rise = 16, stagger = 0.08 } = {}) {
	els.forEach((e, i) => {
		tl.add(e, [{ opacity: 0 }, { opacity: 1 }], at + i * stagger, 0.28, 'linear');
		tl.add(
			e,
			[{ transform: `translateY(${rise}px)` }, { transform: 'translateY(0px)' }],
			at + i * stagger,
			0.72,
			'glide'
		);
	});
}

/**
 * Lines rising into place from behind their own edge (each word in its mask, `words`), one
 * line after another: the house's masked reveal. They travel a quarter past their height, clear
 * of the padding a mask keeps for accents and descenders.
 */
export function reveal(tl: TL, lines: Element[][], at: number, { stagger = 0.08, dur = 0.9 } = {}) {
	lines.forEach((line, i) => {
		for (const e of line)
			tl.add(
				e,
				[{ transform: 'translateY(125%)' }, { transform: 'translateY(0%)' }],
				at + i * stagger,
				dur,
				'glide'
			);
	});
	return at + (lines.length - 1) * stagger + dur;
}

/** The way out: everything lifts away behind its edge, accelerating. */
export function conceal(
	tl: TL,
	lines: Element[][],
	at: number,
	{ stagger = 0.03, dur = 0.42 } = {}
) {
	lines.forEach((line, i) => {
		for (const e of line)
			tl.add(
				e,
				[{ transform: 'translateY(0%)' }, { transform: 'translateY(-125%)' }],
				at + i * stagger,
				dur,
				'leave'
			);
	});
}

/** An element of its own inside a mask, so it can rise from behind the mask's edge. */
export function masked(parent: Element, cls: string, text?: string) {
	const m = div('oa-mask', parent);
	return div(cls, m, text);
}

/**
 * The red label block ("Up next"): a sharp field of the brand's red that wipes open from its
 * left edge, its word rising into it.
 */
export function label(parent: Element, text: string, cls = '') {
	const el = div(`oa-label ${cls}`.trim(), parent);
	const fill = div('oa-label-fill', el);
	const word = span('oa-label-word', el, text);
	return { el, fill, word };
}

export function labelIn(tl: TL, l: ReturnType<typeof label>, at: number) {
	tl.add(l.fill, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], at, 0.5, 'glide');
	tl.add(
		l.word,
		[{ transform: 'translateY(105%)' }, { transform: 'translateY(0%)' }],
		at + 0.1,
		0.62,
		'glide'
	);
}

/** The label leaves: its word lifts out and the field closes toward its right edge. */
export function labelOut(tl: TL, l: ReturnType<typeof label>, at: number) {
	tl.add(
		l.word,
		[{ transform: 'translateY(0%)' }, { transform: 'translateY(-105%)' }],
		at,
		0.32,
		'leave'
	);
	tl.add(
		l.fill,
		[
			{ transform: 'translateX(0px) scaleX(1)' },
			{ transform: `translateX(${l.el.offsetWidth}px) scaleX(0)` }
		],
		at + 0.06,
		0.36,
		'leave'
	);
}

/** A meta line: items separated by small squares (never a middle dot: it sits too high). */
export function meta(parent: Element, cls: string, items: string[]) {
	const el = div(`oa-meta ${cls}`.trim(), parent);
	items.forEach((x, i) => {
		if (i) span('oa-sep', el);
		span(null, el, x);
	});
	return el;
}

/** Whether a title's logo can be drawn: one it has that loaded. */
export const hasLogo = (t: Title, assets: Assets) =>
	!!t.logo?.url && !assets.missing.includes(t.logo.url);

/**
 * A logo's size within a box: logos vary from a 4:1 bar to a near square, so each gets the same
 * area (an equal visual weight), within the box's width and height, never stretched.
 */
export function logoSize(aspect: number, { area, width, height }: LogoBox) {
	let w = Math.sqrt(area * aspect);
	let h = w / aspect;
	if (w > width) [w, h] = [width, width / aspect];
	if (h > height) [w, h] = [height * aspect, height];
	return { width: Math.round(w), height: Math.round(h) };
}

export interface LogoBox {
	/** The area every logo gets, px². */
	area: number;
	width: number;
	height: number;
}

export interface Mark {
	el: HTMLElement;
	/** What rises into place, line by line (the logo is one line). */
	lines: Element[][];
	logo: boolean;
}

/**
 * A title shown big: its own logo, sized by its aspect (`logoSize`), else its name fitted
 * (`fitText`) in the element's type role. A logo replaces the name, never sits beside it; the
 * name stays as the logo's accessible text.
 */
export function titleMark(
	parent: Element,
	title: Title,
	assets: Assets,
	cls: string,
	fit: FitBox,
	box: LogoBox
): Mark {
	if (title.logo && hasLogo(title, assets)) {
		const { width, height } = logoSize(title.logo.aspect, box);
		const m = div(`oa-mask oa-logo ${cls}`, parent);
		const i = css(img(null, m, title.logo.url), { width: `${width}px`, height: `${height}px` });
		i.alt = title.name;
		return { el: m, lines: [[i]], logo: true };
	}
	const host = div(cls, parent);
	const w = words(host, title.name);
	fitText(host, fit);
	return { el: host, lines: byLines(host, w.items), logo: false };
}

/** Words grouped by the lines `fitText` broke them onto (`<br>`s), or as laid out. */
export function byLines<T extends Element>(root: Element, items: T[]): T[][] {
	const set = new Set<Element>(items);
	const lines: T[][] = [[]];
	for (const e of root.querySelectorAll('*')) {
		if (e.tagName === 'BR') lines.push([]);
		else if (set.has(e)) lines[lines.length - 1].push(e as T);
	}
	return lines.filter((l) => l.length);
}

// --- scenes ---------------------------------------------------------------------------------

export interface ScenesOptions {
	/** The clip's time the scenes start (they show from here). */
	start?: number;
	/** And end. */
	end?: number;
	/**
	 * How far into the reel `start` is: a clip continuing another's picture (up next after an
	 * expanding squeeze) starts where that one was.
	 */
	offset?: number;
	/** The backdrop's crop when the title has no scenes, and its zoom over the clip. */
	crop?: Crop;
	zoom?: [number, number];
	/** The view, px (full frame by default). */
	view?: { w: number; h: number };
}

/** Seconds of a dissolve between two scenes, and the longest a scene holds the screen. */
export const DISSOLVE = 0.8;
export const LONGEST = 3.2;
const FPS = 25;

export interface ReelCut {
	shot: FootageShot;
	/** Seconds into the reel the cut starts and ends. */
	from: number;
	to: number;
	/** Seconds its dissolve in takes (the first takes none). */
	dissolve: number;
}

/**
 * The reel a title's scenes make, from reel time 0 to `until`: its shots in order, cycling,
 * each for its length (at most LONGEST), dissolving into the next. Pure: a screen and the one
 * continuing it agree on every cut.
 */
export function reel(shots: FootageShot[], until: number): ReelCut[] {
	const out: ReelCut[] = [];
	let at = 0;
	for (let k = 0; at < until && shots.length; k++) {
		const shot = shots[k % shots.length];
		const len = Math.min(LONGEST, shot.frames.length / FPS);
		const next = shots[(k + 1) % shots.length];
		const nextLen = Math.min(LONGEST, next.frames.length / FPS);
		// a short scene dissolves for less, so it still shows on its own for a moment
		const dissolve = Math.min(DISSOLVE, 0.4 * len, 0.4 * nextLen);
		const prev = out[out.length - 1];
		out.push({ shot, from: at, to: at + len, dissolve: prev ? prev.dissolve : 0 });
		at += len - dissolve;
		if (out.length > 64) break;
	}
	// each cut's dissolve in is the overlap with the one before it
	for (let i = 1; i < out.length; i++) out[i].dissolve = out[i - 1].to - out[i].from;
	if (out.length) out[0].dissolve = 0;
	return out;
}

/**
 * A title's picture, full bleed in its view: its scenes (`footage`) dissolving one into the
 * next, else its backdrop, else the channel's own plate. The returned element is the camera:
 * push it (`push`) and the whole picture moves as one.
 */
export function scenes(
	tl: TL,
	parent: Element,
	title: Title,
	assets: Assets,
	o: ScenesOptions = {}
): HTMLDivElement {
	const start = o.start ?? 0;
	const end = o.end ?? tl.duration;
	const offset = o.offset ?? 0;
	const view = o.view ?? { w: 1920, h: 1080 };
	const cam = css(div('oa-camera', parent), { width: `${view.w}px`, height: `${view.h}px` });
	const shots = (title.footage ?? []).filter((s) => s.frames.length);
	if (shots.length) {
		for (const cut of reel(shots, offset + end - start)) {
			const a = start + cut.from - offset;
			const b = start + cut.to - offset;
			if (b <= start || a >= end) continue;
			const box = div('oa-scene', cam);
			const frame = img(null, box);
			if (cut.shot.focus) {
				const [x, y, w, h] = cut.shot.focus;
				frame.style.objectPosition = `${((x + w / 2) * 100).toFixed(1)}% ${((y + h / 2) * 100).toFixed(1)}%`;
			}
			tl.footage(frame, cut.shot.frames, a);
			tl.show(box, Math.max(start, a), Math.min(end, b) + 0.001);
			// the incoming scene dissolves in over the one before it (the image, not its box: a
			// fading group would cost an extra pass), part way through when the view opens on it
			if (cut.dissolve > 0 && a + cut.dissolve > start)
				tl.add(frame, [{ opacity: 0 }, { opacity: 1 }], a, cut.dissolve, 'sine');
		}
		return cam;
	}
	if (title.backdrop && !assets.missing.includes(title.backdrop)) {
		const s = framed(cam, assets.picture(title.backdrop), view);
		s.img.style.transform = s.tf(o.crop ?? [0.03, 0.03, 0.94, 0.94]);
		return cam;
	}
	plate(cam);
	return cam;
}

/** A slow push on a camera: zoom over a stretch, linear (a drift across a whole clip). */
export function push(tl: TL, cam: Element, at: number, dur: number, [z0, z1]: [number, number]) {
	tl.add(cam, [{ transform: `scale(${z0})` }, { transform: `scale(${z1})` }], at, dur, 'linear');
}

/**
 * The channel's own picture for a title without artwork (DESIGN.md 1.3): night above, the sky
 * reddening to a low horizon where the sun went down, a line of ember light on it, night below.
 */
export function plate(parent: Element, { horizon = 700, sun = 1340 } = {}) {
	const p = div('fill oa-plate', parent);
	const hz = ((horizon / 1080) * 100).toFixed(2);
	css(p, {
		background: [
			`radial-gradient(900px 360px at ${sun}px ${horizon}px, rgb(${rgb(INK.ember500)} / 0.85) 0%, rgb(${rgb(INK.ember600)} / 0.45) 40%, transparent 100%)`,
			`linear-gradient(180deg, ${INK.night} 0%, ${INK.ember900} 30%, ${INK.ember800} 62%, ${INK.ember600} ${hz}%, ${INK.night} calc(${hz}% + 1px), ${INK.night} 100%)`
		].join(', ')
	});
	css(div('abs', p), {
		left: '0',
		top: `${horizon}px`,
		width: '1920px',
		height: `${1080 - horizon}px`,
		background: INK.night
	});
	css(div('abs', p), {
		left: '0',
		top: `${horizon}px`,
		width: '1920px',
		height: '2px',
		background: INK.ember500
	});
	return p;
}
