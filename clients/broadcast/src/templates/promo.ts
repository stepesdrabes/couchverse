// The promo: a timeless body over the title's own soundtrack (rendered once per title), then a
// 4 s end card with the airing. The moods are the identity's three lights: cinematic is dusk
// (long takes pushed slowly, dissolves, quiet type low in the frame), grit is night
// (letterboxed, hard cuts on the beat, night cards with big words), bright is day (the brand's
// red as a field that slides across the frame, the picture framed on paper). Every picture is
// the title's own scenes where the spec has them, its artwork otherwise; where the title shows
// big it is its own logo, else its name.
//
// Section starts are nominal times snapped to the bed's onsets; runs cut on its beats. The body
// never shows anything about when the title airs: the end card's timeless layer (the night
// column, the picture, the title) enters during the body's last moments, the airing (eyebrow,
// day, time) only from the cut on, in a slot whose height never depends on it, and the tag
// lands on the sonic logo. The look is sharp and flat: right angles, colour fields and rules,
// type that rises out of its own mask; only transform and opacity move.

import type { Bus, LoadedBed } from '../audio/engine';
import { beatsIn, snapper } from '../audio/onsets';
import {
	air,
	bowed,
	glass,
	house,
	impact,
	inhale,
	pad,
	padSweep,
	piano,
	pulse,
	riser,
	shimmer,
	sub,
	type House
} from '../audio/voices';
import { channelTag } from '../brand/lockup';
import { BEAT } from '../brand/sonic';
import { css, div, img, span } from '../core/dom';
import { fitText, type FitBox } from '../core/fit';
import { framed, inset } from '../core/framing';
import { edge, motion, type Move } from '../core/moves';
import { FPS, frame } from '../core/timeline';
import type {
	Airing,
	Crop,
	EvergreenEndSpec,
	FootageShot,
	PromoContent,
	PromoSpec,
	Shot,
	Title
} from '../spec';
import { timeline, type Ctx, type Template, type TL } from './shared';

type Mood = PromoContent['mood'];

type Section =
	| { kind: 'line'; at: number; line: number; snap?: false }
	| { kind: 'montage'; at: number; word?: string }
	| { kind: 'premiere'; at: number }
	| { kind: 'title'; at: number }
	| { kind: 'open'; at: number; snap?: false }
	| { kind: 'cuts'; at: number }
	| { kind: 'card'; at: number; word: string }
	| { kind: 'low'; at: number; line: number }
	| { kind: 'slab'; at: number; line: number; snap?: false }
	| { kind: 'end'; at: number };

interface MoodSpec {
	/** Seconds between the cuts of a run (a beat; half a beat in grit), the bed's beats if any. */
	gap: number;
	/** The weakest onset a cut follows. */
	cutMin: number;
	/** A take's push, zoom from and to. */
	push: [number, number];
	/** Seconds of dissolve where a long take outlasts its footage and goes on in the next shot. */
	chain: number;
	/** Seconds before the cut the end card's timeless layer starts to enter. */
	lead: number;
	sequence: (premiere: boolean, words: string[]) => Section[];
}

const MOODS: Record<Mood, MoodSpec> = {
	cinematic: {
		gap: BEAT,
		cutMin: 0.55,
		push: [1, 1.06],
		chain: 0.6,
		lead: 0.6,
		sequence: (premiere) => [
			{ kind: 'line', at: 0, line: 0, snap: false },
			{ kind: 'line', at: 4.0, line: 1 },
			{ kind: 'montage', at: 8.0 },
			...(premiere
				? ([
						{ kind: 'premiere', at: 11.2 },
						{ kind: 'title', at: 13.6 },
						{ kind: 'end', at: 17.6 }
					] as Section[])
				: ([
						{ kind: 'title', at: 11.6 },
						{ kind: 'end', at: 16.0 }
					] as Section[]))
		]
	},
	grit: {
		gap: BEAT / 2,
		cutMin: 0.3,
		push: [1.02, 1.08],
		chain: 0,
		lead: 0,
		sequence: (premiere, w) => [
			{ kind: 'open', at: 0, snap: false },
			{ kind: 'cuts', at: 2.4 },
			{ kind: 'card', at: 4.0, word: premiere ? 'Premiere' : (w[0] ?? '') },
			{ kind: 'cuts', at: 5.2 },
			{ kind: 'title', at: 7.2 },
			{ kind: 'card', at: 9.6, word: w[1] ?? '' },
			{ kind: 'low', at: 10.4, line: 1 },
			{ kind: 'end', at: 12.8 }
		]
	},
	bright: {
		gap: BEAT,
		cutMin: 0.4,
		push: [1.02, 1.08],
		chain: 0,
		lead: 0.6,
		sequence: (premiere, w) => [
			{ kind: 'slab', at: 0, line: 0, snap: false },
			{ kind: 'montage', at: 2.8, word: premiere ? 'Premiere' : w[0] },
			{ kind: 'slab', at: 6.0, line: 1 },
			{ kind: 'title', at: 8.8 },
			{ kind: 'end', at: 12.8 }
		]
	}
};

/** How long the end card runs after the cut. */
export const END_CARD = 4;

// the body is rendered with this airing whatever the spec says, so it cannot depend on one
const BODY_AIRING: Airing = { eyebrow: 'Tonight', time: '20:15' };

const CUE: Record<Section['kind'], string> = {
	line: 'tagline over a long take',
	montage: 'montage, cuts on the beat',
	premiere: 'premiere card',
	title: 'the title',
	open: 'the opening line on night',
	cuts: 'cuts on the beat',
	card: 'word card',
	low: 'the last line over a shot',
	slab: 'tagline on the red field',
	end: 'end card (airing)'
};

const W = 1920;
const H = 1080;
const NIGHT = '5 5 5';
/** Grit's letterbox bars: 2.39:1 inside the frame, so scope scenes show whole. */
const BAR = 132;

interface View {
	x: number;
	y: number;
	w: number;
	h: number;
}

const FULL: View = { x: 0, y: 0, w: W, h: H };
const BAND: View = { x: 0, y: BAR, w: W, h: H - 2 * BAR };

/**
 * The end card's grid: a night column to x 1008 (its edge a red rule) holding the title and the
 * airing's slot, the picture right of it. The slot's height is fixed (the eyebrow's block, then
 * the time line or the lockup), so the title sits where it sits whatever the airing.
 */
const EC = {
	edge: 1008,
	rule: 8,
	x: 96,
	w: 852,
	gap: 56,
	slot: 60 + 22 + 116
};

export const promo: Template<PromoSpec> = {
	version: 5,
	build(ctx, spec) {
		const airing = spec.part === 'body' ? BODY_AIRING : (spec.airing ?? null);
		const tl = buildPromo(ctx, spec, airing);
		if (spec.part !== 'full') {
			const [a, b] = tl.parts[spec.part];
			// in a break the body's sound runs on through its end card (the break plays the full
			// promo there), and an end card's words are seen, never heard, so it is the same sound
			tl.parts = spec.part === 'body' ? { clip: [a, b], sound: tl.parts.full } : { clip: [a, b] };
		}
		return tl;
	}
};

export const evergreenEnd: Template<EvergreenEndSpec> = {
	version: 5,
	build(ctx, spec) {
		const tl = buildPromo(ctx, spec, null);
		tl.parts = { clip: tl.parts.end };
		return tl;
	}
};

function defaultWords(c: PromoContent): string[] {
	if (c.words?.length) return c.words;
	return [c.title.genres[0], c.title.year ? String(c.title.year) : undefined].filter(
		(x): x is string => !!x
	);
}

const fx = (n: number) => n.toFixed(2);

/** A fitted text's lines, each moved into a mask of its own that it rises out of. */
function maskLines(el: HTMLElement): HTMLElement[] {
	const groups: Node[][] = [[]];
	for (const n of [...el.childNodes]) {
		if (n.nodeName === 'BR') groups.push([]);
		else groups[groups.length - 1].push(n);
	}
	el.replaceChildren();
	return groups
		.filter((g) => g.length)
		.map((nodes, i) => {
			if (i) el.appendChild(document.createElement('br'));
			const first = nodes[0];
			const last = nodes[nodes.length - 1];
			if (first instanceof Text) first.data = first.data.trimStart();
			if (last instanceof Text) last.data = last.data.trimEnd();
			const inner = span('pm-in', span('pm-mask', el));
			inner.append(...nodes);
			return inner;
		});
}

/** Type or a logo that rises into place out of its masks. */
interface Rising {
	el: HTMLElement;
	parts: HTMLElement[];
	/** Where each part starts below its mask. */
	from: string;
	/** Its height on the stage, px. */
	h: number;
	/** Its widest line (a logo's width), px. */
	w: number;
}

/** Spec text fitted to its box at `size` (the class's own size by default), in line masks. */
function fitted(parent: Element, cls: string, text: string, box: FitBox, size?: number): Rising {
	const el = div(cls, parent, text);
	if (size) el.style.fontSize = `${size}px`;
	fitText(el, box);
	const parts = maskLines(el);
	return {
		el,
		parts,
		from: '1.3em',
		h: el.offsetHeight,
		w: Math.max(...parts.map((p) => p.offsetWidth))
	};
}

interface LogoBox {
	/** The widest and tallest it may be, px. */
	w: number;
	h: number;
	/** The area it takes when neither bound applies: logos of every shape weigh alike. */
	area: number;
}

/**
 * The title as it shows big: its logo, sized by its aspect within the box and never stretched
 * (the same area for a 4:1 wordmark and a 2:1 stack, so neither looks the bigger), else its
 * name in type, fitted.
 */
function titleMark(
	parent: Element,
	t: Title,
	logoBox: LogoBox,
	cls: string,
	box: FitBox,
	size?: number
): Rising {
	if (!t.logo) return fitted(parent, cls, t.name, box, size);
	const aspect = t.logo.aspect;
	let w = Math.min(logoBox.w, Math.sqrt(logoBox.area * aspect));
	if (w / aspect > logoBox.h) w = logoBox.h * aspect;
	w = Math.round(w);
	const h = Math.round(w / aspect);
	const el = css(div('pm-logo', parent), { width: `${w + 24}px`, height: `${h + 24}px` });
	const im = css(img(null, el, t.logo.url), { width: `${w}px`, height: `${h}px` });
	im.alt = t.name;
	return { el, parts: [im], from: `${h + 16}px`, h, w };
}

/** Parts rise into place out of their masks, one after another. */
function rise(tl: TL, r: Pick<Rising, 'parts' | 'from'>, at: number, stagger = 0.09, dur = 0.95) {
	r.parts.forEach((p, i) =>
		tl.add(
			p,
			[{ transform: `translateY(${r.from})` }, { transform: 'translateY(0px)' }],
			at + i * stagger,
			dur,
			'glide'
		)
	);
}

/** Parts leave upward through the tops of their masks. */
function sink(tl: TL, r: Pick<Rising, 'parts' | 'from'>, at: number, stagger = 0.04, dur = 0.42) {
	const up = r.from === '1.3em' ? '-1.36em' : `-${r.from}`;
	r.parts.forEach((p, i) =>
		tl.add(
			p,
			[{ transform: 'translateY(0px)' }, { transform: `translateY(${up})` }],
			at + i * stagger,
			dur,
			'leave'
		)
	);
}

/** The year, two genres and the runtime, split by small squares. */
function metaLine(parent: Element, t: Title) {
	const el = div('pm-meta', parent);
	[
		t.year ? String(t.year) : '',
		...t.genres.slice(0, 2),
		t.runtimeMinutes ? `${t.runtimeMinutes} min` : ''
	]
		.filter(Boolean)
		.forEach((x, i) => {
			if (i) span('sep', el);
			span(null, el, x);
		});
	return el;
}

/** A static grade: one full-frame layer, shown only while its section is. */
const shade = (parent: Element, background: string) => css(div('pm-fill', parent), { background });

/** A centred column over the frame. */
const centred = (parent: Element) =>
	css(div('pm-fill pm-center', parent), {
		display: 'flex',
		flexDirection: 'column',
		alignItems: 'center',
		justifyContent: 'center'
	});

const place = (e: HTMLElement, v: View) =>
	css(e, { left: `${v.x}px`, top: `${v.y}px`, width: `${v.w}px`, height: `${v.h}px` });

/** A title without artwork: the channel paints the dusk over a low horizon, never a grey box. */
function plate(parent: Element, v: View) {
	const p = place(div('pm-take', parent), v);
	const hz = Math.round(v.h * 0.65);
	const sun = Math.round(v.w * 0.7);
	css(div('pm-fill', p), {
		background: [
			`radial-gradient(${Math.round(v.w * 0.47)}px ${Math.round(v.h * 0.33)}px at ${sun}px ${hz}px, rgb(122 10 20 / 0.85), rgb(86 6 16 / 0.45) 40%, transparent) 0 0 / 100% ${hz}px no-repeat`,
			`radial-gradient(${Math.round(v.w * 0.57)}px ${Math.round(v.h * 0.2)}px at ${sun}px 0px, rgb(58 4 9 / 0.55), transparent) 0 ${hz + 2}px / 100% ${v.h - hz - 2}px no-repeat`,
			`linear-gradient(180deg, #050505 0%, #140203 30%, #260306 62%, #560610 ${hz}px, #7a0a14 ${hz}px, #7a0a14 ${hz + 2}px, #050505 ${hz + 2}px)`
		].join(', ')
	});
	return p;
}

type CueFn = (at: number, fn: (h: House, w: number) => void, dur?: number) => void;

/** The whole promo (body and end card); `airing` null is the evergreen end card. */
function buildPromo({ root, sound, assets }: Ctx, spec: PromoContent, airing: Airing | null): TL {
	const t = spec.title;
	const mood = spec.mood;
	const P = MOODS[mood];
	const grit = mood === 'grit';
	const bright = mood === 'bright';
	const bed: LoadedBed | null = assets.bed(spec.bed);
	const onsets = bed?.onsets ?? [];
	const snap = snapper(onsets);
	const lines = t.taglines ?? [];

	const nominal = P.sequence(!!spec.premiere, defaultWords(spec));
	const seq = nominal.map((s) => ({
		...s,
		at: 'snap' in s && s.snap === false ? s.at : snap(s.at, 0.7, 0.5)
	})) as (Section & { end: number })[];
	// the cut to the end card lands on the frame grid (the body and the end are separate clips)
	const last = seq[seq.length - 1];
	last.at = frame(last.at);
	const B = last.at;
	const D = frame(B + END_CARD);
	seq.forEach((s, i) => (s.end = i + 1 < seq.length ? seq[i + 1].at : D));
	const titleAt = seq.find((s) => s.kind === 'title')?.at ?? B;

	const tl = timeline(root, D);
	tl.parts = { body: [0, B], end: [B, D], full: [0, D] };
	root.classList.add('pm', `pm-${mood}`);

	const pics = div('pm-fill', root);
	const grades = div('pm-fill', root);
	const fields = div('pm-fill', root);
	const text = div('pm-fill', root);
	const endLayer = div('pm-fill', root);
	const bars = grit
		? [
				css(div('pm-letterbox', root), { top: '0', transformOrigin: '50% 0' }),
				css(div('pm-letterbox', root), { bottom: '0', transformOrigin: '50% 100%' })
			]
		: [];
	const view = grit ? BAND : FULL;

	// --- pictures --------------------------------------------------------------------------

	const pool: Shot[] = spec.shots.length
		? spec.shots
		: [{ kind: 'still', image: 'backdrop', from: [0, 0, 1, 1] }];
	let poolAt = 0;
	const nextShot = () => pool[poolAt++ % pool.length];
	// where each scene was left (by its first frame: the title's footage and the body's shots are
	// the same scenes), so a scene used again goes on rather than repeating
	const cursor = new Map<string, number>();
	const key = (s: FootageShot) => s.frames[0];
	const left = (s: Shot) =>
		s.kind === 'footage' ? s.frames.length - (cursor.get(key(s)) ?? 0) : 0;
	let playing: string | null = null;
	/** The scene with the most left to show (not the one on screen), for a long take. */
	const freshest = (from: Shot[]) => {
		let best: Shot | undefined;
		for (const s of from)
			if (s.kind === 'footage' && key(s) !== playing && (!best || left(s) > left(best))) best = s;
		return best;
	};

	interface TakeOptions {
		view?: View;
		parent?: HTMLElement;
		/** Seconds of dissolve centred on the cut (0: a cut). */
		dissolve?: number;
		/** Seconds it fades up from night (the clip's first picture). */
		fade?: number;
		push?: [number, number];
		pushEase?: string;
	}

	/** Seconds of a shot a take of `need` seconds can show (footage runs out; stills never do). */
	const available = (s: Shot, need: number) => {
		if (s.kind !== 'footage') return need;
		const n = s.frames.length;
		const from = cursor.get(key(s)) ?? 0;
		return (n - from) / FPS >= need ? need : Math.min(need, n / FPS);
	};

	/** One take of a shot over [a, b): footage frame by frame, or a still's slow move. */
	function take(a: number, b: number, s: Shot, o: TakeOptions = {}) {
		const v = o.view ?? view;
		const parent = o.parent ?? pics;
		const d = a > 0 ? (o.dissolve ?? 0) : 0;
		const inT = a - d / 2;
		const outT = b + d / 2;
		const [z0, z1] = o.push ?? P.push;
		let box: HTMLElement;
		if (s.kind === 'footage') {
			box = place(div('pm-take', parent), v);
			const im = img('pm-footage', box);
			const [x, y, w, h] = s.focus ?? [0, 0, 1, 1];
			const focus = `${fx((x + w / 2) * 100)}% ${fx((y + h / 2) * 100)}%`;
			css(im, { objectPosition: focus, transformOrigin: focus });
			const need = Math.ceil((outT - inT) * FPS) + 1;
			let from = cursor.get(key(s)) ?? 0;
			if (from + need > s.frames.length) from = 0;
			cursor.set(key(s), Math.min(s.frames.length, from + need));
			playing = key(s);
			tl.footage(im, s.frames.slice(from), inT);
			tl.add(
				im,
				[{ transform: `scale(${z0})` }, { transform: `scale(${z1})` }],
				inT,
				outT - inT,
				o.pushEase ?? 'linear'
			);
		} else {
			const url = s.image === 'poster' ? t.poster : t.backdrop;
			if (!url) box = plate(parent, v);
			else {
				box = place(div('pm-take', parent), v);
				const f = framed(box, assets.picture(url), { w: v.w, h: v.h }, 'pm-still');
				const to: Crop = s.to ?? inset(s.from, 0.9);
				tl.add(
					f.img,
					[{ transform: f.tf(s.from, z0) }, { transform: f.tf(to, z1) }],
					inT,
					outT - inT,
					o.pushEase ?? 'linear'
				);
			}
		}
		tl.show(box, edge(inT), edge(outT));
		if (d) tl.add(box, [{ opacity: 0 }, { opacity: 1 }], inT, d, 'sine');
		else if (o.fade) tl.add(box, [{ opacity: 0 }, { opacity: 1 }], a, o.fade, 'sine');
		return box;
	}

	/** Cut points in [a, b): the bed's beats, or a steady grid without a bed. */
	const cutsIn = (a: number, b: number, gap: number) => {
		if (onsets.length) return [a, ...beatsIn(onsets, a, b, gap * 0.9, P.cutMin)];
		const out = [a];
		for (let i = 1; frame(a + i * gap) < b - gap * 0.6; i++) out.push(frame(a + i * gap));
		return out;
	};

	/**
	 * Takes over [a, b) cut every `gap` (0: one long take), the first of `first` when given. A
	 * take never outlasts its footage: a long take of a short shot goes on in the next one.
	 */
	function run(
		a: number,
		b: number,
		gap: number,
		o: TakeOptions & { first?: Shot; shots?: () => Shot; chain?: number } = {}
	) {
		const cuts = gap > 0 ? cutsIn(a, b, gap) : [a];
		let first = o.first;
		cuts.forEach((c, i) => {
			const e = i + 1 < cuts.length ? cuts[i + 1] : b;
			let at = c;
			let n = 0;
			while (e - at > 0.02) {
				const s = first ?? (o.shots ?? nextShot)();
				first = undefined;
				let len = available(s, e - at);
				// leave the take after it long enough to read
				if (len < e - at && e - at - len < 0.6 && e - at > 1.4) len = e - at - 0.6;
				const end = len >= e - at - 1e-6 ? e : Math.max(frame(at + len), frame(at) + 1 / FPS);
				take(at, end, s, {
					...o,
					dissolve: n === 0 ? (i === 0 ? o.dissolve : 0) : (o.chain ?? P.chain),
					fade: i === 0 && n === 0 ? o.fade : 0
				});
				at = end;
				n++;
			}
		});
	}

	// --- the sections ------------------------------------------------------------------------

	const night = (a: number, b: number) => {
		const f = div('pm-fill pm-night', fields);
		tl.show(f, edge(a), edge(b));
		return f;
	};

	// bright's red field: one sheet that slides across the frame between its slabs
	let red: HTMLElement | null = null;
	const redMoves: Move<number>[] = [];

	for (const s of seq) {
		const a = s.at;
		const b = s.end;
		tl.cue(s.kind === 'end' && !airing ? 'end card (coming up)' : CUE[s.kind], a);
		switch (s.kind) {
			case 'line': {
				run(a, b, 0, { dissolve: a ? 0.6 : 0, fade: a ? 0 : 1.2 });
				const line = lines[s.line];
				if (!line) break;
				// a foot shade lifts quiet type from any picture
				const g = shade(
					grades,
					`linear-gradient(0deg, rgb(${NIGHT} / 0.75) 0%, rgb(${NIGHT} / 0.3) 24%, rgb(${NIGHT} / 0) 45%)`
				);
				tl.show(g, edge(a), edge(b));
				if (a > 0) tl.add(g, [{ opacity: 0 }, { opacity: 1 }], a, 0.5, 'sine');
				tl.add(g, [{ opacity: 1 }, { opacity: 0 }], b - 0.5, 0.5, 'sine');
				const box = css(div('pm-fill pm-center', text), {
					top: 'auto',
					bottom: '150px',
					left: '96px',
					right: '96px'
				});
				tl.show(box, edge(a), edge(b));
				const r = fitted(box, 'pm-headline', line, { width: 1600, min: 48, lines: 2 });
				rise(tl, r, a + (a ? 0.5 : 1.0), 0.1, 1.0);
				sink(tl, r, b - 0.5);
				break;
			}
			case 'montage': {
				run(a, b, P.gap, { dissolve: bright ? 0 : 0.4, push: [1.02, 1.08] });
				if (bright && s.word) redBlock(a, b, s.word);
				break;
			}
			case 'premiere': {
				night(a, b);
				// the dusk rising behind the word
				const dusk = shade(
					fields,
					'radial-gradient(1500px 640px at 960px 1240px, rgb(86 6 16 / 0.85), rgb(38 3 6 / 0.64) 38%, rgb(20 2 3 / 0.34) 62%, transparent)'
				);
				tl.show(dusk, edge(a), edge(b));
				tl.add(dusk, [{ opacity: 0 }, { opacity: 1 }], a, 1.2, 'sine');
				const box = centred(text);
				tl.show(box, edge(a), edge(b));
				const r = fitted(box, 'pm-display1', 'Premiere', { width: 1640, min: 120 }, 184);
				const rule = css(div('pm-rule', box), { width: '520px', marginTop: '28px' });
				rise(tl, r, a + 0.2, 0, 1.0);
				tl.add(
					rule,
					[{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }],
					a + 0.5,
					1.2,
					'glide'
				);
				sink(tl, r, b - 0.4);
				tl.add(
					rule,
					[{ transform: 'scaleX(1)' }, { transform: 'scaleX(0)' }],
					b - 0.4,
					0.36,
					'leave'
				);
				break;
			}
			case 'title':
				if (bright && !t.logo) brightTitle(a, b);
				else if (bright) plateTitle(a, b);
				else titleCard(a, b);
				break;
			case 'open': {
				const line = lines[0];
				if (!line) {
					run(a, b, P.gap, { fade: 0.3 });
					break;
				}
				const box = centred(text);
				tl.show(box, edge(a), edge(b));
				const r = fitted(box, 'pm-shout', line, { width: 1500, min: 56, lines: 2 });
				rise(tl, r, a + 0.3, 0.1, 0.8);
				break;
			}
			case 'cuts':
				run(a, b, P.gap);
				break;
			case 'card': {
				if (!s.word) {
					run(a, b, P.gap);
					break;
				}
				night(a, b);
				const box = centred(text);
				tl.show(box, edge(a), edge(b));
				const inner = css(div(null, box), { display: 'flex', flexDirection: 'column' });
				const r = fitted(inner, 'pm-word', s.word, { width: 1640, min: 120 });
				const rule = css(div('pm-rule', inner), {
					width: `${Math.round(r.w)}px`,
					marginTop: '18px',
					transformOrigin: '0 50%'
				});
				// cut in whole on the beat; the rule draws itself under the word
				tl.add(
					rule,
					[{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }],
					a + 0.04,
					0.5,
					'glide'
				);
				break;
			}
			case 'low': {
				run(a, b, 0, { push: [1.06, 1.1] });
				const line = lines[s.line];
				if (!line) break;
				const g = shade(grades, `rgb(${NIGHT} / 0.5)`);
				tl.show(g, edge(a), edge(b));
				const box = css(div('pm-fill pm-center', text), {
					top: 'auto',
					bottom: '220px',
					left: '96px',
					right: '96px'
				});
				tl.show(box, edge(a), edge(b));
				const r = fitted(box, 'pm-headline', line, { width: 1600, min: 48, lines: 2 });
				rise(tl, r, a + 0.3, 0.1, 0.8);
				break;
			}
			case 'slab': {
				const line = lines[s.line];
				if (!line) {
					run(a, b, P.gap, { fade: a ? 0 : 0.32 });
					break;
				}
				if (!red) {
					red = div('pm-fill pm-red', fields);
					if (a === 0) tl.add(red, [{ opacity: 0 }, { opacity: 1 }], 0, 0.32, 'sine');
				}
				// the field slides in from the right over what plays, and away to the left on the
				// beat the next section starts on, uncovering it
				if (a > 0) redMoves.push({ at: a - 0.4, dur: 0.55, to: 0, ease: 'move' });
				redMoves.push({ at: b, dur: 0.55, to: -W, ease: 'move' });
				redMoves.push({ at: b + 0.6, to: W });
				const box = css(div('pm-fill pm-center', red), {
					display: 'flex',
					alignItems: 'center',
					justifyContent: 'center'
				});
				tl.show(box, edge(a - 0.45), edge(b + 0.6));
				const r = fitted(box, 'pm-display2', line, { width: 1500, min: 80, lines: 2 });
				rise(tl, r, a + 0.3, 0.08, 0.9);
				break;
			}
			case 'end':
				endCard(a);
				break;
		}
	}
	if (red) {
		const first = seq.find((s) => s.kind === 'slab' && lines[s.line]);
		motion(tl, red, first && first.at === 0 ? 0 : W, redMoves, (v) => ({
			transform: `translateX(${v}px)`
		}));
	}

	/** Bright's first word: a red block opens on the montage and holds it. */
	function redBlock(a: number, b: number, word: string) {
		const size = 660;
		const blk = css(div('pm-fill', text), {
			left: `${960 - size / 2}px`,
			top: `${540 - size / 2}px`,
			width: `${size}px`,
			height: `${size}px`,
			display: 'flex',
			alignItems: 'center',
			justifyContent: 'center'
		});
		tl.show(blk, edge(a + 0.8), edge(b));
		const fill = css(div('pm-fill pm-red', blk), { transformOrigin: '0 50%' });
		tl.add(
			fill,
			[{ transform: 'translateX(0px) scaleX(0)' }, { transform: 'translateX(0px) scaleX(1)' }],
			a + 0.8,
			0.5,
			'glide'
		);
		// it closes toward its right edge, the way the field leaves
		tl.add(
			fill,
			[
				{ transform: 'translateX(0px) scaleX(1)' },
				{ transform: `translateX(${size}px) scaleX(0)` }
			],
			b - 0.5,
			0.42,
			'leave'
		);
		const wordBox = css(div('pm-center', blk), { position: 'relative' });
		const r = fitted(wordBox, 'pm-display2', word, { width: 560, min: 72 }, 120);
		rise(tl, r, a + 1.0, 0.08, 0.8);
		sink(tl, r, b - 0.62, 0.04, 0.3);
	}

	/**
	 * Bright's title with a logo, still in daylight: the scene undimmed, the logo on a sharp night
	 * plate (a title's logo is drawn for the dark; on paper a yellow one would vanish) that opens
	 * from the left as the red field leaves that way.
	 */
	function plateTitle(a: number, b: number) {
		run(a, b, 0, {
			first: spec.hero ?? freshest(pool) ?? nextShot(),
			shots: () => freshest(pool) ?? nextShot(),
			push: [1.04, 1.1]
		});
		const box = centred(text);
		tl.show(box, edge(a), edge(b));
		const plate = css(div(null, box), {
			position: 'relative',
			display: 'flex',
			flexDirection: 'column',
			alignItems: 'center',
			padding: '64px 72px 52px'
		});
		const back = css(div('pm-fill pm-night', plate), { transformOrigin: '0 50%' });
		const mark = titleMark(plate, t, { w: 1000, h: 300, area: 220000 }, 'pm-display1', {
			width: 1500,
			min: 96,
			lines: 3
		});
		mark.el.style.position = 'relative';
		const rule = css(div('pm-rule', plate), {
			width: '96px',
			marginTop: '34px',
			position: 'relative'
		});
		const meta = css(metaLine(plate, t), { marginTop: '22px', position: 'relative' });
		tl.add(back, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], a, 0.6, 'glide');
		rise(tl, mark, a + 0.22, 0.09, 0.95);
		tl.add(rule, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], a + 0.55, 0.8, 'glide');
		tl.add(
			meta,
			[
				{ opacity: 0, transform: 'translateY(14px)' },
				{ opacity: 1, transform: 'translateY(0px)' }
			],
			a + 0.7,
			0.7,
			'glide'
		);
	}

	/** The title over its hero shot: the logo (or the name) rising, a red rule, the meta line. */
	function titleCard(a: number, b: number) {
		run(a, b, 0, {
			first: spec.hero ?? freshest(pool) ?? nextShot(),
			shots: () => freshest(pool) ?? nextShot(),
			dissolve: mood === 'cinematic' ? 0.4 : 0,
			push: [1.04, 1.1]
		});
		// the dim, and a little more behind the title so it reads over the brightest scene
		const dim = shade(
			grades,
			`radial-gradient(1500px 640px at 960px 540px, rgb(${NIGHT} / 0.3), rgb(${NIGHT} / 0) 72%), rgb(${NIGHT} / ${grit ? 0.62 : 0.5})`
		);
		const soft = mood === 'cinematic';
		tl.show(dim, edge(soft ? a - 0.2 : a), edge(b));
		if (soft) tl.add(dim, [{ opacity: 0 }, { opacity: 1 }], a - 0.2, 0.4, 'sine');
		const box = centred(text);
		tl.show(box, edge(a), edge(b));
		const group = css(div(null, box), {
			display: 'flex',
			flexDirection: 'column',
			alignItems: 'center'
		});
		const mark = titleMark(group, t, { w: 1100, h: 360, area: 300000 }, 'pm-display1', {
			width: 1640,
			min: 104,
			lines: 3,
			height: 560
		});
		const rule = css(div('pm-rule', group), {
			width: '96px',
			marginTop: t.logo ? '40px' : '34px'
		});
		const meta = metaLine(group, t);
		css(meta, { marginTop: '26px' });
		// grit cuts it in whole on the beat, rule and all
		if (grit) return;
		rise(tl, mark, a + 0.3, 0.1, 1.0);
		tl.add(rule, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], a + 0.7, 0.9, 'glide');
		tl.add(
			meta,
			[
				{ opacity: 0, transform: 'translateY(14px)' },
				{ opacity: 1, transform: 'translateY(0px)' }
			],
			a + 0.9,
			0.7,
			'glide'
		);
		// the type pushes with the picture, a little slower: the frame has depth
		tl.add(group, [{ transform: 'scale(1)' }, { transform: 'scale(1.03)' }], a, b - a, 'linear');
		sink(tl, mark, b - 0.45);
		tl.add(rule, [{ transform: 'scaleX(1)' }, { transform: 'scaleX(0)' }], b - 0.45, 0.36, 'leave');
		tl.add(meta, [{ opacity: 1 }, { opacity: 0 }], b - 0.45, 0.3, 'leave');
	}

	/** Bright's title without a logo, in daylight: the name in night on paper, the picture framed. */
	function brightTitle(a: number, b: number) {
		const field = div('pm-fill pm-paper', fields);
		tl.show(field, edge(a - 0.3), edge(b + 1));
		// the frame bleeds off the right edge and slides in as the red field slides away
		const fv: View = { x: 1004, y: 130, w: W - 1004, h: 820 };
		const frameBox = place(div('pm-take', fields), fv);
		tl.show(frameBox, edge(a - 0.3), edge(b + 1));
		const inner = div('pm-fill', frameBox);
		tl.add(
			frameBox,
			[{ transform: `translateX(${fv.w}px)` }, { transform: 'translateX(0px)' }],
			a - 0.1,
			1.0,
			'glide'
		);
		tl.add(
			inner,
			[{ transform: `translateX(${-fv.w / 2}px)` }, { transform: 'translateX(0px)' }],
			a - 0.1,
			1.0,
			'glide'
		);
		run(a - 0.1, b + 1, 0, {
			first: spec.hero ?? freshest(pool) ?? nextShot(),
			shots: () => freshest(pool) ?? nextShot(),
			view: { x: 0, y: 0, w: fv.w, h: fv.h },
			parent: inner,
			push: [1.12, 1.0],
			pushEase: 'glide'
		});
		const col = css(div('pm-fill', text), {
			left: '96px',
			width: '860px',
			display: 'flex',
			flexDirection: 'column',
			justifyContent: 'center'
		});
		tl.show(col, edge(a - 0.3), edge(b + 1));
		const r = fitted(col, 'pm-display1', t.name, { width: 860, min: 96, lines: 3 });
		css(r.el, { color: '#050505' });
		const meta = css(metaLine(col, t), { color: '#5f5a55', marginTop: '26px' });
		meta.querySelectorAll<HTMLElement>('.sep').forEach((e) => (e.style.background = '#a39d96'));
		rise(tl, r, a + 0.25, 0.09, 0.95);
		tl.add(
			meta,
			[
				{ opacity: 0, transform: 'translateY(14px)' },
				{ opacity: 1, transform: 'translateY(0px)' }
			],
			a + 0.6,
			0.7,
			'glide'
		);
	}

	// --- the end card --------------------------------------------------------------------------

	function endCard(at: number) {
		const lead = P.lead;
		const T = at - lead;
		const layer = endLayer;
		tl.show(layer, edge(T), D + 1);

		// the picture: the title's own scenes, else the body's, else its backdrop held still (a
		// still that moved would cost every frame of a clip rendered for every airing)
		const footage = pool.filter((x) => x.kind === 'footage');
		const held: Shot =
			spec.hero?.kind === 'still'
				? { ...spec.hero, to: spec.hero.from }
				: { kind: 'still', image: 'backdrop', from: [0.2, 0, 0.8, 1], to: [0.2, 0, 0.8, 1] };
		const scenes: Shot[] = t.footage?.length ? t.footage : footage.length ? footage : [held];
		const still = !scenes.some((x) => x.kind === 'footage');
		let k = 0;
		const picBox = place(div('pm-take', layer), { x: EC.edge, y: 0, w: W - EC.edge, h: H });
		const picIn = div('pm-fill', picBox);
		run(T, D + 0.04, 0, {
			first: still ? scenes[0] : undefined,
			shots: () => freshest(scenes) ?? scenes[k++ % scenes.length],
			view: { x: 0, y: 0, w: W - EC.edge, h: H },
			parent: picIn,
			push: still ? [1, 1] : [1.0, 1.05],
			chain: grit ? 0 : 0.5
		});
		// the foot falls to night under the tag
		shade(
			picBox,
			`linear-gradient(0deg, rgb(${NIGHT} / 0.85) 0%, rgb(${NIGHT} / 0.45) 16%, rgb(${NIGHT} / 0) 32%)`
		);

		const col = css(div('pm-fill pm-night', layer), { width: `${EC.edge}px` });
		// a quiet dusk low in the column, and its edge a red rule
		shade(
			col,
			'radial-gradient(1300px 560px at 400px 1220px, rgb(86 6 16 / 0.62), rgb(38 3 6 / 0.45) 38%, rgb(20 2 3 / 0.22) 62%, transparent)'
		);
		css(div('pm-red', col), {
			position: 'absolute',
			top: '0',
			bottom: '0',
			right: '0',
			width: `${EC.rule}px`
		});

		if (lead) {
			// two sharp panels close in: the night column from the left, the picture from the right
			// with its scene moving at half the speed
			const close = (e: HTMLElement, from: number) =>
				tl.add(
					e,
					[{ transform: `translateX(${from}px)` }, { transform: 'translateX(0px)' }],
					T,
					1.1,
					'glide'
				);
			close(col, -EC.edge);
			close(picBox, W - EC.edge);
			close(picIn, -(W - EC.edge) / 2);
		}

		// the title, then the airing's slot of fixed height, centred on the frame
		const group = css(div(null, layer), {
			position: 'absolute',
			left: `${EC.x}px`,
			width: `${EC.w}px`
		});
		const mark = titleMark(group, t, { w: EC.w, h: 300, area: 133000 }, 'pm-ec-title', {
			width: EC.w,
			min: 60,
			lines: 3,
			height: 330
		});
		const top = Math.round((H - (mark.h + EC.gap + EC.slot)) / 2 - 10);
		group.style.top = `${top}px`;
		rise(tl, mark, grit ? at + 0.1 : T + 0.3, 0.09, 0.95);

		for (const bar of bars)
			tl.add(bar, [{ transform: 'scaleY(1)' }, { transform: 'scaleY(0)' }], at, 0.6, 'move');

		// the airing: nothing of it exists before the cut
		const slot = css(div(null, layer), {
			position: 'absolute',
			left: `${EC.x}px`,
			top: `${top + mark.h + EC.gap}px`,
			width: `${EC.w}px`
		});
		tl.show(slot, edge(at), D + 1);
		const block = div('pm-block', slot);
		const blockFill = div('pm-red', block);
		const label = fitted(block, 'pm-label', airing ? airing.eyebrow : 'Coming up', {
			width: EC.w - 40,
			min: 24
		});
		tl.add(
			blockFill,
			[{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }],
			at + 0.05,
			0.45,
			'glide'
		);
		rise(tl, label, at + 0.14, 0, 0.7);
		let tagAt: number;
		if (airing) {
			const when = css(div('pm-when', slot), { marginTop: '22px' });
			const parts = [
				airing.day ? span('pm-day', when, airing.day) : null,
				span(null, when, airing.time)
			];
			fitText(when, { width: EC.w, min: 88 });
			const inner = parts
				.filter((p): p is HTMLSpanElement => !!p)
				.map((p) => {
					const m = span('pm-mask', when);
					when.insertBefore(m, p);
					m.appendChild(p);
					p.classList.add('pm-in');
					return p;
				});
			rise(tl, { parts: inner, from: '1.3em' }, at + 0.25, 0.1, 0.9);
			tagAt = at + 1.3;
			channelTag(tl, layer, tagAt, sound, { gain: grit ? 0.62 : 0.5 });
		} else {
			// evergreen: the lockup lands in the airing's place, the last word of "Coming up"
			const lock = css(div(null, slot), { marginTop: '28px', position: 'relative' });
			tagAt = at + 1.0;
			channelTag(tl, lock, tagAt, sound, { height: 104, cls: 'pm-ec-lockup', gain: 0.55 });
		}

		const out = div('pm-fill pm-night', root);
		tl.add(out, [{ opacity: 0 }, { opacity: 1 }], D - 0.32, 0.32, 'sine');

		score(tagAt);
	}

	// --- sound ----------------------------------------------------------------------------------

	function score(tagAt: number) {
		const bedOpts = grit
			? { fadeIn: 0.05, fadeOut: 0.08, end: tagAt - BEAT, gain: 1 }
			: {
					fadeIn: 0.2,
					fadeOut: 1.4,
					end: D,
					gain: 1,
					ducks: [
						...seq
							.filter((s) => s.kind === 'premiere')
							.map((s) => ({ at: s.at, depth: 0.7, hold: 1.6, attack: 0.4, release: 0.6 })),
						{ at: tagAt, depth: 0.4, hold: 1.4, attack: 0.25, release: 0.8 }
					]
				};
		tl.sound(0, (w, bus: Bus, skip) => sound.bed(bus, w, bed, skip, bedOpts), D);
		if (grit) tl.cue('the bed stops a beat before the tag', tagAt - BEAT);
		const cue: CueFn = (at, fn, dur = 0) =>
			tl.sound(at, (w, b, skip) => fn(house(sound, b), w - skip), dur);
		if (bed) punctuate(cue);
		else if (mood === 'cinematic') cinematicScore(cue, tagAt);
		else if (grit) gritScore(cue, tagAt);
		else brightScore(cue, tagAt);
	}

	/**
	 * Over a title's own bed the house only punctuates, unpitched and well under it (the bed's
	 * key is anyone's guess): air into the changes, a soft landing where the title arrives.
	 */
	function punctuate(cue: CueFn) {
		for (const s of seq) {
			if (s.kind === 'title') {
				cue(s.at - 0.9, (h, w) =>
					air(h, w, 1.3, { peakAt: 0.9, band: [300, 2200], gain: 0.25, seed: 51 })
				);
				cue(s.at, (h, w) => impact(h, w, { gain: grit ? 0.32 : 0.22, root: 'D1', seed: 4250 }));
			} else if (s.kind === 'premiere') {
				cue(s.at - 0.8, (h, w) => inhale(h, w + 0.8, 0.8, { gain: 0.3, seed: 52 }));
				cue(s.at, (h, w) => impact(h, w, { gain: 0.24, root: 'D1', seed: 4251 }));
			} else if (s.kind === 'card' && s.word) {
				cue(s.at, (h, w) => impact(h, w, { gain: 0.3, root: 'D1', seed: 4252 + Math.round(s.at) }));
			} else if (bright && (s.kind === 'slab' || s.kind === 'montage') && s.at > 0) {
				// the red field crossing the frame
				cue(s.at - 0.4, (h, w) =>
					air(h, w, 0.8, {
						peakAt: 0.4,
						band: [500, 3200],
						gain: 0.2,
						pans: s.kind === 'slab' ? [0.6, -0.4] : [-0.4, 0.6],
						seed: 53
					})
				);
			}
		}
		if (!grit)
			cue(B - P.lead - 0.2, (h, w) =>
				air(h, w, 1.4, { peakAt: 0.6, band: [300, 1800], gain: 0.2, pans: [-0.5, 0.5], seed: 55 })
			);
	}

	/**
	 * Without a bed the house is the promo's music, at the house tempo in D. Cinematic: a slow
	 * swell from D through B minor and G to the dominant, landing on D with the title; the felt
	 * piano a few notes into the hall, never the sonic logo's figure; the end card's pad leaves
	 * the D to the tag.
	 */
	function cinematicScore(cue: CueFn, tagAt: number) {
		const montage = seq.find((s) => s.kind === 'montage');
		const prem = seq.find((s) => s.kind === 'premiere');
		const m0 = montage?.at ?? 8;
		const build = prem?.at ?? titleAt;
		cue(
			0,
			(h, w) =>
				padSweep(h, w, [
					[0, 420],
					[m0, 1600],
					[build, 3600],
					[titleAt + 1.6, 2400],
					[D, 900]
				]),
			D
		);
		const chord = (at: number, dur: number, notes: string[], level: number, seed: number) =>
			cue(
				at,
				(h, w) => pad(h, w, notes, dur, { attack: 1.2, release: 1.6, level, seed: 600 + seed }),
				dur
			);
		chord(0, 3.4, ['D2', 'A2', 'D3', 'F#3', 'A3', 'E4'], 0.8, 1);
		chord(4.0, 3.4, ['B1', 'F#2', 'B2', 'D3', 'F#3', 'A3'], 0.85, 2);
		chord(m0, Math.max(0.4, build - m0 - 1.4), ['G1', 'D2', 'G2', 'B2', 'D3', 'F#3', 'A3'], 0.9, 3);
		const keys: [number, string][] = [
			[1.0, 'F#4'],
			[2.6, 'A4'],
			[3.4, 'E4'],
			[4.6, 'D4'],
			[6.2, 'F#4'],
			[7.0, 'B3'],
			[m0, 'G3'],
			[m0 + 0.8, 'D4'],
			[m0 + 1.6, 'B4'],
			[m0 + 2.4, 'A4']
		];
		keys.forEach(([at, n], i) =>
			cue(at, (h, w) =>
				piano(h, w, n, {
					vel: 0.42,
					dur: 2.6,
					pan: (i % 3) * 0.12 - 0.12,
					verb: 0.75,
					gain: 0.9,
					seed: 640 + i
				})
			)
		);
		cue(4.0, (h, w) =>
			bowed(h, w, 'F#3', 2.6, { gain: 0.7, attack: 0.9, release: 1.2, verb: 0.6, seed: 660 })
		);
		cue(m0, (h, w) => sub(h, w, 'G1', 1.2, { gain: 0.28, attack: 0.4, release: 1.0 }));
		// the build: the dominant swells, the arrival rises reversed into it
		cue(
			build - 2.4,
			(h, w) =>
				pad(h, w, ['A1', 'E2', 'A2', 'D3', 'E3', 'A3'], 1.0, {
					attack: 1.2,
					level: 1.0,
					cut: true,
					seed: 670
				}),
			2.4
		);
		cue(build - 1.6, (h, w) =>
			riser(h, w + 1.6, ['A2', 'E3', 'A3', 'E4', 'A4'], 1.6, { gain: 0.8, breath: 0.04, seed: 671 })
		);
		if (prem) {
			cue(prem.at, (h, w) => impact(h, w, { gain: 0.28, root: 'A1', seed: 4260 }));
			cue(
				prem.at,
				(h, w) =>
					pad(h, w, ['A1', 'E2', 'A2', 'C#3', 'E3', 'B3'], 1.2, {
						attack: 0.2,
						release: 1.4,
						level: 0.6,
						seed: 672
					}),
				2
			);
			cue(prem.at + 0.2, (h, w) =>
				glass(h, w, 'E5', { decay: 2.2, gain: 0.8, pan: 0.15, verb: 0.7, seed: 673 })
			);
			cue(prem.at + 0.6, (h, w) =>
				bowed(h, w, 'A3', 1.2, { gain: 0.8, attack: 0.5, release: 1.0, seed: 674 })
			);
			cue(titleAt - 0.8, (h, w) => inhale(h, w + 0.8, 0.8, { gain: 0.45, seed: 675 }));
		}
		// the title lands: D major, wide and low, light above it
		const hold = B - titleAt;
		cue(titleAt, (h, w) => impact(h, w, { gain: 0.22, root: 'D1', seed: 4261 }));
		cue(
			titleAt,
			(h, w) =>
				pad(h, w, ['D2', 'A2', 'D3', 'F#3', 'A3', 'E4'], hold + 1.2, {
					attack: 0.15,
					release: 2.0,
					level: 0.6,
					spread: 26,
					seed: 676
				}),
			hold + 2
		);
		cue(titleAt, (h, w) =>
			sub(h, w, 'D2', Math.max(0.4, hold - 0.4), { gain: 0.3, attack: 0.08, release: 1.4 })
		);
		cue(titleAt + 0.3, (h, w) =>
			shimmer(h, w, ['A5', 'D6', 'F#6'], 2.2, { gain: 0.6, attack: 0.6, release: 2.0, seed: 677 })
		);
		(['D3', 'A3', 'F#4'] as const).forEach((n, i) =>
			cue(titleAt + 0.006 * i, (h, w) =>
				piano(h, w, n, {
					vel: 0.6,
					dur: 3.2,
					pan: -0.12 + 0.12 * i,
					verb: 0.7,
					gain: 0.55,
					seed: 680 + i
				})
			)
		);
		cue(titleAt + 1.6, (h, w) =>
			glass(h, w, 'A5', { decay: 2.6, gain: 0.55, pan: 0.2, verb: 0.75, seed: 683 })
		);
		// the end card: the subdominant under the panels, which the tag's D resolves
		cue(
			B - 0.4,
			(h, w) =>
				pad(h, w, ['G2', 'D3', 'G3', 'B3', 'D4'], Math.max(0.4, tagAt - B - 0.2), {
					attack: 1.0,
					release: 1.6,
					level: 0.6,
					seed: 684
				}),
			3
		);
		cue(B - 0.4, (h, w) =>
			air(h, w, 1.6, { peakAt: 0.7, band: [300, 1800], gain: 0.35, pans: [-0.5, 0.5], seed: 685 })
		);
	}

	/**
	 * Grit's house music: a heartbeat on the cuts over a low D minor drone whose filter barely
	 * opens, the word cards struck on low piano octaves with a soft impact, a bowed swell under
	 * the title, and silence a beat before the tag.
	 */
	function gritScore(cue: CueFn, tagAt: number) {
		const stop = tagAt - BEAT;
		cue(
			0,
			(h, w) =>
				padSweep(h, w, [
					[0, 620],
					[titleAt, 1200],
					[stop - 0.1, 700]
				]),
			D
		);
		cue(
			0,
			(h, w) =>
				pad(h, w, ['D2', 'A2', 'D3', 'F3'], Math.max(0.5, stop - 1.2), {
					attack: 1.0,
					level: 1.1,
					cut: true,
					seed: 700
				}),
			stop
		);
		cue(0.3, (h, w) => air(h, w, 2.2, { peakAt: 1.4, band: [200, 1200], gain: 0.5, seed: 701 }));
		const start = seq.find((s) => s.kind === 'cuts')?.at ?? 2.4;
		for (let i = 0; start + (i * BEAT) / 2 < stop - 0.05; i++) {
			const strong = i % 2 === 0;
			cue(start + (i * BEAT) / 2, (h, w) =>
				pulse(h, w, strong ? 'D2' : 'A2', {
					gain: strong ? 1.1 : 0.45,
					open: strong ? 0.25 : 0.1,
					decay: strong ? 0.36 : 0.18,
					seed: 710 + i
				})
			);
		}
		for (const s of seq) {
			if (s.kind === 'card' && s.word) {
				const notes = s.at < titleAt ? ['D1', 'D2', 'A2'] : ['F1', 'F2', 'C3'];
				notes.forEach((n, i) =>
					cue(s.at + 0.004 * i, (h, w) =>
						piano(h, w, n, {
							vel: 0.9,
							dur: 1.1,
							pan: -0.1 + 0.1 * i,
							verb: 0.5,
							gain: 0.6,
							seed: 720 + i + Math.round(s.at)
						})
					)
				);
				cue(s.at, (h, w) => impact(h, w, { gain: 0.3, root: 'D1', seed: 4270 + Math.round(s.at) }));
			} else if (s.kind === 'title') {
				cue(s.at, (h, w) => impact(h, w, { gain: 0.35, root: 'D1', seed: 4280 }));
				cue(s.at, (h, w) =>
					bowed(h, w, 'D3', Math.max(0.4, s.end - s.at - 0.6), {
						gain: 0.7,
						attack: 0.4,
						release: 0.6,
						seed: 730
					})
				);
				cue(s.at, (h, w) =>
					sub(h, w, 'D2', Math.max(0.4, s.end - s.at - 0.8), {
						gain: 0.4,
						attack: 0.05,
						release: 0.5
					})
				);
			} else if (s.kind === 'low') {
				cue(s.at, (h, w) =>
					bowed(h, w, 'A2', Math.max(0.4, stop - s.at - 0.6), {
						gain: 0.8,
						attack: 0.6,
						release: 0.3,
						seed: 731
					})
				);
				cue(s.at, (h, w) =>
					glass(h, w, 'F4', { decay: 2.0, gain: 0.5, pan: -0.2, verb: 0.7, seed: 732 })
				);
			}
		}
		cue(B, (h, w) => impact(h, w, { gain: 0.3, root: 'D1', seed: 4290 }));
	}

	/**
	 * Bright's house music: an open pad, felt piano chords on the red fields' beats, glass
	 * mallets climbing the pentatonic in eighths through the montage over a soft pulse, light
	 * over the title.
	 */
	function brightScore(cue: CueFn, tagAt: number) {
		cue(
			0,
			(h, w) =>
				padSweep(h, w, [
					[0, 1800],
					[titleAt, 3400],
					[D, 2000]
				]),
			D
		);
		const chord = (at: number, dur: number, notes: string[], seed: number) =>
			cue(
				at,
				(h, w) =>
					pad(h, w, notes, dur, { attack: 0.5, release: 1.0, level: 0.7, seed: 740 + seed }),
				dur + 1
			);
		chord(0, 2.4, ['D3', 'A3', 'E4', 'F#4'], 0);
		const montage = seq.find((s) => s.kind === 'montage');
		const slab2 = seq.filter((s) => s.kind === 'slab')[1];
		if (montage) chord(montage.at, montage.end - montage.at - 0.4, ['G2', 'D3', 'A3', 'B3'], 1);
		if (slab2) chord(slab2.at, slab2.end - slab2.at - 0.4, ['B2', 'F#3', 'A3', 'D4'], 2);
		for (const s of seq) {
			if (s.kind !== 'slab') continue;
			const notes = s.at === 0 ? ['D3', 'F#3', 'A3', 'D4'] : ['B2', 'D3', 'F#3', 'A3'];
			for (let k = 0; k < 3; k++)
				notes.forEach((n, i) =>
					cue(s.at + 0.3 + k * BEAT + 0.005 * i, (h, w) =>
						piano(h, w, n, {
							vel: k === 0 ? 0.55 : 0.4,
							dur: 0.7,
							pan: -0.15 + 0.1 * i,
							verb: 0.5,
							gain: 0.75,
							seed: 750 + i + k * 4
						})
					)
				);
		}
		if (montage) {
			const scale = ['D5', 'E5', 'F#5', 'A5', 'B5', 'A5', 'F#5', 'E5'];
			for (let i = 0; montage.at + (i * BEAT) / 2 < montage.end - 0.2; i++) {
				const at = montage.at + (i * BEAT) / 2;
				cue(at, (h, w) =>
					glass(h, w, scale[i % scale.length], {
						decay: 0.9,
						gain: i % 2 ? 0.45 : 0.65,
						pan: i % 2 ? 0.3 : -0.3,
						verb: 0.5,
						seed: 770 + i
					})
				);
				if (i % 2 === 0)
					cue(at, (h, w) =>
						pulse(h, w, 'D2', { gain: 0.5, open: 0.4, decay: 0.25, seed: 790 + i })
					);
			}
		}
		cue(titleAt, (h, w) => impact(h, w, { gain: 0.2, root: 'D1', seed: 4300 }));
		cue(
			titleAt,
			(h, w) =>
				pad(h, w, ['D3', 'A3', 'D4', 'E4', 'F#4'], B - titleAt, {
					attack: 0.2,
					release: 1.6,
					level: 0.6,
					seed: 760
				}),
			B - titleAt + 2
		);
		cue(titleAt + 0.25, (h, w) =>
			shimmer(h, w, ['A5', 'D6', 'E6', 'F#6'], 2.0, {
				gain: 0.7,
				attack: 0.4,
				release: 1.6,
				seed: 761
			})
		);
		['F#5', 'A5', 'D6'].forEach((n, i) =>
			cue(titleAt + 0.25 + (i * BEAT) / 2, (h, w) =>
				glass(h, w, n, { decay: 1.8, gain: 0.45, pan: -0.2 + 0.2 * i, verb: 0.6, seed: 762 + i })
			)
		);
		cue(B - 0.4, (h, w) =>
			air(h, w, 1.4, { peakAt: 0.6, band: [500, 2600], gain: 0.3, pans: [-0.5, 0.5], seed: 765 })
		);
		// the end card: the subdominant under the panels, which the tag's D resolves
		cue(
			B - 0.4,
			(h, w) =>
				pad(h, w, ['G2', 'D3', 'G3', 'B3', 'D4'], Math.max(0.4, tagAt - B - 0.2), {
					attack: 0.8,
					release: 1.4,
					level: 0.55,
					seed: 766
				}),
			3
		);
	}

	return tl;
}
