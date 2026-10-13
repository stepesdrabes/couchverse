// "Assemble": the Couchverse couch is geometry, and the ident builds it. It opens on an eclipse
// close to the lens: a red circle, a white one behind it, orbiting each other on a hairline ring;
// on bar 1 the white one passes in front, then the orbit widens, flattens and the two lock as
// the arms, the white one turning red like a phase of the moon. The dome rises between them like
// a sun from its horizon and unfolds into its place around them; the slab glides in under them;
// the feet step down; the knockout gaps between the parts close and posts grow into the arms:
// one glyph. The camera eases back while the parts find each other, then pulls back in one move
// to the couch alone on black. Then the approved ending: a bucket of popcorn drops onto the
// couch, the two are the Couchverse logo, and couchtv rises in on the three notes of the sonic
// logo.
//
// The picture is one pure function of t (`scene`), drawn into full-frame copies at sub-frame
// times and averaged by stacking (opacity 1, 1/2, 1/3, ...): a 180-degree shutter's motion blur.
// Identical copies average to themselves, so held frames stay crisp.

import { FONT_FAMILY } from '../../../assets';
import { css, div, span } from '../../../core/dom';
import { bezierPoints, cubicBezierAt } from '../../../core/motion';
import { frame } from '../../../core/timeline';
import { timeline, type Ctx, type TL } from '../../../templates/shared';
import {
	ARM_LX,
	ARM_R,
	ARM_RX,
	ARM_Y,
	BASE,
	DOME,
	DOME_BOTTOM,
	FOOT_L,
	FOOT_LX,
	FOOT_R,
	FOOT_RX,
	MID_X,
	POPCORN,
	POST_L,
	POST_R,
	SEAT_Y
} from './geom';
import { LENGTH, T } from './beats';
import { score } from './score';

const SVGNS = 'http://www.w3.org/2000/svg';
const RED = '#e50914';
/** Pure black: it is the one colour the averaged copies reproduce exactly on every tile. */
const BLACK = '#000000';
const WHITE = '#f3eee8';

const W = 1920;
const H = 1080;
/** The lockup: the logo 540 px wide, its top at y 222; couchtv under it. */
const LOCK = { w: 540, y: 222, word: 184, wordY: 650 };
const KL = LOCK.w / 1046;
/** Focal length in logo units: a piece at depth z shows at F / (dist + z) px per unit. */
const F = 1000;
/** The world point the camera looks at when the lockup is framed (the logo's middle, and so on). */
const LOOK_END = { x: LOCK.w / 2 / KL, y: (H / 2 - LOCK.y) / KL };

// --- easing ----------------------------------------------------------------------------------

const clamp01 = (x: number) => (x <= 0 ? 0 : x >= 1 ? 1 : x);
type Curve = (x: number) => number;
const bez = (e: string): Curve => {
	const p = bezierPoints(e);
	return (x) => cubicBezierAt(p, clamp01(x));
};
const E = {
	inOut: bez('cubic-bezier(0.65, 0, 0.35, 1)'),
	sine: bez('cubic-bezier(0.37, 0, 0.63, 1)'),
	out: bez('cubic-bezier(0.22, 1, 0.36, 1)'),
	rise: bez('cubic-bezier(0.32, 0.55, 0.22, 1)'),
	slab: bez('cubic-bezier(0.25, 0.75, 0.2, 1)'),
	type: bez('cubic-bezier(0.2, 0.9, 0.25, 1)')
};
/** Progress of t through [a, b] on a curve. */
const seg = (t: number, a: number, b: number, c: Curve = E.inOut) => c((t - a) / (b - a));
const mix = (a: number, b: number, k: number) => a + (b - a) * k;

// --- the scene -------------------------------------------------------------------------------

/**
 * A smooth curve through keys (monotone cubic, Fritsch-Carlson): no overshoot between keys, flat
 * at the ends, so a camera or a value driven by it never wobbles.
 */
function spline(keys: [number, number][], slope0 = 0, slope1 = 0): (t: number) => number {
	const n = keys.length;
	const xs = keys.map((k) => k[0]);
	const ys = keys.map((k) => k[1]);
	const d: number[] = [];
	for (let i = 0; i < n - 1; i++) d.push((ys[i + 1] - ys[i]) / (xs[i + 1] - xs[i]));
	const m: number[] = new Array(n).fill(0);
	m[0] = slope0;
	m[n - 1] = slope1;
	for (let i = 1; i < n - 1; i++) {
		if (d[i - 1] * d[i] <= 0) m[i] = 0;
		else {
			const w1 = 2 * (xs[i + 1] - xs[i]) + (xs[i] - xs[i - 1]);
			const w2 = xs[i + 1] - xs[i] + 2 * (xs[i] - xs[i - 1]);
			m[i] = (w1 + w2) / (w1 / d[i - 1] + w2 / d[i]);
		}
	}
	return (t) => {
		if (t <= xs[0]) return ys[0];
		if (t >= xs[n - 1]) return ys[n - 1];
		let i = 0;
		while (t > xs[i + 1]) i++;
		const h = xs[i + 1] - xs[i];
		const u = (t - xs[i]) / h;
		const h00 = 2 * u ** 3 - 3 * u ** 2 + 1;
		const h10 = u ** 3 - 2 * u ** 2 + u;
		const h01 = -2 * u ** 3 + 3 * u ** 2;
		const h11 = u ** 3 - u ** 2;
		return h00 * ys[i] + h10 * h * m[i] + h01 * ys[i + 1] + h11 * h * m[i + 1];
	};
}

interface Camera {
	dist: number;
	x: number;
	y: number;
	roll: number;
}

// the pull-back as px per logo unit at the plane, interpolated in log space: it eases back
// while the parts find each other, holds the couch large while it is finished, then pulls
// back in one move to the lockup and settles just before the popcorn drops
const camScale = spline([
	[0, Math.log(2.05)],
	[1.28, Math.log(1.72)],
	[2.56, Math.log(1.32)],
	[3.84, Math.log(1.04)],
	[4.48, Math.log(0.97)],
	[5.04, Math.log(0.93)],
	[5.7, Math.log(KL)]
]);
const camX = spline([
	[0, MID_X + 20],
	[1.28, MID_X - 20],
	[3.84, MID_X],
	[5.04, MID_X],
	[5.7, LOOK_END.x]
]);
const camY = spline([
	[0, ARM_Y + 10],
	[1.28, ARM_Y + 10],
	[2.56, ARM_Y - 40],
	[3.84, ARM_Y - 10],
	[5.04, ARM_Y + 10],
	[5.7, LOOK_END.y]
]);
const camRoll = spline([
	[0, -6],
	[3.84, 0]
]);

function camera(t: number): Camera {
	return { dist: F / Math.exp(camScale(t)), x: camX(t), y: camY(t), roll: camRoll(t) };
}

interface Pose {
	/** World position of the piece's pivot. */
	x: number;
	y: number;
	/** Depth: negative is nearer the lens. */
	z: number;
	rot?: number;
	sx?: number;
	sy?: number;
}

/** A world point through the camera: screen x, y and the scale there (px per logo unit). */
function project(cam: Camera, x: number, y: number, z: number): [number, number, number] {
	const s = F / Math.max(1, cam.dist + z);
	const r = (cam.roll * Math.PI) / 180;
	const dx = (x - cam.x) * s;
	const dy = (y - cam.y) * s;
	return [
		W / 2 + dx * Math.cos(r) - dy * Math.sin(r),
		H / 2 + dx * Math.sin(r) + dy * Math.cos(r),
		s
	];
}

/** The scale (px per logo unit) at depth z. */
const scaleAt = (cam: Camera, z: number) => F / Math.max(1, cam.dist + z);

/** The SVG transform that draws a piece (pivot px, py in logo units) at a pose through the camera. */
function place(cam: Camera, p: Pose, px: number, py: number): string {
	const [sx, sy, s] = project(cam, p.x, p.y, p.z);
	const rot = cam.roll + (p.rot ?? 0);
	const kx = s * (p.sx ?? 1);
	const ky = s * (p.sy ?? 1);
	return `translate(${sx.toFixed(2)} ${sy.toFixed(2)}) rotate(${rot.toFixed(3)}) scale(${kx.toFixed(5)} ${ky.toFixed(5)}) translate(${(-px).toFixed(2)} ${(-py).toFixed(2)})`;
}

interface PieceState {
	tf: string;
	/** Brightness of its red, 1 = the brand red, 0 = black. */
	fill: number;
	/** A white piece taking red ink from (x, y) out to radius r (logo units). */
	ink?: [number, number, number];
	/** Knockout gap around the piece, logo units. */
	gap: number;
	/** Paint order: higher is further back. */
	z: number;
	hidden?: boolean;
}

type PieceName = 'dome' | 'base' | 'postL' | 'postR' | 'footL' | 'footR' | 'armL' | 'armR' | 'pop';

interface State {
	pieces: Record<PieceName, PieceState>;
	/** The orbit's hairline: screen-space pieces with their opacity and width. */
	ring: RingSeg[];
	/** The dome's horizon: a clip in world space, drawn through the camera. */
	horizon: string;
	/** couch, t, v: rise (% of their height, from below) and opacity. */
	word: { y: number; o: number }[];
	/** The frame's give on the landing, screen px. */
	give: number;
}

/**
 * The orbit: two circles circling each other in depth, seen from a little above. It opens on an
 * eclipse (the red one near the lens, a crescent of the white one behind it), shows the other
 * eclipse on bar 1 (white in front), widens and settles into the plane: the arms.
 */
const orbitAngle = spline(
	[
		[0, -0.62 * Math.PI],
		[T.eclipse, -1.44 * Math.PI],
		[T.armsLock, -2 * Math.PI]
	],
	-0.22 * Math.PI
);
const orbitRx = spline([
	[0, 175],
	[T.eclipse, 235],
	[T.armsLock, ARM_RX - MID_X]
]);
/** The orbit's depth as a share of the camera's distance, so the near one never crowds the lens. */
const orbitRz = spline([
	[0, 0.4],
	[T.eclipse, 0.36],
	[T.armsLock, 0]
]);
const orbitTilt = spline([
	[0, 0.26],
	[T.eclipse, 0.22],
	[T.armsLock, 0]
]);

function orbitPoint(t: number, cam: Camera, a: number) {
	const rx = orbitRx(t);
	const rz = orbitRz(t) * cam.dist;
	const tilt = orbitTilt(t);
	const z = rz * Math.sin(a);
	return { x: MID_X + rx * Math.cos(a), y: ARM_Y - tilt * z, z };
}

function arms(t: number, cam: Camera) {
	const th = orbitAngle(t);
	return { right: orbitPoint(t, cam, th), left: orbitPoint(t, cam, th + Math.PI) };
}

/** How much of the orbit's hairline shows: arcs of this half-width around each circle. */
const ringSpan = (t: number) =>
	(Math.PI / 2) * (seg(t, 0.12, 1.1, E.inOut) - seg(t, 1.5, 2.16, E.inOut));

/** Pieces of the hairline, so its near side can be brighter and a touch heavier than its far side. */
const RING_SEGMENTS = 14;

interface RingSeg {
	d: string;
	o: number;
	w: number;
}

/** The orbit's hairline in perspective: drawn out of the circles, reeled back in as they lock. */
function ring(t: number, cam: Camera): RingSeg[] {
	const w = ringSpan(t);
	const none = { d: '', o: 0, w: 0 };
	if (w < 0.002) return Array.from({ length: RING_SEGMENTS * 2 }, () => none);
	const th = orbitAngle(t);
	const rz = Math.max(1e-6, orbitRz(t) * cam.dist);
	const segs: RingSeg[] = [];
	for (const c of [th, th + Math.PI]) {
		for (let k = 0; k < RING_SEGMENTS; k++) {
			const a0 = c - w + (2 * w * k) / RING_SEGMENTS;
			const a1 = c - w + (2 * w * (k + 1)) / RING_SEGMENTS;
			const n = Math.max(2, Math.ceil(((a1 - a0) * 180) / Math.PI / 1.5));
			const pts: string[] = [];
			for (let i = 0; i <= n; i++) {
				const p = orbitPoint(t, cam, a0 + ((a1 - a0) * i) / n);
				const [x, y] = project(cam, p.x, p.y, p.z);
				pts.push(`${x.toFixed(1)} ${y.toFixed(1)}`);
			}
			// -1 nearest the lens, +1 farthest
			const depth = Math.max(-1, Math.min(1, orbitPoint(t, cam, (a0 + a1) / 2).z / rz));
			const near = (1 - depth) / 2;
			segs.push({ d: `M${pts.join('L')}`, o: mix(0.32, 0.92, near), w: mix(1.5, 2.8, near) });
		}
	}
	return segs;
}

/** Far is darker: a piece's red at depth z (none in front of the plane). */
function shade(z: number) {
	const k = clamp01(z / 1800);
	return 1 - 0.78 * (1 - (1 - k) ** 2);
}

function scene(t: number): State {
	t = Math.max(0, Math.min(t, LENGTH));
	const cam = camera(t);
	// the couch takes the popcorn's weight
	const lt = t - T.land;
	const press = lt >= 0 && lt < 0.26 ? 12 * Math.sin((lt / 0.26) * Math.PI) : 0;
	// the knockout gaps between the parts (screen px, the logo's own at the lockup) close: one glyph
	const gapPx = 16 * (1 - seg(t, T.fuse - 0.12, T.fuse + 0.28, E.inOut));
	const gap = (z: number) => gapPx / scaleAt(cam, z);

	const m = arms(t, cam);
	// the white one takes the red as it locks, like a phase of the moon: a red disc slides across
	// it from its partner's side
	const ik = seg(t, T.armsLock - 0.1, T.armsLock + 0.42, E.inOut);
	const armInk: [number, number, number] | undefined =
		ik >= 1 ? undefined : [ARM_LX + 2.15 * ARM_R * (1 - ik), ARM_Y, ARM_R + 6];
	const armL = place(cam, { x: m.left.x, y: m.left.y + press, z: m.left.z }, ARM_LX, ARM_Y);
	const armR = place(cam, { x: m.right.x, y: m.right.y + press, z: m.right.z }, ARM_RX, ARM_Y);

	// the dome rises between them like a sun from its horizon (its own bottom edge) and unfolds
	// sideways, its hollows closing round the arms as it locks
	const d = seg(t, T.armsLock - 0.62, T.domeLock, E.rise);
	const spread = seg(t, T.armsLock - 0.3, T.domeLock, E.inOut);
	const dome = place(
		cam,
		{ x: MID_X, y: DOME_BOTTOM + 380 * (1 - d) + press, z: 0, sx: mix(0.62, 1, spread) },
		MID_X,
		DOME_BOTTOM
	);
	const horizon = place(cam, { x: MID_X, y: DOME_BOTTOM + press, z: 0 }, MID_X, DOME_BOTTOM);

	// the slab glides in from the left, nearer the lens, and settles into the plane under them
	const g = seg(t, T.baseLock - 1.2, T.baseLock, E.slab);
	const baseZ = -300 * (1 - g);
	const base = place(
		cam,
		{ x: MID_X - 1900 * (1 - g), y: 630 + 40 * (1 - g) + press, z: baseZ, rot: -5 * (1 - g) },
		MID_X,
		630
	);

	// the posts rise out of the seat into the arms as the gaps close, filling the notch under each
	const pg = seg(t, T.fuse - 0.1, T.fuse + 0.3, E.inOut);
	const postSy = Math.max(0.001, pg);
	const postL = place(cam, { x: 150, y: SEAT_Y + press, z: 0, sy: postSy }, 150, SEAT_Y);
	const postR = place(cam, { x: 896, y: SEAT_Y + press, z: 0, sy: postSy }, 896, SEAT_Y);

	// the feet step down out of the slab
	const fl = seg(t, T.footL - 0.3, T.footL, E.out);
	const fr = seg(t, T.footR - 0.3, T.footR, E.out);
	const footL = place(cam, { x: FOOT_LX, y: 728 - 36 * (1 - fl) + press, z: 0 }, FOOT_LX, 728);
	const footR = place(cam, { x: FOOT_RX, y: 728 - 36 * (1 - fr) + press, z: 0 }, FOOT_RX, 728);

	// the popcorn: gravity, a small rebound, still
	let pdy = 0;
	let prot = 0;
	const fall = (t - T.fall) / (T.land - T.fall);
	if (fall < 1) {
		const q = clamp01(fall) ** 2;
		pdy = -1480 * (1 - q);
		prot = 14 * (1 - q);
	} else {
		const q = lt / 0.34;
		if (q < 1) {
			pdy = -44 * Math.sin(q * Math.PI) * (1 - q);
			prot = -2.2 * Math.sin(q * Math.PI) * (1 - q);
		}
	}
	const pop = place(cam, { x: 500, y: 600 + pdy, z: 0, rot: prot }, 500, 600);

	const pieces: Record<PieceName, PieceState> = {
		dome: { tf: dome, fill: 1, gap: gap(0), z: 5000 },
		base: { tf: base, fill: 1, gap: gap(baseZ), z: 4000 },
		postL: { tf: postL, fill: 1, gap: 0, z: 4600, hidden: pg <= 0 },
		postR: { tf: postR, fill: 1, gap: 0, z: 4600, hidden: pg <= 0 },
		footL: { tf: footL, fill: 1, gap: 0, z: 4500, hidden: t < T.baseLock - 0.3 },
		footR: { tf: footR, fill: 1, gap: 0, z: 4500, hidden: t < T.baseLock - 0.3 },
		armL: { tf: armL, fill: 1, ink: armInk, gap: gap(m.left.z), z: m.left.z },
		armR: { tf: armR, fill: shade(m.right.z), gap: gap(m.right.z), z: m.right.z },
		pop: { tf: pop, fill: 1, gap: 30, z: -10000, hidden: t < T.fall - 0.05 }
	};

	// couchtv: couch on the first note, t on the second, v on the third, each rising into place
	const word = T.notes.map((n, i) => {
		const k = seg(t, n - 0.02, n + (i ? 0.3 : 0.42), E.type);
		return { y: 104 * (1 - k), o: seg(t, n - 0.02, n + 0.1, E.sine) };
	});

	// the landing: the frame gives a little and settles, a damped swing
	const give = lt >= 0 && lt < 0.6 ? 7 * Math.sin(lt * Math.PI * 4.2) * Math.exp(-lt * 9) : 0;

	return { pieces, ring: ring(t, cam), horizon, word, give };
}

// --- drawing ---------------------------------------------------------------------------------

function el<K extends keyof SVGElementTagNameMap>(
	tag: K,
	attrs: Record<string, string | number>,
	parent: Element
): SVGElementTagNameMap[K] {
	const e = document.createElementNS(SVGNS, tag);
	for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, String(v));
	parent.appendChild(e);
	return e;
}

interface Part {
	g: SVGGElement;
	halo: SVGElement;
	body: SVGElement;
	ink: SVGElement;
	inkDot: SVGCircleElement;
}

/** A piece: a halo in the field's colour (its knockout gap), the body, and red ink for a white body. */
function part(parent: Element, defs: Element, id: string, shape: (p: Element) => SVGElement): Part {
	const g = el('g', {}, parent);
	const halo = shape(g);
	halo.setAttribute('fill', BLACK);
	halo.setAttribute('stroke', BLACK);
	halo.setAttribute('stroke-linejoin', 'round');
	const body = shape(g);
	const clip = el('clipPath', { id, clipPathUnits: 'userSpaceOnUse' }, defs);
	const inkDot = el('circle', { cx: 0, cy: 0, r: 0 }, clip);
	const ink = shape(g);
	ink.setAttribute('fill', RED);
	ink.setAttribute('clip-path', `url(#${id})`);
	return { g, halo, body, ink, inkDot };
}

const RGB = [0xe5, 0x09, 0x14];
/** The brand red at a brightness (1 is the red itself, exactly). */
const red = (k: number) =>
	k >= 1 ? RED : `rgb(${RGB.map((v) => Math.round(v * clamp01(k))).join(',')})`;

const path = (d: string) => (p: Element) => el('path', { d }, p);
const circle = (cx: number) => (p: Element) => el('circle', { cx, cy: ARM_Y, r: ARM_R }, p);

const CSS = `
.asm { position: absolute; inset: 0; overflow: hidden; background: ${BLACK}; }
.asm-copy { position: absolute; inset: 0; background: ${BLACK}; }
.asm-dark { position: absolute; inset: 0; background: ${BLACK}; pointer-events: none; }
.asm-copy > svg { position: absolute; left: 0; top: 0; overflow: visible; }
.asm-word { position: absolute; left: 0; width: ${W}px; display: flex; justify-content: center;
  font-family: '${FONT_FAMILY}'; font-weight: 800; letter-spacing: -0.035em; line-height: 1;
  color: #fff; white-space: nowrap; }
.asm-word .mask { display: inline-block; clip-path: inset(-40% -30% 0 -30%); }
.asm-word .mask > span { display: inline-block; }
.asm-word .tv { color: ${RED}; }
`;

interface Copy {
	svg: SVGSVGElement;
	world: SVGGElement;
	parts: Record<PieceName, Part>;
	order: string;
	horizon: SVGRectElement;
	ring: SVGPathElement[];
	words: HTMLElement[];
}

const NAMES: PieceName[] = [
	'dome',
	'footL',
	'footR',
	'postL',
	'postR',
	'base',
	'armL',
	'armR',
	'pop'
];

const SHAPES: Record<PieceName, (p: Element) => SVGElement> = {
	dome: path(DOME),
	base: path(BASE),
	postL: path(POST_L),
	postR: path(POST_R),
	footL: path(FOOT_L),
	footR: path(FOOT_R),
	armL: circle(ARM_LX),
	armR: circle(ARM_RX),
	pop: path(POPCORN)
};

function buildCopy(stage: HTMLElement, k: number, id: string): Copy {
	const root = css(div('asm-copy', stage), { opacity: String(1 / (k + 1)) });
	const svg = el('svg', { width: W, height: H, viewBox: `0 0 ${W} ${H}` }, root);
	const defs = el('defs', {}, svg);
	const clip = el('clipPath', { id: `${id}-h`, clipPathUnits: 'userSpaceOnUse' }, defs);
	// the horizon: everything above the dome's bottom edge (a hair below it, so it never cuts the edge at rest)
	const horizon = el(
		'rect',
		{ x: -6000, y: -6000, width: 13000, height: 6000 + DOME_BOTTOM + 1 },
		clip
	);
	const ringG = el(
		'g',
		{ fill: 'none', stroke: WHITE, 'stroke-linecap': 'butt', 'stroke-linejoin': 'round' },
		svg
	);
	const ringPaths = Array.from({ length: RING_SEGMENTS * 2 }, () =>
		el('path', { d: 'M0 0' }, ringG)
	);
	const world = el('g', {}, svg);
	const domeClip = el('g', { 'clip-path': `url(#${id}-h)` }, world);
	const parts = {} as Record<PieceName, Part>;
	for (const n of NAMES) {
		const parent = n === 'dome' ? domeClip : world;
		parts[n] = part(parent, defs, `${id}-${n}`, SHAPES[n]);
	}
	const word = css(div('asm-word', root), { top: `${LOCK.wordY}px`, fontSize: `${LOCK.word}px` });
	// each part rises from behind an invisible line just under the baseline (the mask's edge)
	const words = (
		[
			[null, 'couch'],
			['tv', 't'],
			['tv', 'v']
		] as const
	).map(([cls, text]) => span(cls, span('mask', word), text));
	return { svg, world, parts, order: '', horizon, ring: ringPaths, words };
}

function draw(c: Copy, s: State) {
	c.svg.style.transform = s.give ? `translate(0px, ${s.give.toFixed(2)}px)` : '';
	c.horizon.setAttribute('transform', s.horizon);
	s.ring.forEach((r, i) => {
		const e = c.ring[i];
		e.setAttribute('d', r.d || 'M0 0');
		e.setAttribute('stroke-opacity', r.d ? r.o.toFixed(3) : '0');
		e.setAttribute('stroke-width', r.w.toFixed(2));
	});
	for (const name of NAMES) {
		const ps = s.pieces[name];
		const p = c.parts[name];
		p.g.setAttribute('transform', ps.tf);
		p.g.setAttribute('visibility', ps.hidden ? 'hidden' : 'visible');
		p.halo.setAttribute('stroke-width', (ps.gap * 2).toFixed(2));
		p.halo.setAttribute('visibility', ps.gap > 0.05 ? 'inherit' : 'hidden');
		p.body.setAttribute('fill', ps.ink ? WHITE : red(ps.fill));
		p.ink.setAttribute('visibility', ps.ink ? 'inherit' : 'hidden');
		if (ps.ink) {
			p.inkDot.setAttribute('cx', ps.ink[0].toFixed(2));
			p.inkDot.setAttribute('cy', ps.ink[1].toFixed(2));
			p.inkDot.setAttribute('r', ps.ink[2].toFixed(2));
		}
	}
	// paint far to near (the dome stays in its horizon clip, at the back)
	const movable = NAMES.filter((n) => n !== 'dome').sort((a, b) => s.pieces[b].z - s.pieces[a].z);
	const order = movable.join(',');
	if (order !== c.order) {
		c.order = order;
		for (const n of movable) c.world.appendChild(c.parts[n].g);
	}
	s.word.forEach((w, i) => {
		const e = c.words[i];
		e.style.opacity = w.o.toFixed(3);
		e.style.transform = `translateY(${w.y.toFixed(2)}%)`;
	});
}

/** Clip ids must be unique within the stage's shadow root. */
let builds = 0;

/** Sub-frame samples per frame and the shutter (fraction of a frame). */
const SAMPLES = 16;
const SHUTTER = 0.5;

export function assemble({ root, sound }: Ctx): TL {
	const D = frame(LENGTH);
	const tl = timeline(root, D);
	const style = document.createElement('style');
	style.textContent = CSS;
	root.appendChild(style);
	const stage = div('asm', root);
	const id = `asm${++builds}`;
	const offs = Array.from(
		{ length: SAMPLES },
		(_, k) => ((k + 0.5) / SAMPLES - 0.5) * (SHUTTER / 25)
	);
	offs.forEach((off, k) => {
		const copy = buildCopy(stage, k, `${id}-${k}`);
		tl.dynamic(
			(t) => JSON.stringify(scene(t + off)),
			(v) => draw(copy, JSON.parse(String(v)) as State)
		);
	});
	// the eclipse comes up out of the dark with the sound, instead of cutting in
	const dark = div('asm-dark', stage);
	tl.dynamic(
		(t) => (1 - seg(t, 0, 0.4, E.sine)).toFixed(3),
		(v) => (dark.style.opacity = String(v))
	);
	tl.cue('the orbit', 0);
	tl.cue('the eclipse', T.eclipse);
	tl.cue('the arms lock', T.armsLock);
	tl.cue('the dome', T.domeLock);
	tl.cue('the slab', T.baseLock);
	tl.cue('one glyph', T.fuse);
	tl.cue('the popcorn falls', T.fall);
	tl.cue('the logo', T.land);
	tl.cue('couchtv', T.notes[0]);
	// one cue for the whole score, built before an offline render starts (as the approved render
	// built it): its voices start slow oscillators up to a period early and its chorus takes its
	// phase from the moment it is built, while a render schedules a cue only shortly before it
	// plays. Live playback from part-way skips what is over, as a cue of its own would be.
	const events = score(sound, D);
	tl.sound(
		0,
		(w, b, skip) => {
			for (const e of events)
				if (e.at >= skip - 0.03 || skip < e.at + e.dur) e.fn(w - skip + e.at, b);
		},
		D
	);
	return tl;
}
