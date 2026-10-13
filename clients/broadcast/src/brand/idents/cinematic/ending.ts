// The approved ending: the couch alone on black, a bucket of popcorn dropping onto it (gravity, a
// small rebound, the frame giving a little), the two becoming the Couchverse logo, then couchtv
// typed in on the three notes of the sonic logo. Layout as approved: the logo 540 px wide with
// its top at y = 222, couchtv in Plus Jakarta Sans ExtraBold at 184 px, its box top at 650 px.

import { FONT_FAMILY } from '../../../assets';
import { bezierPoints, cubicBezierAt, type Ease } from '../../../core/motion';
import { drawPopcorn } from './couch';
import { BLACK, RED, T, WHITE } from './time';

const ease = (e: Ease, p: number) => cubicBezierAt(bezierPoints(e), Math.max(0, Math.min(1, p)));

/**
 * The popcorn starts its drop a beat before the landing, from rest just above the frame, so the
 * whole fall is seen, accelerating all the way.
 */
export const DROP_FROM = T.land - 0.6;
const HEIGHT = 520;

/** The popcorn's offset and tilt at t (logo placement), or null before it appears. */
export function popcornPose(t: number): { dy: number; rot: number } | null {
	if (t < DROP_FROM) return null;
	if (t < T.land) {
		const u = (t - DROP_FROM) / (T.land - DROP_FROM);
		const p = u * u;
		return { dy: -(1 - p) * HEIGHT, rot: 14 * (1 - p) };
	}
	const q = (t - T.land) / 0.3;
	if (q < 1)
		return { dy: -26 * Math.sin(q * Math.PI) * (1 - q), rot: -2.5 * Math.sin(q * Math.PI) };
	return { dy: 0, rot: 0 };
}

/** The landing presses the couch down a hair. */
export function couchPress(t: number) {
	const q = (t - T.land) / 0.22;
	return q >= 0 && q < 1 ? 7 * Math.sin(q * Math.PI) : 0;
}

/** The frame gives a little on the impact (the approved keyframes, eased between). */
export function frameGive(t: number) {
	const keys: [number, number][] = [
		[0, 0],
		[0.18, 7],
		[0.45, -3],
		[0.7, 1],
		[1, 0]
	];
	const q = (t - T.land) / 0.36;
	if (q <= 0 || q >= 1) return 0;
	for (let i = 1; i < keys.length; i++) {
		const [a, va] = keys[i - 1];
		const [b, vb] = keys[i];
		if (q <= b) {
			const k = (q - a) / (b - a);
			// sine in-out between the keys, so the give has no corners
			return va + (vb - va) * (0.5 - 0.5 * Math.cos(Math.PI * k));
		}
	}
	return 0;
}

/**
 * The popcorn in the couch's own frame (`couch` maps logo units to the screen): it lands where the
 * logo has it, even while the camera is still easing in.
 */
export function popcornMatrix(t: number, couch: DOMMatrix): DOMMatrix | null {
	const pose = popcornPose(t);
	if (!pose) return null;
	// it tilts about a point low in the bucket, as approved
	return couch
		.translate(0, pose.dy / couch.a)
		.multiply(new DOMMatrix().translate(500, 600).rotate(pose.rot).translate(-500, -600));
}

export function drawPopcornAt(ctx: CanvasRenderingContext2D, t: number, couch: DOMMatrix) {
	const m = popcornMatrix(t, couch);
	if (m) drawPopcorn(ctx, m, RED, BLACK);
}

// --- couchtv ---------------------------------------------------------------------------------

const WORD = { size: 184, top: 650, tracking: -0.035 };
const PARTS = [
	{ text: 'couch', color: WHITE, at: 0, rise: 40, scale: 1, dur: 0.34 },
	{ text: 't', color: RED, at: 1, rise: 34, scale: 1.14, dur: 0.3 },
	{ text: 'v', color: RED, at: 2, rise: 34, scale: 1.14, dur: 0.3 }
];

let layout: { x: number[]; w: number[]; base: number; mid: number } | null = null;

function font(ctx: CanvasRenderingContext2D) {
	ctx.font = `800 ${WORD.size}px "${FONT_FAMILY}"`;
	ctx.letterSpacing = `${WORD.tracking * WORD.size}px`;
	ctx.textBaseline = 'alphabetic';
	ctx.textAlign = 'left';
}

function measure(ctx: CanvasRenderingContext2D) {
	font(ctx);
	const w = PARTS.map((p) => ctx.measureText(p.text).width);
	const total = w.reduce((a, b) => a + b, 0);
	const m = ctx.measureText('couchtv');
	// a line box as tall as the font size, the font's own ascent and descent centred in it
	const asc = m.fontBoundingBoxAscent;
	const desc = m.fontBoundingBoxDescent;
	const base = WORD.top + (WORD.size - (asc + desc)) / 2 + asc;
	const x: number[] = [];
	let cx = 960 - total / 2;
	for (const wi of w) {
		x.push(cx);
		cx += wi;
	}
	return { x, w, base, mid: WORD.top + WORD.size / 2 };
}

/** couchtv at t: each part rises in on its note. */
export function drawWord(ctx: CanvasRenderingContext2D, t: number) {
	ctx.save();
	ctx.setTransform(1, 0, 0, 1, 0, 0);
	layout ??= measure(ctx);
	font(ctx);
	PARTS.forEach((p, i) => {
		const at = T.notes[p.at];
		if (t < at) return;
		const k = ease('out', (t - at) / p.dur);
		const fade = ease('out', (t - at) / (p.dur * 0.8));
		const y = (1 - k) * p.rise;
		const s = p.scale + (1 - p.scale) * k;
		const L = layout as NonNullable<typeof layout>;
		const cx = L.x[i] + L.w[i] / 2;
		ctx.save();
		ctx.globalAlpha = fade;
		ctx.translate(cx, L.mid + y);
		ctx.scale(s, s);
		ctx.translate(-cx, -L.mid);
		ctx.fillStyle = p.color;
		ctx.fillText(p.text, L.x[i], L.base);
		ctx.restore();
	});
	ctx.restore();
}

/** How far the ending's moving parts travel over one frame at t, in pixels. */
export function endingMotion(t: number) {
	const dt = 1 / 25;
	let worst = 0;
	const a = popcornPose(t);
	const b = popcornPose(t + dt);
	if (a && b) worst = Math.max(worst, Math.abs(b.dy - a.dy) + Math.abs(b.rot - a.rot) * 6);
	if (!a && b) worst = Math.max(worst, 40);
	worst = Math.max(
		worst,
		Math.abs(couchPress(t + dt) - couchPress(t)),
		Math.abs(frameGive(t + dt) - frameGive(t))
	);
	for (const p of PARTS) {
		const at = T.notes[p.at];
		const y = (x: number) => (x < at ? p.rise : (1 - ease('out', (x - at) / p.dur)) * p.rise);
		const s = (x: number) =>
			x < at ? p.scale : p.scale + (1 - p.scale) * ease('out', (x - at) / p.dur);
		if (t + dt >= at && t < at + p.dur)
			worst = Math.max(worst, Math.abs(y(t + dt) - y(t)) + Math.abs(s(t + dt) - s(t)) * 300);
	}
	return worst;
}
