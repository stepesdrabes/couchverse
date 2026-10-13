// The picture of the morph ident: one canvas redrawn per frame as a pure function of t.
//
// Four couch posters in flat Swiss-poster geometry (night, sunset, space, the stage), joined
// by morphs instead of cuts, every element becoming the next scene's: the full moon swells
// into the sky's light and the red sun kindles inside it; the light then contracts around the
// sun into a ring as the sun cools into a planet; the planet comes down under the couch as the
// spotlight's pool and its ring rises to be the lamp. Then the light goes down, the couch rises
// into the lockup, the popcorn drops onto it and the two are the Couchverse logo; couchtv
// types itself in under it.
//
// Rendering: the field (the sky, the light, the beam) is computed per pixel in floats, averaged
// over the shutter's sub-frame states and dithered, so its gradients never band; shapes go on
// top as canvas paths; knockouts (the logo's gaps) are painted with the field itself, so a gap
// always shows what lies behind. Shapes are motion blurred by averaging sub-frame samples over
// a half-frame shutter, as a film camera would see them.

import { FONT_FAMILY } from '../../../assets';
import { rng } from '../../../core/rng';
import { COUCH, GAP, LOGO_W, POPCORN } from '../../glyph';
import { clamp01, color, CURVE, ease, hexLab, labRgb, num, span, type Key, type Lab } from './anim';

export const W = 1920;
export const H = 1080;

export const RED = '#e50914';
export const DEEP = '#a3060e';
const EARTH = '#3a0409';
export const BLACK = '#050505';
const PAPER = '#f4ede4';
const PAPER_LOW = '#f2e3d7';
const WHITE = '#ffffff';

/** The storyboard, in seconds (75 bpm: a beat is 0.8 s, 20 frames). */
export const T = {
	/** Night to sunset: the moon fills, swells into the light, the sun kindles inside it. */
	m1: [1.0, 2.2],
	/** Sunset to space: the light contracts into the ring, the ground falls away. */
	m2: [2.6, 3.8],
	/** Space to the stage: the planet comes down as the pool, the ring rises as the lamp. */
	m3: [4.2, 5.25],
	/** The couch rises into the lockup while the stage light goes down. */
	rise: [5.6, 6.24],
	/** The popcorn falls (it enters the frame at the top) and lands. */
	drop: 5.72,
	land: 6.4,
	/** couch, t, v: the sonic logo's notes. */
	notes: [7.2, 7.6, 8.0],
	/** Nothing moves after this. */
	still: 8.9
} as const;

/** The lockup: the logo centred, LOCK.w wide, its top at LOCK.y; couchtv under it. */
const LOCK = { w: 540, y: 222, word: 184, wordY: 650 };
const KL = LOCK.w / LOGO_W;
/** The couch's feet (the logo's bottom) in the lockup. */
const LOCK_FEET = LOCK.y + 745 * KL;

const HORIZON = 800;
const STAGE_Y = 830;
const LAMP_Y = 54;

const [m1a, m1b] = T.m1;
const [m2a, m2b] = T.m2;
const [m3a, m3b] = T.m3;
const [ra, rb] = T.rise;

/** Halfway between frames: no frame's shutter straddles a switch placed there. */
const between = (t: number) => (Math.round(t * 25) + 0.5) / 25;
/** The moon's last moment as a shape: from here on it is the light, in the field. */
const FULL = between(m1a + 0.36);
/**
 * From here the ring's near half is drawn over the planet. The light is always in the field
 * (behind everything); this overlay starts while the ring's hole still holds the whole planet,
 * so it starts by drawing nothing.
 */
const OVER = between(m2a + 0.86);

// --- the storyboard as tracks ------------------------------------------------------------------

/** One path for the moon, the sun and the planet: the light and the disc share it. */
const PATH_X: Key<number>[] = [
	[0, 1470, CURVE.sine],
	[m1a, 1446, CURVE.glide],
	[m1b, 960, CURVE.sine],
	[m2a, 960, CURVE.glide],
	[m2b, 520, CURVE.sine],
	[m3a, 532, CURVE.glide]
];
const PATH_Y: Key<number>[] = [
	[0, 300, CURVE.sine],
	[m1a, 268, [0.55, 0, 0.3, 1]],
	[m1b, 880, CURVE.sine],
	[m2a, 892, CURVE.glide],
	[m2b, 330, CURVE.sine],
	[m3a, 338, CURVE.glide]
];

/** The light's radius that covers the whole frame from the sun. */
const COVER = 1520;

/** The light: the moon, the flooded sky, the ring, the lamp (an elliptical annulus). */
const light = {
	x: num([...PATH_X, [m3b, 960]]),
	y: num([...PATH_Y, [m3b, LAMP_Y]]),
	ro: num([
		[0, 104],
		[m1a + 0.15, 104, [0.5, 0, 0.45, 1]],
		[m1b + 0.15, COVER],
		[m2a, COVER, [0.45, 0, 0.3, 1]],
		[m2a + 0.7, 640, [0.3, 0, 0.45, 1]],
		[m2b + 0.1, 446],
		[m3a, 448, CURVE.glide],
		[m3b, 150]
	]),
	/** The lamp fills in once it has risen and shrunk (1: the ring's band, 0: a disc). */
	fill: num([
		[m3b - 0.42, 1, CURVE.inOut],
		[m3b + 0.02, 0]
	]),
	// the ring keeps settling through the hold, one move into the next
	sy: num([
		[m2a + 0.7, 1, [0.45, 0, 0.2, 1]],
		[m3a, 0.18, CURVE.glide],
		[m3b, 0.21]
	]),
	rot: num([
		[m2a + 0.7, 0, [0.5, 0, 0.2, 1]],
		[m3a, -16, CURVE.glide],
		[m3b, 0]
	]),
	alpha: num([
		[ra, 1, CURVE.sine],
		[rb - 0.1, 0]
	])
};
const BAND = 30;

/**
 * The light's hole. It grows behind the sun and comes to rest at the sun's rim, then opens
 * out from there into the ring's band: it leaves the rim at no speed, so its edge emerges
 * crisp instead of as a blurred shadow.
 */
const HOLE = [m2a + 0.1, m2a + 0.45, m2b] as const;
function inner(t: number): number {
	if (t < HOLE[0]) return 0;
	const r = disc.r(t);
	if (t < HOLE[1]) return r * ease([0.4, 0, 0.35, 1], span(t, HOLE[0], HOLE[1]));
	const band = Math.max(0, light.ro(t) - BAND);
	if (t < HOLE[2]) return r + ease([0.45, 0, 0.3, 1], span(t, HOLE[1], HOLE[2])) * (band - r);
	return band * light.fill(t);
}

/** The moon's dark side (where the moon is not painted): slides off until the moon is full. */
const shade = {
	dx: num([
		[m1a - 0.1, -0.42, CURVE.in],
		[FULL, -2.2]
	]),
	dy: num([
		[m1a - 0.1, -0.3, CURVE.in],
		[FULL, -1.6]
	])
};

/** The disc: the sun, the planet, the pool. Kindles inside the moon's light. */
const disc = {
	x: num([...PATH_X, [m3b - 0.05, 960]]),
	y: num([...PATH_Y, [m3b - 0.05, STAGE_Y]]),
	r: num([
		[FULL - 0.06, 0, [0.4, 0, 0.25, 1]],
		[m1b + 0.05, 480, CURVE.sine],
		[m2a, 484, CURVE.glide],
		[m2b, 205, CURVE.sine],
		[m3a, 208, CURVE.glide],
		[m3b - 0.05, 440, CURVE.sine],
		[ra, 446, CURVE.inOut],
		[rb, 300]
	]),
	sy: num([
		[m3a + 0.05, 1, CURVE.glide],
		[m3b - 0.05, 0.13]
	]),
	/** The spotlight coming on: the pool fills with light from its centre. */
	lit: num([
		[m3a + 0.45, 0, CURVE.glide],
		[m3b + 0.05, 1]
	]),
	alpha: num([
		[ra, 1, CURVE.sine],
		[rb - 0.1, 0]
	])
};

/** The planet's night side, in a deeper red: slides in as the sun cools, out as it lands. */
const night = {
	d: num([
		[m2a + 0.35, 2.2, CURVE.glide],
		[m2b + 0.1, 1.25, CURVE.sine],
		[m3a, 1.22, CURVE.in],
		[m3a + 0.45, 2.2]
	])
};

/** The ground: a gentle hill at night, flat at sunset, then it falls away. */
const ground = {
	y: num([
		[0, HORIZON],
		[m2a + 0.1, HORIZON, CURVE.in],
		[m2a + 0.8, 1320]
	]),
	bend: num([
		[m1a + 0.1, 1 / 3600, CURVE.glide],
		[m1b, 0]
	]),
	col: color([
		[m1a + 0.4, EARTH, CURVE.sine],
		[m1b - 0.15, BLACK]
	])
};

/** The couch: centre x, feet y, width, rotation (degrees). */
const couch = {
	x: num([
		[m2a + 0.15, 960, CURVE.glide],
		[m2b + 0.15, 1250, CURVE.sine],
		[m3a + 0.05, 1262, CURVE.glide],
		[m3b, 960]
	]),
	feet: num([
		[m2a + 0.05, HORIZON, CURVE.glide],
		[m2b + 0.1, 720, CURVE.sine],
		[m3a + 0.05, 708, CURVE.glide],
		[m3b, STAGE_Y, CURVE.sine],
		[ra, STAGE_Y, CURVE.inOut],
		[rb, LOCK_FEET]
	]),
	w: num([
		[m2a + 0.1, 500, CURVE.glide],
		[m2b + 0.1, 450, CURVE.sine],
		[m3a + 0.05, 450, CURVE.glide],
		[m3b, 500, CURVE.sine],
		[ra, 500, CURVE.inOut],
		[rb, LOCK.w]
	]),
	rot: num([
		[m2a + 0.15, 0, CURVE.glide],
		[m2b + 0.15, -9, CURVE.sine],
		[m3a + 0.05, -11, CURVE.glide],
		[m3b, 0]
	]),
	col: color([
		[m1a + 0.55, RED, CURVE.sine],
		[m1b - 0.05, BLACK],
		[m2a + 0.3, BLACK, CURVE.sine],
		[m2b - 0.2, RED]
	])
};

/** The sky under the light: a vertical gradient (top, bottom). */
const sky = {
	top: color([
		[0, '#030303'],
		[m1b, '#030303', CURVE.sine],
		[m2a, BLACK]
	]),
	low: color([
		[0, '#160b0c'],
		[m1b, '#160b0c', CURVE.sine],
		[m2a, BLACK]
	])
};

const beam = {
	alpha: num([
		[m3b - 0.45, 0, CURVE.sine],
		[m3b + 0.1, 1, CURVE.sine],
		[ra, 1, CURVE.sine],
		[rb - 0.1, 0]
	])
};

const stars = num([
	[m3a, 1, CURVE.sine],
	[m3a + 0.6, 0]
]);

/** The camera: a slow push and drift for the posters, home for the lockup. */
const camera = {
	z: num([
		[0, 1.0, CURVE.sine],
		[ra - 0.2, 1.045, CURVE.inOut],
		[rb, 1]
	]),
	x: num([
		[0, 26, CURVE.sine],
		[ra - 0.2, -22, CURVE.inOut],
		[rb, 0]
	]),
	y: num([
		[0, 64, CURVE.expo],
		[1.4, 6, CURVE.sine],
		[ra - 0.2, 0]
	])
};

/** How far the light and the disc are: with the sky, then on the stage. */
const depth = num([
	[m3a, 0.6, CURVE.inOut],
	[m3b, 1]
]);

const fadeIn = num([
	[0, 1, CURVE.sine],
	[0.7, 0]
]);

// --- the landing ---------------------------------------------------------------------------

const DROP_H = 520;
const G = (2 * DROP_H) / (T.land - T.drop) ** 2;
const V_LAND = G * (T.land - T.drop);
/** One small rebound: the popcorn keeps a little of its speed. */
const V_UP = V_LAND * 0.2;
const BOUNCE = (2 * V_UP) / G;

/** The popcorn's offset above its place in the logo (px) and its tilt (degrees). */
function popcorn(t: number): { dy: number; rot: number } | null {
	if (t < T.drop - 0.02) return null;
	if (t < T.land) {
		const s = Math.max(0, t - T.drop);
		const p = s / (T.land - T.drop);
		return { dy: -DROP_H + 0.5 * G * s * s, rot: 9 * (1 - ease(CURVE.sine, p)) };
	}
	const s = t - T.land;
	if (s < BOUNCE) {
		const dy = -(V_UP * s - 0.5 * G * s * s);
		return { dy, rot: -1.6 * Math.sin((s / BOUNCE) * Math.PI) };
	}
	return { dy: 0, rot: 0 };
}

/** The frame gives a little on the landing: a damped spring, down first. */
function give(t: number): number {
	const s = t - T.land;
	if (s < 0 || s > 0.8) return 0;
	return 8 * Math.exp(-s * 9) * Math.sin(2 * Math.PI * 3.1 * s);
}

/** The couch takes the landing: a few pixels down and back. */
function press(t: number): number {
	const s = t - T.land;
	if (s < 0 || s > 0.5) return 0;
	return 4 * Math.exp(-s * 11) * Math.sin(Math.PI * Math.min(1, s / 0.16));
}

/** couchtv: each part rises into place on its note. */
function word(t: number, i: number): { a: number; dy: number } {
	const s = t - T.notes[i];
	if (s < -0.04) return { a: 0, dy: 0 };
	return {
		a: ease(CURVE.out, clamp01((s + 0.04) / 0.2)),
		dy: 44 * (1 - ease(CURVE.expo, (s + 0.04) / 0.62))
	};
}

// --- the state at t --------------------------------------------------------------------------

interface Cam {
	z: number;
	x: number;
	y: number;
	shake: number;
}

/** An elliptical annulus, centre (x, y), radii ro > ri along its axis, sy the minor scale. */
export interface Ring {
	x: number;
	y: number;
	ro: number;
	ri: number;
	sy: number;
	/** Degrees. */
	rot: number;
}

interface State {
	t: number;
	cam: Cam;
	fade: number;
	sky: [Lab, Lab];
	light: Ring & { alpha: number };
	/** The moon as a shape (before it is the light), with its dark side's offset. */
	moon: { dx: number; dy: number } | null;
	disc: { x: number; y: number; r: number; sy: number; lit: number; alpha: number; night: number };
	ground: { y: number; bend: number; col: Lab };
	couch: { x: number; feet: number; w: number; rot: number; col: Lab };
	pop: { dy: number; rot: number } | null;
	beam: number;
	stars: number;
	/** The light's and the disc's depth for the parallax. */
	depth: number;
	word: { a: number; dy: number }[];
}

export function stateAt(t: number): State {
	return {
		t,
		cam: { z: camera.z(t), x: camera.x(t), y: camera.y(t), shake: give(t) },
		fade: fadeIn(t),
		sky: [sky.top(t), sky.low(t)],
		light: {
			x: light.x(t),
			y: light.y(t),
			ro: light.ro(t),
			ri: inner(t),
			sy: light.sy(t),
			rot: light.rot(t),
			alpha: light.alpha(t)
		},
		moon: t < FULL ? { dx: shade.dx(t), dy: shade.dy(t) } : null,
		disc: {
			x: disc.x(t),
			y: disc.y(t),
			r: disc.r(t),
			sy: disc.sy(t),
			lit: disc.lit(t),
			alpha: disc.alpha(t),
			night: night.d(t)
		},
		ground: { y: ground.y(t), bend: ground.bend(t), col: ground.col(t) },
		couch: {
			x: couch.x(t),
			feet: couch.feet(t) + press(t),
			w: couch.w(t),
			rot: couch.rot(t),
			col: couch.col(t)
		},
		pop: popcorn(t),
		beam: beam.alpha(t),
		stars: stars(t),
		depth: depth(t),
		word: [0, 1, 2].map((i) => word(t, i))
	};
}

/** How far (px) the shapes' fastest point moves between two states: the sample count. */
function shapeMotion(a: State, b: State): number {
	const d = (x: number, y: number) => Math.abs(x - y);
	const L = (s: State) => s.light;
	const ringMoves =
		a.t >= OVER || b.t >= OVER
			? d(L(a).x, L(b).x) +
				d(L(a).y, L(b).y) +
				d(L(a).ro, L(b).ro) +
				d(L(a).ri, L(b).ri) +
				d(L(a).sy, L(b).sy) * 400 +
				d(L(a).rot, L(b).rot) * 8
			: 0;
	const ds = [
		ringMoves,
		a.moon && b.moon
			? (d(a.moon.dx, b.moon.dx) + d(a.moon.dy, b.moon.dy)) * 92 +
				d(L(a).x, L(b).x) +
				d(L(a).y, L(b).y)
			: 0,
		d(a.disc.x, b.disc.x) +
			d(a.disc.y, b.disc.y) +
			d(a.disc.r, b.disc.r) +
			d(a.disc.sy, b.disc.sy) * a.disc.r,
		d(a.disc.lit, b.disc.lit) * a.disc.r,
		d(a.disc.night, b.disc.night) * a.disc.r,
		d(a.ground.y, b.ground.y) + d(a.ground.bend, b.ground.bend) * 120000,
		d(a.couch.x, b.couch.x) + d(a.couch.feet, b.couch.feet) + d(a.couch.w, b.couch.w),
		d(a.couch.rot, b.couch.rot) * 5,
		a.pop && b.pop ? d(a.pop.dy, b.pop.dy) + d(a.pop.rot, b.pop.rot) * 4 : 0,
		d(a.cam.z, b.cam.z) * 1000 +
			d(a.cam.x, b.cam.x) +
			d(a.cam.y, b.cam.y) +
			d(a.cam.shake, b.cam.shake),
		...a.word.map((w, i) => d(w.dy, b.word[i].dy))
	];
	return Math.max(...ds);
}

/** How far the field's edges move: the light's rim and the beam. */
function fieldMotion(a: State, b: State): number {
	const d = (x: number, y: number) => Math.abs(x - y);
	const A = a.light;
	const B = b.light;
	const move = d(A.x, B.x) + d(A.y, B.y) + d(a.depth, b.depth) * 200 + d(a.cam.z, b.cam.z) * 1000;
	return Math.max(
		a.beam > 0 || b.beam > 0 ? 4 + d(a.disc.r, b.disc.r) + d(a.disc.y, b.disc.y) : 0,
		move + d(A.ro, B.ro) + d(A.sy, B.sy) * A.ro + d(A.rot, B.rot) * A.ro * 0.0175,
		move + d(A.ri, B.ri) + d(A.sy, B.sy) * A.ri + d(A.rot, B.rot) * A.ri * 0.0175
	);
}

// --- drawing -----------------------------------------------------------------------------------

/** Layer depths for the parallax: the sky is far, the couch is the camera's subject. */
const DEPTH = { stars: 0.25, near: 1 };

/** A layer's screen transform: the camera's zoom and pan scaled by the layer's depth. */
function layer(cam: Cam, k: number): DOMMatrix {
	const z = 1 + (cam.z - 1) * k;
	return new DOMMatrix()
		.translate(960 + cam.x * k, 540 + cam.y * k + cam.shake)
		.scale(z)
		.translate(-960, -540);
}

/** The light as the field draws it: in screen space, rotation in radians. */
interface ScreenRing {
	x: number;
	y: number;
	ro: number;
	ri: number;
	sy: number;
	c: number;
	s: number;
	a: number;
}

function screenRing(s: State): ScreenRing | null {
	const L = s.light;
	if (s.moon || L.ro <= 0 || L.alpha <= 0) return null;
	const m = layer(s.cam, s.depth);
	const p = m.transformPoint(new DOMPoint(L.x, L.y));
	const rad = (L.rot * Math.PI) / 180;
	return {
		x: p.x,
		y: p.y,
		ro: L.ro * m.a,
		ri: L.ri * m.a,
		sy: Math.max(0.02, L.sy),
		c: Math.cos(rad),
		s: Math.sin(rad),
		a: L.alpha
	};
}

interface ScreenBeam {
	/** Top centre and half width, bottom centre and half width. */
	tx: number;
	ty: number;
	tw: number;
	bx: number;
	by: number;
	bw: number;
	a: number;
}

function screenBeam(s: State): ScreenBeam | null {
	if (s.beam <= 0) return null;
	const far = layer(s.cam, s.depth);
	const near = far;
	const top = far.transformPoint(new DOMPoint(s.light.x, s.light.y));
	const bot = near.transformPoint(new DOMPoint(s.disc.x, s.disc.y));
	return {
		tx: top.x,
		ty: top.y,
		tw: s.light.ro * far.a,
		bx: bot.x,
		by: bot.y,
		bw: s.disc.r * near.a,
		a: s.beam * Math.min(s.light.alpha, s.disc.alpha)
	};
}

/** The chord of an ellipse (radius r along its axis) on row y: [x0, x1] or null. */
function chord(e: ScreenRing, r: number, y: number): [number, number] | null {
	if (r <= 0) return null;
	const q = e.sy * e.sy;
	const dy = y - e.y;
	const A = e.c * e.c + (e.s * e.s) / q;
	const B = 2 * dy * e.c * e.s * (1 - 1 / q);
	const C = dy * dy * (e.s * e.s + (e.c * e.c) / q) - r * r;
	const disc = B * B - 4 * A * C;
	if (disc <= 0) return null;
	const sq = Math.sqrt(disc);
	return [e.x + (-B - sq) / (2 * A), e.x + (-B + sq) / (2 * A)];
}

/** Which x on row y are surely inside (in), or maybe on the edge (edge), of an ellipse. */
function zones(
	e: ScreenRing,
	r: number,
	y: number
): { in0: number; in1: number; e0: number; e1: number } | null {
	const rows = [chord(e, r, y - 1.5), chord(e, r, y + 0.5), chord(e, r, y + 2.5)];
	const hit = rows.filter((c): c is [number, number] => !!c);
	if (!hit.length) return null;
	const all = hit.length === rows.length;
	return {
		in0: all ? Math.max(...hit.map((c) => c[0])) + 2 : Infinity,
		in1: all ? Math.min(...hit.map((c) => c[1])) - 2 : -Infinity,
		e0: Math.min(...hit.map((c) => c[0])) - 2,
		e1: Math.max(...hit.map((c) => c[1])) + 2
	};
}

/** Exact-enough coverage of pixel centre (px, py) by an ellipse: its signed distance, AA'd. */
function cover(e: ScreenRing, r: number, px: number, py: number): number {
	const dx = px - e.x;
	const dy = py - e.y;
	const u = dx * e.c + dy * e.s;
	const v = (-dx * e.s + dy * e.c) / e.sy;
	const g = Math.sqrt(u * u + v * v);
	if (g < 1e-6) return 1;
	const gx = (u * e.c - (v * e.s) / e.sy) / g;
	const gy = (u * e.s + (v * e.c) / e.sy) / g;
	const sd = (g - r) / Math.max(1e-6, Math.sqrt(gx * gx + gy * gy));
	return clamp01(0.5 - sd);
}

const SHUTTER = 0.5 / 25;

interface StarDot {
	x: number;
	y: number;
	r: number;
	a: number;
}

export class Picture {
	readonly canvas: HTMLCanvasElement;
	private readonly ctx: CanvasRenderingContext2D;
	private readonly field: HTMLCanvasElement;
	private readonly fctx: CanvasRenderingContext2D;
	private readonly img: ImageData;
	private readonly acc: Float32Array;
	private readonly dither: Float32Array;
	private readonly rowL = new Float32Array(W);
	private readonly sky: CanvasRenderingContext2D;
	private readonly rowB = new Float32Array(W);
	private readonly couchPath = new Path2D(COUCH);
	private readonly popPath = new Path2D(POPCORN);
	private readonly stars: StarDot[] = [];
	private wordLayout: { x: number[]; base: number } | null = null;

	constructor(parent: Element) {
		this.canvas = document.createElement('canvas');
		this.canvas.width = W;
		this.canvas.height = H;
		Object.assign(this.canvas.style, {
			position: 'absolute',
			left: '0',
			top: '0',
			width: `${W}px`,
			height: `${H}px`
		});
		parent.appendChild(this.canvas);
		this.ctx = this.canvas.getContext('2d', { willReadFrequently: true, alpha: false })!;
		this.field = document.createElement('canvas');
		this.field.width = W;
		this.field.height = H;
		this.fctx = this.field.getContext('2d', { willReadFrequently: true, alpha: false })!;
		this.img = this.fctx.createImageData(W, H);
		const sky = document.createElement('canvas');
		sky.width = W;
		sky.height = H;
		this.sky = sky.getContext('2d', { willReadFrequently: true })!;
		this.acc = new Float32Array(W * H * 3);
		// triangular dither, +-1 LSB, fixed: a held frame is always the same frame
		const r = rng(77);
		this.dither = new Float32Array(W * H);
		for (let i = 0; i < this.dither.length; i++) this.dither[i] = r() - r();
		const s = rng(19);
		for (let i = 0; i < 70; i++) {
			const x = -220 + s() * 2360;
			const y = -180 + s() * 960;
			this.stars.push({ x, y, r: 1.1 + s() ** 2 * 2.1, a: 0.4 + s() * 0.6 });
		}
	}

	/**
	 * Draw the frame at t. `hq` motion-blurs (the render); live playback draws one sample.
	 */
	render(t: number, hq: boolean) {
		const a = stateAt(Math.max(0, t - SHUTTER / 2));
		const b = stateAt(t + SHUTTER / 2);
		const mid = stateAt(t);
		const nf = hq ? Math.max(1, Math.min(96, Math.ceil(fieldMotion(a, b) / 1.25))) : 1;
		const subs: State[] = [];
		for (let i = 0; i < nf; i++)
			subs.push(nf === 1 ? mid : stateAt(Math.max(0, t + ((i + 0.5) / nf - 0.5) * SHUTTER)));
		this.paintField(mid, subs);
		const n = hq ? Math.max(1, Math.min(40, Math.ceil(shapeMotion(a, b) / 1.5))) : 1;
		if (n === 1) {
			this.shapes(mid);
			return;
		}
		const acc = this.acc;
		acc.fill(0);
		for (let i = 0; i < n; i++) {
			this.shapes(stateAt(Math.max(0, t + ((i + 0.5) / n - 0.5) * SHUTTER)));
			const px = this.ctx.getImageData(0, 0, W, H).data;
			for (let p = 0, q = 0; p < px.length; p += 4, q += 3) {
				acc[q] += px[p];
				acc[q + 1] += px[p + 1];
				acc[q + 2] += px[p + 2];
			}
		}
		// the samples share the field's dither, so what holds still stays exactly as a single
		// sample draws it (no grain switching as motion starts and stops)
		const out = this.img.data;
		const inv = 1 / n;
		for (let p = 0, q = 0; p < out.length; p += 4, q += 3) {
			out[p] = acc[q] * inv;
			out[p + 1] = acc[q + 1] * inv;
			out[p + 2] = acc[q + 2] * inv;
			out[p + 3] = 255;
		}
		this.ctx.setTransform(1, 0, 0, 1, 0, 0);
		this.ctx.putImageData(this.img, 0, 0);
	}

	/** The shapes over the field, at one state. */
	private shapes(s: State) {
		const ctx = this.ctx;
		ctx.setTransform(1, 0, 0, 1, 0, 0);
		ctx.globalAlpha = 1;
		ctx.drawImage(this.field, 0, 0);
		const knock = ctx.createPattern(this.field, 'no-repeat')!;
		const knockAt = () => {
			knock.setTransform(ctx.getTransform().inverse());
			return knock;
		};
		const far = layer(s.cam, s.depth);
		const near = layer(s.cam, DEPTH.near);

		// the moon as a shape: never painted where its dark side is (painting the dark side over
		// it would leave a ghost of the full disc in the antialiased rim)
		if (s.moon) {
			const L = s.light;
			ctx.save();
			ctx.setTransform(far.translate(L.x, L.y));
			if (Math.hypot(s.moon.dx, s.moon.dy) < 1.97)
				this.outside(s.moon.dx * L.ro, s.moon.dy * L.ro, L.ro * 0.97);
			ctx.beginPath();
			ctx.arc(0, 0, L.ro, 0, Math.PI * 2);
			ctx.setTransform(1, 0, 0, 1, 0, 0);
			ctx.fillStyle = this.lightFill();
			ctx.fill();
			ctx.restore();
		}

		// the disc in front of the light; the ring's near half back over the disc
		this.disc(s, far);
		if (s.t >= OVER && s.light.alpha > 0) this.ringFront(s, far, knockAt);

		// the horizon line, cut out of the sun
		const gy = s.ground.y;
		if (gy < H + 40) {
			ctx.setTransform(near);
			this.groundPath(s, -3);
			ctx.strokeStyle = knockAt();
			ctx.lineWidth = 6 + 24;
			ctx.stroke();
			this.groundPath(s, -3);
			ctx.strokeStyle = PAPER;
			ctx.lineWidth = 6;
			ctx.stroke();
		}

		// the couch: its knockout, then the ground it stands on, then the couch
		const ct = this.couchMatrix(s, near);
		ctx.setTransform(ct);
		ctx.fillStyle = knockAt();
		ctx.strokeStyle = knockAt();
		ctx.lineWidth = GAP * 2;
		ctx.lineJoin = 'round';
		ctx.fill(this.couchPath);
		ctx.stroke(this.couchPath);
		if (gy < H + 40) {
			ctx.setTransform(near);
			this.groundPath(s, 0);
			ctx.lineTo(W + 600, H + 900);
			ctx.lineTo(-600, H + 900);
			ctx.closePath();
			ctx.fillStyle = rgb(s.ground.col);
			ctx.fill();
		}
		ctx.setTransform(ct);
		ctx.fillStyle = rgb(s.couch.col);
		ctx.fill(this.couchPath);

		// the popcorn, cut out of the couch
		if (s.pop) {
			const pt = near
				.translate(960 - LOCK.w / 2, LOCK.y + s.pop.dy)
				.scale(KL)
				.translate(500, 600)
				.rotate(s.pop.rot)
				.translate(-500, -600);
			ctx.setTransform(pt);
			ctx.fillStyle = knockAt();
			ctx.strokeStyle = knockAt();
			ctx.lineWidth = GAP * 2;
			ctx.fill(this.popPath);
			ctx.stroke(this.popPath);
			ctx.fillStyle = RED;
			ctx.fill(this.popPath);
		}

		this.word(s);

		if (s.fade > 0) {
			ctx.setTransform(1, 0, 0, 1, 0, 0);
			ctx.fillStyle = `rgb(0 0 0 / ${s.fade.toFixed(4)})`;
			ctx.fillRect(0, 0, W, H);
		}
	}

	/** The ground's top edge, `off` px above it, left to right (a path to stroke or close). */
	private groundPath(s: State, off: number) {
		const ctx = this.ctx;
		const { y, bend } = s.ground;
		ctx.beginPath();
		if (bend < 1e-6) {
			ctx.moveTo(-600, y + off);
			ctx.lineTo(W + 600, y + off);
			return;
		}
		const R = 1 / bend;
		const half = Math.min(R * 0.999, 1700);
		const a = Math.asin(half / R);
		ctx.arc(960, y + R, R - off, -Math.PI / 2 - a, -Math.PI / 2 + a);
	}

	private couchMatrix(s: State, cam: DOMMatrix): DOMMatrix {
		const c = s.couch;
		return cam
			.translate(c.x, c.feet)
			.scale(c.w / LOGO_W)
			.translate(0, 456 - 745)
			.rotate(c.rot)
			.translate(-523, -456);
	}

	/** The sun, the planet (with its night side), the pool (lit from its centre). */
	private disc(s: State, cam: DOMMatrix) {
		const ctx = this.ctx;
		const c = s.disc;
		if (c.alpha <= 0 || c.r <= 0.5) return;
		ctx.setTransform(cam.translate(c.x, c.y).scale(1, c.sy));
		ctx.globalAlpha = c.alpha;
		if (c.lit < 1) {
			// the night side: the deeper red under, the lit red over it through a clip that
			// leaves the night side out (no bright rim at the planet's dark edge)
			const night = c.night < 2.2;
			ctx.beginPath();
			ctx.arc(0, 0, c.r, 0, Math.PI * 2);
			ctx.fillStyle = night ? DEEP : RED;
			ctx.fill();
			if (night) {
				const ang = Math.atan2(0.72, 1);
				ctx.save();
				this.outside(Math.cos(ang) * c.night * c.r, Math.sin(ang) * c.night * c.r, c.r * 0.98);
				ctx.beginPath();
				ctx.arc(0, 0, c.r, 0, Math.PI * 2);
				ctx.fillStyle = RED;
				ctx.fill();
				ctx.restore();
			}
		}
		if (c.lit > 0) {
			ctx.beginPath();
			ctx.arc(0, 0, c.r * Math.min(1, c.lit), 0, Math.PI * 2);
			ctx.fillStyle = PAPER;
			ctx.fill();
			// as the light reaches the rim it covers the red's antialiased edge too
			const rim = clamp01((c.lit - 0.97) / 0.03) * 2.4;
			if (rim > 0) {
				ctx.setTransform(1, 0, 0, 1, 0, 0);
				ctx.strokeStyle = PAPER;
				ctx.lineWidth = rim;
				ctx.stroke();
			}
		}
		ctx.globalAlpha = 1;
	}

	/** The ring's near half where it crosses the disc, with its gap cut out of the disc. */
	private ringFront(s: State, cam: DOMMatrix, knockAt: () => CanvasPattern) {
		const ctx = this.ctx;
		const L = s.light;
		const d = s.disc;
		if (d.alpha <= 0 || d.r <= 0.5) return;
		const base = cam.translate(L.x, L.y).rotate(L.rot).scale(1, Math.max(0.02, L.sy));
		const half = () => {
			ctx.beginPath();
			ctx.arc(0, 0, L.ro, 0, Math.PI);
			if (L.ri > 0) ctx.arc(0, 0, L.ri, Math.PI, 0, true);
			else ctx.lineTo(0, 0);
			ctx.closePath();
		};
		ctx.save();
		ctx.setTransform(cam.translate(d.x, d.y).scale(1, d.sy));
		ctx.beginPath();
		ctx.arc(0, 0, d.r + 0.5, 0, Math.PI * 2);
		ctx.clip();
		ctx.globalAlpha = L.alpha;
		// the gap: the band grown by 12 px in screen pixels (stroked without the squash)
		ctx.setTransform(base);
		half();
		ctx.setTransform(1, 0, 0, 1, 0, 0);
		ctx.fillStyle = knockAt();
		ctx.strokeStyle = knockAt();
		ctx.lineWidth = 24;
		ctx.lineJoin = 'round';
		ctx.fill();
		ctx.stroke();
		ctx.setTransform(base);
		half();
		ctx.setTransform(1, 0, 0, 1, 0, 0);
		ctx.fillStyle = this.lightFill();
		ctx.fill();
		ctx.restore();
		ctx.globalAlpha = 1;
	}

	/** Clips to everything outside a circle (in the current transform). */
	private outside(x: number, y: number, r: number) {
		const ctx = this.ctx;
		ctx.beginPath();
		ctx.rect(-1e5, -1e5, 2e5, 2e5);
		ctx.arc(x, y, r, 0, Math.PI * 2);
		ctx.clip('evenodd');
	}

	/** The light's colour as the field paints it (screen space: fill with no transform set). */
	private lightFill(): CanvasGradient {
		const g = this.ctx.createLinearGradient(0, 0.5, 0, H - 0.5);
		g.addColorStop(0, PAPER);
		g.addColorStop(1, PAPER_LOW);
		return g;
	}

	private word(s: State) {
		if (s.word.every((w) => w.a <= 0)) return;
		const ctx = this.ctx;
		ctx.setTransform(1, 0, 0, 1, 0, s.cam.shake);
		ctx.font = `800 ${LOCK.word}px "${FONT_FAMILY}"`;
		ctx.letterSpacing = `${(-0.035 * LOCK.word).toFixed(2)}px`;
		ctx.textAlign = 'left';
		ctx.textBaseline = 'alphabetic';
		const parts = ['couch', 't', 'v'];
		if (!this.wordLayout) {
			const ws = parts.map((p) => ctx.measureText(p).width);
			const m = ctx.measureText('couchtv');
			const total = ws.reduce((x, y) => x + y, 0);
			const x0 = 960 - total / 2;
			const asc = m.fontBoundingBoxAscent;
			const desc = m.fontBoundingBoxDescent;
			this.wordLayout = {
				x: [x0, x0 + ws[0], x0 + ws[0] + ws[1]],
				base: LOCK.wordY + (LOCK.word - (asc + desc)) / 2 + asc
			};
		}
		const L = this.wordLayout;
		parts.forEach((p, i) => {
			const w = s.word[i];
			if (w.a <= 0) return;
			ctx.globalAlpha = w.a;
			ctx.fillStyle = i ? RED : WHITE;
			ctx.fillText(p, L.x[i], L.base + w.dy);
		});
		ctx.globalAlpha = 1;
	}

	/**
	 * The field, per pixel in floats: the sky's gradient, the light (averaged over the shutter's
	 * sub-states, so its rim is motion-blurred exactly), the beam; dithered; then the stars.
	 */
	private paintField(s: State, subs: State[]) {
		const d = this.img.data;
		const dz = this.dither;
		const [top, low] = s.sky.map(labRgb);
		const lt = labRgb(hexLab(PAPER));
		const ll = labRgb(hexLab(PAPER_LOW));
		const rings = subs.map(screenRing);
		const beams = subs.map(screenBeam);
		const nr = rings.length;
		const anyRing = rings.some((r) => r);
		const anyBeam = beams.some((b) => b);
		const rowL = this.rowL;
		const rowB = this.rowB;
		// the stars, as a coverage map under the light (the light hides them)
		let starA: Uint8ClampedArray | null = null;
		if (s.stars > 0) {
			const sc = this.sky;
			sc.setTransform(1, 0, 0, 1, 0, 0);
			sc.clearRect(0, 0, W, H);
			sc.setTransform(layer(s.cam, DEPTH.stars));
			sc.fillStyle = '#fff';
			for (const st of this.stars) {
				sc.globalAlpha = st.a * s.stars;
				sc.beginPath();
				sc.arc(st.x, st.y, st.r, 0, Math.PI * 2);
				sc.fill();
			}
			sc.globalAlpha = 1;
			starA = sc.getImageData(0, 0, W, H).data;
		}
		for (let y = 0; y < H; y++) {
			const g = y / (H - 1);
			const gs = g * g * (3 - 2 * g);
			const br = top[0] + (low[0] - top[0]) * gs;
			const bg = top[1] + (low[1] - top[1]) * gs;
			const bb = top[2] + (low[2] - top[2]) * gs;
			const lr = lt[0] + (ll[0] - lt[0]) * g;
			const lg = lt[1] + (ll[1] - lt[1]) * g;
			const lb = lt[2] + (ll[2] - lt[2]) * g;
			const py = y + 0.5;
			let lit = false;
			if (anyRing) {
				rowL.fill(0);
				for (const e of rings) {
					if (!e) continue;
					const zo = zones(e, e.ro, py);
					if (!zo) continue;
					lit = true;
					const zi = e.ri > 0 ? zones(e, e.ri, py) : null;
					const x0 = Math.max(0, Math.floor(zo.e0));
					const x1 = Math.min(W - 1, Math.ceil(zo.e1));
					for (let x = x0; x <= x1; x++) {
						const px = x + 0.5;
						let c = px > zo.in0 && px < zo.in1 ? 1 : cover(e, e.ro, px, py);
						if (zi && c > 0 && px > zi.e0 && px < zi.e1)
							c -= px > zi.in0 && px < zi.in1 ? 1 : cover(e, e.ri, px, py);
						if (c > 0) rowL[x] += c * e.a;
					}
				}
			}
			let beamed = false;
			if (anyBeam) {
				rowB.fill(0);
				for (const bm of beams) {
					if (!bm || py < bm.ty || py > bm.by) continue;
					beamed = true;
					const v = (py - bm.ty) / Math.max(1, bm.by - bm.ty);
					const cx = bm.tx + (bm.bx - bm.tx) * v;
					const hw = bm.tw + (bm.bw - bm.tw) * v;
					// soft falloff down the beam: brightest under the lamp
					const a = bm.a * (0.15 - 0.06 * v);
					const x0 = Math.max(0, Math.floor(cx - hw - 2));
					const x1 = Math.min(W - 1, Math.ceil(cx + hw + 2));
					for (let x = x0; x <= x1; x++) {
						const px = x + 0.5;
						const c = clamp01(px - (cx - hw) + 0.5) * clamp01(cx + hw - px + 0.5);
						if (c > 0) rowB[x] += c * a;
					}
				}
			}
			let p = y * W * 4;
			let j = y * W;
			for (let x = 0; x < W; x++, p += 4, j++) {
				let r = br,
					gg = bg,
					b = bb;
				if (starA) {
					const a = starA[j * 4 + 3] / 255;
					if (a > 0) {
						r += (lt[0] - r) * a;
						gg += (lt[1] - gg) * a;
						b += (lt[2] - b) * a;
					}
				}
				if (lit) {
					const c = rowL[x] / nr;
					if (c > 0) {
						r += (lr - r) * c;
						gg += (lg - gg) * c;
						b += (lb - b) * c;
					}
				}
				if (beamed) {
					const c = rowB[x] / nr;
					if (c > 0) {
						r += (lt[0] - r) * c;
						gg += (lt[1] - gg) * c;
						b += (lt[2] - b) * c;
					}
				}
				const n = dz[j] + 0.5;
				d[p] = r + n;
				d[p + 1] = gg + n;
				d[p + 2] = b + n;
				d[p + 3] = 255;
			}
		}
		const f = this.fctx;
		f.setTransform(1, 0, 0, 1, 0, 0);
		f.putImageData(this.img, 0, 0);
	}
}

function rgb(lab: Lab): string {
	const [r, g, b] = labRgb(lab).map((v) => Math.round(v));
	return `rgb(${r} ${g} ${b})`;
}
