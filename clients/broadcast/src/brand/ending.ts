// The popcorn ending every ident closes on, as its own piece: the couch alone on black in its
// place in the lockup, a bucket of popcorn dropping onto it (gravity from rest above the frame,
// one small rebound, the frame giving a little and the couch taking the weight), the two the
// Couchverse logo, then couch, t and v typing in on the sonic logo's notes. The motion is the
// approved ending's (concepts' anywhere ident, which the four full idents end on); the short
// ident and the stings place it in time.
//
// The picture is one canvas drawn from t. While anything moves, a frame averages sub-frame
// samples over a half-frame shutter in linear light (a film camera's motion blur), only where
// something moved; a still frame is drawn once, on the same CPU canvas, so nothing shifts as
// the blur starts or stops. Live playback draws one sample.

import { bezierPoints, cubicBezierAt, type Ease } from '../core/motion';
import { edge, windows } from '../core/moves';
import { FPS } from '../core/timeline';
import type { TL } from '../templates/shared';
import { COUCH, LOGO_GAP, LOGO_H, LOGO_W, PIVOT, POPCORN, POPCORN_BOX, SILHOUETTE } from './glyph';
import { BRAND, geometry, PARTS, WORD } from './wordmark';

export const W = 1920;
export const H = 1080;

/** The stacked lockup on the stage, as every ident ends: the logo 540 px wide, its top at 222. */
export const STAGE = (() => {
	const size = 184;
	const g = geometry(size, 'stack');
	const left = 960 - (WORD.advance / 2) * size - g.word.x;
	const top = 222 - g.logo.y;
	return {
		size,
		logo: { x: left + g.logo.x, y: top + g.logo.y, k: g.logo.k },
		word: { x: left + g.word.x, y: top + g.word.y, k: g.word.k },
		box: { x: left, y: top, width: g.width, height: g.height }
	};
})();

/** The popcorn falls this long, from rest 760 px above its place: it shows in its last half. */
export const FALL = 0.72;
const DROP = 760;

const ease = (e: Ease, p: number) => cubicBezierAt(bezierPoints(e), Math.max(0, Math.min(1, p)));

/** The popcorn's offset above its place in the logo (px) and its tilt (degrees), `s` from the landing. */
function popcorn(s: number): { dy: number; rot: number } | null {
	if (s < -FALL) return null;
	if (s < 0) {
		const u = (s + FALL) / FALL;
		const p = u * u;
		return { dy: -(1 - p) * DROP, rot: 14 * (1 - p) };
	}
	const q = s / 0.3;
	if (q < 1)
		return { dy: -26 * Math.sin(q * Math.PI) * (1 - q), rot: -2.5 * Math.sin(q * Math.PI) };
	return { dy: 0, rot: 0 };
}

/** The landing presses the couch down a hair. */
function press(s: number) {
	const q = s / 0.22;
	return q > 0 && q < 1 ? 7 * Math.sin(q * Math.PI) : 0;
}

const GIVE: [number, number][] = [
	[0, 0],
	[0.18, 7],
	[0.45, -3],
	[0.7, 1],
	[1, 0]
];

/** The frame gives on the impact: the approved keyframes, eased between so it has no corners. */
function give(s: number) {
	const q = s / 0.36;
	if (q <= 0 || q >= 1) return 0;
	for (let i = 1; i < GIVE.length; i++) {
		const [a, va] = GIVE[i - 1];
		const [b, vb] = GIVE[i];
		if (q <= b) return va + (vb - va) * (0.5 - 0.5 * Math.cos(Math.PI * ((q - a) / (b - a))));
	}
	return 0;
}

/** couch rises 40 px into place; t and v rise 34 px and settle from 1.2 times their size. */
const TYPE = [
	{ rise: 40, scale: 1, dur: 0.34, ease: 'out' },
	{ rise: 34, scale: 1.2, dur: 0.26, ease: 'snap' },
	{ rise: 34, scale: 1.2, dur: 0.26, ease: 'snap' }
] as const;

export interface EndingTimes {
	/** When the popcorn lands; before it falls the couch is alone. */
	land: number;
	/** The couch alone fading in over this span (else it is there from the start). */
	couchIn?: [number, number];
	/** couch, t and v. */
	notes: readonly [number, number, number];
}

interface Part {
	alpha: number;
	dy: number;
	scale: number;
}

/** Everything that moves, at t. */
export interface State {
	give: number;
	couch: { alpha: number; dy: number };
	pop: { dy: number; rot: number } | null;
	word: Part[];
}

export function stateAt(times: EndingTimes, t: number): State {
	const s = t - times.land;
	let couchAlpha = 1;
	if (times.couchIn) {
		const [a, b] = times.couchIn;
		couchAlpha = ease('sine', (t - a) / (b - a));
	}
	return {
		give: give(s),
		couch: { alpha: couchAlpha, dy: press(s) },
		pop: popcorn(s),
		word: TYPE.map((p, i) => {
			const start = edge(times.notes[i]);
			if (t < start) return { alpha: 0, dy: 0, scale: 1 };
			const k = ease(p.ease, (t - start) / p.dur);
			return { alpha: k, dy: (1 - k) * p.rise, scale: p.scale + (1 - p.scale) * k };
		})
	};
}

/** How far the fastest point moves between two states, px: how many samples a frame needs. */
function motion(a: State, b: State): number {
	const d = (x: number, y: number) => Math.abs(x - y);
	let m = d(a.give, b.give) + d(a.couch.dy, b.couch.dy) + d(a.couch.alpha, b.couch.alpha) * 60;
	if (a.pop && b.pop) m = Math.max(m, d(a.pop.dy, b.pop.dy) + d(a.pop.rot, b.pop.rot) * 6);
	else if (a.pop || b.pop) m = Math.max(m, 40);
	a.word.forEach((w, i) => {
		const v = b.word[i];
		m = Math.max(m, d(w.dy, v.dy) + d(w.scale, v.scale) * 120 + d(w.alpha, v.alpha) * 60);
	});
	return m;
}

const SHUTTER = 0.5 / FPS;

// sRGB to linear light for 8-bit values, and back through a fine table
const LIN = Float32Array.from({ length: 256 }, (_, i) => {
	const v = i / 255;
	return v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
});
const ENC_N = 65535;
let enc: Uint8Array | null = null;
function encoder() {
	if (!enc) {
		enc = new Uint8Array(ENC_N + 1);
		for (let i = 0; i <= ENC_N; i++) {
			const x = i / ENC_N;
			const s = x <= 0.0031308 ? 12.92 * x : 1.055 * x ** (1 / 2.4) - 0.055;
			enc[i] = Math.round(Math.max(0, Math.min(1, s)) * 255);
		}
	}
	return enc;
}

interface Rect {
	x0: number;
	y0: number;
	x1: number;
	y1: number;
}

/** The popcorn's box in logo units, its knockout included. */
const POP_BOX = [
	POPCORN_BOX[0] - LOGO_GAP,
	POPCORN_BOX[1] - LOGO_GAP,
	POPCORN_BOX[2] + LOGO_GAP,
	POPCORN_BOX[3] + LOGO_GAP
];

/** The ending's picture: a canvas the timeline draws per frame. */
export class Ending {
	readonly canvas: HTMLCanvasElement;
	private readonly ctx: CanvasRenderingContext2D;
	private work: CanvasRenderingContext2D | null = null;
	private readonly couch = new Path2D(COUCH);
	private readonly pop = new Path2D(POPCORN);
	private readonly halo = new Path2D(SILHOUETTE);
	private readonly parts = PARTS.map((p) => new Path2D(p.d));

	constructor(
		parent: Element,
		readonly times: EndingTimes
	) {
		this.canvas = document.createElement('canvas');
		this.canvas.width = W;
		this.canvas.height = H;
		this.canvas.className = 'ending';
		parent.appendChild(this.canvas);
		// a CPU canvas: the averaged frames are read back from one, and every frame has to
		// rasterize alike, or edges would shift a hair as the blur starts or stops
		this.ctx = this.canvas.getContext('2d', { alpha: false, willReadFrequently: true })!;
	}

	/**
	 * The frame at t as a value: its time while anything moves, else its state, which every
	 * frame of a still stretch shares (so the renderer reuses the frame).
	 */
	key(t: number): string {
		const a = stateAt(this.times, t - SHUTTER / 2);
		const b = stateAt(this.times, t + SHUTTER / 2);
		return motion(a, b) > 0.01 ? `t${t.toFixed(4)}` : JSON.stringify(stateAt(this.times, t));
	}

	/** Show a frame `key` named. */
	show(key: string, hq: boolean) {
		if (key.startsWith('t')) this.render(Number(key.slice(1)), hq);
		else this.draw(this.ctx, JSON.parse(key) as State);
	}

	/** Draw the frame at t: `hq` averages the shutter's samples (the render), else one sample. */
	render(t: number, hq: boolean) {
		const a = stateAt(this.times, t - SHUTTER / 2);
		const b = stateAt(this.times, t + SHUTTER / 2);
		const n = hq ? Math.max(1, Math.min(32, Math.ceil(motion(a, b) / 1.5))) : 1;
		this.draw(this.ctx, stateAt(this.times, t));
		if (n === 1) return;
		const states = Array.from({ length: n }, (_, i) =>
			stateAt(this.times, t + ((i + 0.5) / n - 0.5) * SHUTTER)
		);
		const r = this.dirty(states);
		if (!r) return;
		const w = r.x1 - r.x0;
		const h = r.y1 - r.y0;
		if (!this.work) {
			const c = document.createElement('canvas');
			c.width = W;
			c.height = H;
			this.work = c.getContext('2d', { alpha: false, willReadFrequently: true })!;
		}
		const acc = new Float32Array(w * h * 3);
		for (const s of states) {
			this.draw(this.work, s, r);
			const px = this.work.getImageData(r.x0, r.y0, w, h).data;
			for (let p = 0, q = 0; p < px.length; p += 4, q += 3) {
				acc[q] += LIN[px[p]];
				acc[q + 1] += LIN[px[p + 1]];
				acc[q + 2] += LIN[px[p + 2]];
			}
		}
		const e = encoder();
		const out = this.ctx.createImageData(w, h);
		const k = ENC_N / n;
		for (let p = 0, q = 0; p < out.data.length; p += 4, q += 3) {
			out.data[p] = e[Math.round(acc[q] * k)];
			out.data[p + 1] = e[Math.round(acc[q + 1] * k)];
			out.data[p + 2] = e[Math.round(acc[q + 2] * k)];
			out.data[p + 3] = 255;
		}
		this.ctx.putImageData(out, r.x0, r.y0);
	}

	/** Where the samples differ: every moving thing's box, through all of them. */
	private dirty(states: State[]): Rect | null {
		const r: Rect = { x0: W, y0: H, x1: 0, y1: 0 };
		const add = (m: DOMMatrix, box: number[]) => {
			for (const [x, y] of [
				[box[0], box[1]],
				[box[2], box[1]],
				[box[0], box[3]],
				[box[2], box[3]]
			]) {
				const p = m.transformPoint(new DOMPoint(x, y));
				r.x0 = Math.min(r.x0, p.x);
				r.y0 = Math.min(r.y0, p.y);
				r.x1 = Math.max(r.x1, p.x);
				r.y1 = Math.max(r.y1, p.y);
			}
		};
		for (const s of states) {
			add(this.couchMatrix(s), [0, 0, LOGO_W, LOGO_H]);
			if (s.pop) add(this.popMatrix(s, s.pop), POP_BOX);
			PARTS.forEach((p, i) => {
				if (s.word[i].alpha > 0) add(this.partMatrix(s, i), [0, -940, p.advance + 40, 40]);
			});
		}
		// a margin for antialiasing; nothing outside the frame
		const x0 = Math.max(0, Math.floor(r.x0) - 2);
		const y0 = Math.max(0, Math.floor(r.y0) - 2);
		const x1 = Math.min(W, Math.ceil(r.x1) + 2);
		const y1 = Math.min(H, Math.ceil(r.y1) + 2);
		return x1 > x0 && y1 > y0 ? { x0, y0, x1, y1 } : null;
	}

	private couchMatrix(s: State) {
		const l = STAGE.logo;
		return new DOMMatrix().translate(l.x, l.y + s.give + s.couch.dy).scale(l.k);
	}

	private popMatrix(s: State, pop: { dy: number; rot: number }) {
		const l = STAGE.logo;
		return new DOMMatrix()
			.translate(l.x, l.y + s.give + pop.dy)
			.scale(l.k)
			.translate(PIVOT.x, PIVOT.y)
			.rotate(pop.rot)
			.translate(-PIVOT.x, -PIVOT.y);
	}

	/** A part of the word, scaled about the middle of its line box as the idents' spans are. */
	private partMatrix(s: State, i: number) {
		const w = STAGE.word;
		const p = PARTS[i];
		const part = s.word[i];
		const cx = w.x + (p.x + p.advance / 2) * w.k;
		const cy = w.y - (WORD.baseline - 0.5) * STAGE.size;
		return new DOMMatrix()
			.translate(0, s.give + part.dy)
			.translate(cx, cy)
			.scale(part.scale)
			.translate(-cx, -cy)
			.translate(w.x + p.x * w.k, w.y)
			.scale(w.k);
	}

	/** One sample of the picture, within `clip` when given. */
	private draw(ctx: CanvasRenderingContext2D, s: State, clip?: Rect) {
		ctx.save();
		ctx.setTransform(1, 0, 0, 1, 0, 0);
		ctx.globalAlpha = 1;
		if (clip) {
			ctx.beginPath();
			ctx.rect(clip.x0, clip.y0, clip.x1 - clip.x0, clip.y1 - clip.y0);
			ctx.clip();
		}
		ctx.fillStyle = BRAND.night;
		ctx.fillRect(0, 0, W, H);
		if (s.couch.alpha > 0) {
			ctx.globalAlpha = s.couch.alpha;
			ctx.setTransform(this.couchMatrix(s));
			ctx.fillStyle = BRAND.red;
			ctx.fill(this.couch);
		}
		if (s.pop) {
			ctx.globalAlpha = 1;
			ctx.setTransform(this.popMatrix(s, s.pop));
			// the knockout: the field's colour around the bucket cuts the couch where they meet
			ctx.fillStyle = BRAND.night;
			ctx.strokeStyle = BRAND.night;
			ctx.lineWidth = LOGO_GAP * 2;
			ctx.lineJoin = 'round';
			ctx.fill(this.halo);
			ctx.stroke(this.halo);
			ctx.fillStyle = BRAND.red;
			ctx.fill(this.pop);
		}
		s.word.forEach((part, i) => {
			if (part.alpha <= 0) return;
			ctx.globalAlpha = Math.min(1, part.alpha);
			ctx.setTransform(this.partMatrix(s, i));
			ctx.fillStyle = i ? BRAND.red : BRAND.white;
			ctx.fill(this.parts[i]);
		});
		ctx.restore();
	}
}

/** The ending on a timeline, shown from `from`: drawn per frame, once per still stretch. */
export function ending(tl: TL, parent: Element, times: EndingTimes, from = 0): Ending {
	const e = new Ending(parent, times);
	tl.dynamic(
		(t) => e.key(t),
		(v, wait) => e.show(String(v), wait !== null)
	);
	if (from > 0) windows(tl, e.canvas, [[from, tl.duration + 1]]);
	return e;
}
