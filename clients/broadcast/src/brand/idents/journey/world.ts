// The journey's world, drawn on a canvas as a pure function of t: one panorama in depth (dusk
// dunes running down to the sea, the sun setting into it, a city lighting up on the far shore,
// the stars), seen from a camera that trucks along with the floating couch and then cranes up
// while the world's lights go down. Every layer sits at a depth p (1 is the couch's own plane):
// it moves p times as far as the truck and the crane, while the tilt moves everything together.

import { rng } from '../../../core/rng';
import { COUCH, GAP, POPCORN } from '../../glyph';
import { css, gradientStops, hex, mix, PAL, ramp, type RGB } from './color';
import { smooth, smoother } from './curves';
import {
	ALONE,
	camera,
	couchPose,
	dim,
	DROP,
	LAND,
	LOCK,
	night,
	SUN_R,
	SUN_X,
	sunAlt,
	truck,
	type Camera
} from './plan';

export const W = 1920;
export const H = 1080;

type C2D = CanvasRenderingContext2D;

const fade = (c: RGB, k: number) => mix(c, PAL.black, k);

// --- sky -------------------------------------------------------------------------------------

/**
 * Dusk, by px above the horizon. The world stays in the red's deeper tones so that the couch,
 * the one thing in pure brand red, always reads, and the sun is the one warm white.
 */
const skyDusk = ramp([
	[0, hex('#b80810')],
	[90, hex('#a3060e')],
	[250, hex('#82050b')],
	[450, hex('#5a040a')],
	[680, hex('#340206')],
	[980, hex('#180203')],
	[1300, hex('#0a0203')],
	[1700, PAL.black]
]);
/** Night: the city's dim red glow low down, black above. */
const skyNight = ramp([
	[0, hex('#300408')],
	[60, hex('#1c0305')],
	[200, hex('#0e0204')],
	[460, hex('#080304')],
	[820, PAL.black],
	[1700, PAL.black]
]);

/** The sky's colour at a height above the horizon, at time t (without the sun's glow). */
function skyAt(alt: number, t: number): RGB {
	return fade(mix(skyDusk(alt), skyNight(alt), night(t)), dim(t));
}

function drawSky(ctx: C2D, cam: Camera, t: number) {
	const n = night(t);
	const top = 1700;
	const g = ctx.createLinearGradient(0, cam.hz, 0, cam.hz - top);
	gradientStops(g, (x) => skyAt(x * top, t), 48);
	ctx.fillStyle = g;
	ctx.fillRect(0, 0, W, Math.min(H, cam.hz + 1));
	// at night the city lights the haze above it: a low, wide red glow that goes out with it
	const cityGlow = smooth(2.8, 4.0, t) * (1 - dim(t));
	if (cityGlow > 0.002) {
		ctx.save();
		ctx.beginPath();
		ctx.rect(0, 0, W, Math.min(H, cam.hz + 1));
		ctx.clip();
		ctx.translate(1560 - (cam.x - truck(3.2)) * 0.08, cam.hz);
		ctx.scale(2.6, 1);
		const glow = ctx.createRadialGradient(0, 0, 0, 0, 0, 380);
		const tone = hex('#5a050b');
		gradientStops(
			glow,
			() => tone,
			20,
			(x) => 0.75 * cityGlow * (1 - smoother(0, 1, x)) ** 1.7
		);
		ctx.fillStyle = glow;
		ctx.fillRect(-W, -H, W * 2, H * 2);
		ctx.restore();
	}
	// the horizon burns brightest under the sun: a wide, low glow that cools as it sets
	const alt = sunAlt(t);
	const heat = smooth(-SUN_R * 0.8, 120, alt) * (1 - n);
	if (heat > 0.002) {
		ctx.save();
		ctx.beginPath();
		ctx.rect(0, 0, W, Math.min(H, cam.hz + 1));
		ctx.clip();
		ctx.translate(SUN_X, cam.hz);
		ctx.scale(3.2, 1);
		const glow = ctx.createRadialGradient(0, 0, 0, 0, 0, 420);
		const hot = hex('#d00a12');
		gradientStops(
			glow,
			() => hot,
			24,
			(x) => 0.85 * heat * (1 - smoother(0, 1, x)) ** 1.5
		);
		ctx.fillStyle = glow;
		ctx.fillRect(-W, -H, W * 2, H * 2);
		ctx.restore();
	}
}

function drawSun(ctx: C2D, cam: Camera, t: number) {
	const alt = sunAlt(t);
	if (alt < -SUN_R - 2) return;
	const sy = cam.hz - alt;
	// a breath of atmosphere: the lower limb a shade warmer than the top
	const g = ctx.createLinearGradient(0, sy - SUN_R, 0, sy + SUN_R);
	gradientStops(g, (k) => mix(PAL.warm, mix(PAL.warm, PAL.red, 0.16), smoother(0.35, 1, k)), 10);
	ctx.fillStyle = g;
	ctx.beginPath();
	ctx.arc(SUN_X, sy, SUN_R, 0, Math.PI * 2);
	ctx.fill();
}

// --- stars -----------------------------------------------------------------------------------

interface Star {
	x: number;
	/** Height above the horizon, px. */
	a: number;
	/** Depth: a touch of parallax, so the field has depth as the camera climbs. */
	p: number;
	b: number;
	r: number;
	f: number;
	ph: number;
	/** When it comes out, 0..1 along the dusk. */
	on: number;
}

const STARS: Star[] = (() => {
	const r = rng(77);
	const out: Star[] = [];
	const star = (x: number, a: number, b: number) =>
		out.push({
			x,
			a,
			p: 0.004 + r() * 0.03,
			b: 0.4 + b * 0.6,
			r: 0.85 + b * 2.1,
			f: 0.2 + r() * 0.45,
			ph: r() * Math.PI * 2,
			on: r()
		});
	for (let i = 0; i < 420; i++) star(r() * 2100 - 90, 70 + r() ** 0.85 * 1550, r() ** 2.8);
	// a faint band of the galaxy: many small stars along a gentle diagonal
	for (let i = 0; i < 360; i++) {
		const k = r();
		const across = (r() + r() + r() - 1.5) * 150;
		star(-200 + k * 2400 + across * 0.35, 420 + k * 640 + across, r() ** 4 * 0.55);
	}
	return out;
})();

/** The stars go out from the edges in toward the couch, the way a cinema's lights go down. */
function starsOut(x: number, y: number, t: number): number {
	const d = Math.min(1, Math.hypot(x - 960, (y - 457) * 1.4) / 1150);
	const start = 5.0 + (1 - d) * 0.42;
	return 1 - smoother(start, start + 0.26, t);
}

function drawStars(ctx: C2D, cam: Camera, t: number) {
	const n = night(t);
	if (n <= 0.001) return;
	ctx.fillStyle = css(PAL.warm);
	for (const s of STARS) {
		// high stars come out first, low ones last
		const at = 0.2 + s.on * 0.45 + (1 - Math.min(1, s.a / 900)) * 0.2;
		const appear = smoother(at, at + 0.2, n);
		if (appear <= 0) continue;
		const x = s.x - cam.x * s.p;
		const y = cam.hz - s.a + (cam.h - 300) * s.p;
		if (y < -10 || y > Math.min(H, cam.hz) + 4 || x < -10 || x > W + 10) continue;
		const out = starsOut(x, y, t);
		if (out <= 0) continue;
		const tw = 1 + 0.14 * Math.sin(t * s.f * Math.PI * 2 + s.ph);
		// stars low over the city's glow are washed out
		const wash = smooth(20, 200, s.a);
		const alpha = Math.min(1, s.b * 1.2) * appear * out * tw * wash;
		if (alpha <= 0.004) continue;
		ctx.globalAlpha = Math.min(1, alpha);
		ctx.beginPath();
		ctx.arc(x, y, s.r, 0, Math.PI * 2);
		ctx.fill();
	}
	ctx.globalAlpha = 1;
}

// --- the sea ---------------------------------------------------------------------------------

/** px below the horizon -> colour, at dusk and at night: the sky's deep reds, darker. */
const seaDusk = ramp([
	[0, hex('#a0060e')],
	[4, hex('#86050c')],
	[26, hex('#5a040a')],
	[95, hex('#330307')],
	[270, hex('#1a0204')],
	[600, hex('#0c0203')]
]);
const seaNight = ramp([
	[0, hex('#240307')],
	[16, hex('#110204')],
	[140, hex('#080304')],
	[600, PAL.black]
]);

interface Glint {
	u: number;
	/** Depth, from the horizon (0) to under the camera. */
	p: number;
	len: number;
	f: number;
	ph: number;
}

const GLINTS: Glint[] = (() => {
	const r = rng(31);
	const out: Glint[] = [];
	for (let i = 0; i < 300; i++)
		out.push({
			u: r() * 2600,
			p: 0.02 + r() ** 1.5 * 1.8,
			len: 0.5 + r(),
			f: 0.25 + r() * 0.5,
			ph: r() * 6.283
		});
	return out;
})();

/** The sun's path on the water: strokes held under the sun (it follows the eye), shimmering. */
const GLITTER = (() => {
	const r = rng(57);
	const out: { d: number; x: number; len: number; f: number; ph: number }[] = [];
	for (let i = 0; i < 84; i++) {
		const d = r() ** 1.4;
		const side = r() * 2 - 1;
		out.push({
			d,
			x: Math.sign(side) * Math.abs(side) ** 1.6,
			len: 0.4 + r() * 0.8,
			f: 0.3 + r() * 0.7,
			ph: r() * 6.283
		});
	}
	return out;
})();

function stroke(ctx: C2D, x: number, y: number, len: number) {
	ctx.beginPath();
	ctx.moveTo(x - len / 2, y);
	ctx.lineTo(x + len / 2, y);
	ctx.stroke();
}

function drawSea(ctx: C2D, cam: Camera, t: number) {
	if (cam.hz >= H) return;
	const n = night(t);
	const out = dim(t);
	const g = ctx.createLinearGradient(0, cam.hz, 0, cam.hz + 600);
	gradientStops(g, (x) => fade(mix(seaDusk(x * 600), seaNight(x * 600), n), out), 40);
	ctx.fillStyle = g;
	ctx.fillRect(0, cam.hz, W, H - cam.hz);

	ctx.lineCap = 'round';
	// ripples catching the sky, each at its own depth, so the water slides by in perspective
	const tone = mix(mix(PAL.red, PAL.warm, 0.18), hex('#4a0610'), n);
	const level = 0.16 * (1 - 0.6 * n) * (1 - out);
	if (level > 0.002)
		for (const gl of GLINTS) {
			const y = cam.hz + cam.h * gl.p;
			if (y > H + 4) continue;
			const span = 2600;
			const x = ((((gl.u - cam.x * gl.p) % span) + span) % span) - 340;
			const s = 0.5 + 0.5 * Math.sin(t * gl.f * 6.283 + gl.ph);
			const len = (5 + 40 * gl.p) * gl.len * (0.4 + 0.6 * s);
			ctx.strokeStyle = css(tone, level * s * smooth(0.02, 0.08, gl.p));
			ctx.lineWidth = 0.9 + gl.p * 1.3;
			stroke(ctx, x, y, len);
		}

	// the sun's path
	const alt = sunAlt(t);
	const light = smooth(-SUN_R * 0.85, 40, alt) * (1 - n);
	if (light > 0.002) {
		const reach = 470;
		for (const gl of GLITTER) {
			const d = 1.5 + gl.d * reach;
			const y = cam.hz + d;
			if (y > H) continue;
			const k = d / reach;
			// narrow at the horizon, opening toward us
			const width = SUN_R * (0.28 + 0.5 * k);
			const s = 0.5 + 0.5 * Math.sin(t * gl.f * 6.283 + gl.ph);
			const x = SUN_X + gl.x * width;
			const len = (14 + 110 * k) * gl.len * (0.3 + 0.7 * s);
			const a = light * (0.25 + 0.75 * s) * (1 - Math.abs(gl.x) ** 2) * (1 - k * 0.55) * 0.85;
			ctx.strokeStyle = css(mix(PAL.warm, PAL.red, 0.12 + k * 0.45), a);
			ctx.lineWidth = 0.9 + 1.7 * k;
			stroke(ctx, x, y, len);
		}
	}
	// the city's glow lies on the water below it
	const cityGlow = smooth(2.8, 4.0, t) * (1 - dim(t));
	if (cityGlow > 0.002) {
		ctx.save();
		ctx.beginPath();
		ctx.rect(0, cam.hz, W, H - cam.hz);
		ctx.clip();
		ctx.translate(1560 - (cam.x - truck(3.2)) * 0.08, cam.hz);
		ctx.scale(2.4, 1);
		const glow = ctx.createRadialGradient(0, 0, 0, 0, 0, 260);
		const tone = hex('#3a0408');
		gradientStops(
			glow,
			() => tone,
			16,
			(x) => 0.7 * cityGlow * (1 - smoother(0, 1, x)) ** 1.6
		);
		ctx.fillStyle = glow;
		ctx.fillRect(-W, -10, W * 2, H);
		ctx.restore();
	}
	drawCityReflections(ctx, cam, t);
}

// --- land ------------------------------------------------------------------------------------

interface Strip {
	/** Depth of the ridge (the strip's back) and of its front edge. */
	p: number;
	pf: number;
	/** Where the land runs out into the sea (layer px). */
	end: number;
	/** The dunes' own tone, and the mist that lies in the valleys in front of them. */
	tone: RGB;
	mist: RGB;
	/** How far the sun's light reaches into this layer (far layers glow, near ones stay dark). */
	glow: number;
	/** How long the land takes to sink into the sea (1 is the default length). */
	reach: number;
	ridge: (u: number) => number;
}

/**
 * A dune line: a few long swells plus crests, each crest an asymmetric peak (a long windward
 * flank, a steeper lee) rounded only at its tip. The swells keep the troughs from going flat.
 */
function dunes(seed: number, spacing: number, lo: number, hi: number) {
	const r = rng(seed);
	const bumps: { c: number; w: number; h: number; k: number }[] = [];
	for (let c = -spacing * 2; c < 9000; c += spacing * (0.75 + r() * 0.6))
		bumps.push({
			c,
			w: spacing * (0.55 + r() * 0.3),
			h: lo + (hi - lo) * r() ** 0.8,
			k: 0.5 + r() * 0.3
		});
	const swell = [0, 1, 2].map(() => ({
		l: spacing * (1.6 + r() * 2),
		ph: r() * 6.283,
		a: lo * (0.15 + r() * 0.2)
	}));
	return (u: number) => {
		// a soft maximum (a cubic norm): crests stay sharp, the troughs between them round off
		let e3 = 0;
		for (const b of bumps) {
			const s = (u - b.c) / b.w;
			if (s < -4 || s > 4) continue;
			const a = s < 0 ? -s : s / b.k;
			e3 += (b.h * Math.exp(-((a * a + 0.002) ** 0.62) * 1.2)) ** 3;
		}
		let e = Math.cbrt(e3);
		for (const w of swell) e += w.a * (1 + Math.sin((u / w.l) * 6.283 + w.ph));
		return e;
	};
}

/** Where a strip's land must end so its tip passes screen x `x` at time `t`. */
const shoreAt = (p: number, x: number, t: number) => x + truck(t) * p;

const STRIPS: Strip[] = [
	{
		p: 0.05,
		pf: 0.12,
		end: shoreAt(0.05, 660, 0),
		tone: hex('#7e050c'),
		mist: hex('#9c060e'),
		glow: 1,
		reach: 1.3,
		ridge: dunes(3, 300, 14, 46)
	},
	{
		p: 0.12,
		pf: 0.26,
		end: shoreAt(0.12, 600, 0.6),
		tone: hex('#62040a'),
		mist: hex('#86050c'),
		glow: 0.9,
		reach: 0.75,
		ridge: dunes(7, 430, 28, 92)
	},
	{
		p: 0.26,
		pf: 0.48,
		end: shoreAt(0.26, 660, 1.2),
		tone: hex('#470308'),
		mist: hex('#6a040a'),
		glow: 0.75,
		reach: 1.15,
		ridge: dunes(11, 580, 46, 140)
	},
	{
		p: 0.48,
		pf: 0.8,
		end: shoreAt(0.48, 740, 1.55),
		tone: hex('#300206'),
		mist: hex('#58040a'),
		glow: 0.65,
		reach: 0.8,
		ridge: dunes(13, 740, 64, 185)
	},
	{
		p: 0.8,
		pf: 1.2,
		end: shoreAt(0.8, 820, 1.85),
		tone: hex('#1f0204'),
		mist: hex('#44040a'),
		glow: 0.5,
		reach: 1.05,
		ridge: dunes(17, 940, 80, 220)
	}
];
/** The foreground dunes pass in front of the couch. */
const FRONT: Strip[] = [
	{
		p: 1.25,
		pf: 3,
		end: shoreAt(1.25, 900, 2.1),
		tone: hex('#140102'),
		mist: hex('#2c0305'),
		glow: 0.35,
		reach: 0.9,
		ridge: dunes(23, 1250, 22, 70)
	}
];

/** The light the low sun throws on the dunes' flanks and crests. */
const SUNLIT = hex('#c0080f');

function drawStrip(ctx: C2D, cam: Camera, t: number, s: Strip) {
	const off = cam.x * s.p;
	const yb = cam.hz + cam.h * s.p;
	const yf = s.pf >= 3 ? H + 60 : cam.hz + cam.h * s.pf;
	const x0 = -20;
	const x1 = Math.min(W + 20, s.end - off);
	if (x1 <= x0 || yb - 320 > H) return;
	// the dunes sink toward the water over `taper` (each strip its own length, so the spits of
	// sand never line up); the land's front edge rises to meet them
	const taper = (460 + 760 * s.p) * s.reach;
	const step = 3;
	const top: [number, number][] = [];
	const bottom: [number, number][] = [];
	let hi = Infinity;
	for (let x = x0; ; x += step) {
		const xx = Math.min(x, x1);
		const u = xx + off;
		const sink = smoother(s.end, s.end - taper, u);
		const y = yb - s.ridge(u) * sink;
		hi = Math.min(hi, y);
		top.push([xx, y]);
		bottom.push([xx, yb + (yf - yb) * smoother(s.end, s.end - taper * 0.45, u)]);
		if (xx >= x1) break;
	}
	const path = new Path2D();
	path.moveTo(top[0][0], top[0][1]);
	for (const [x, y] of top) path.lineTo(x, y);
	for (let i = bottom.length - 1; i >= 0; i--) path.lineTo(bottom[i][0], bottom[i][1]);
	path.closePath();

	const n = night(t);
	const gone = Math.max(dim(t), n * 0.96);
	const tone = fade(s.tone, gone);
	const mist = fade(s.mist, gone);
	// solid at the crests, softening into the mist that lies in the valley in front
	const lo = Math.max(yb + (yf - yb) * 0.35, hi + 1);
	const g = ctx.createLinearGradient(0, hi, 0, lo);
	gradientStops(g, (k) => mix(tone, mist, smoother(0.2, 1, k)), 12);
	ctx.fillStyle = g;
	ctx.fill(path);
	// the low sun reaches across the far dunes: a wide, low glow around it (drawn in a space
	// stretched sideways), gone as it sets
	const up = smooth(-SUN_R, 120, sunAlt(t)) * (1 - n);
	const sunUp = up * s.glow;
	if (sunUp > 0.002) {
		const sy = cam.hz - Math.max(0, sunAlt(t));
		ctx.save();
		ctx.clip(path);
		ctx.translate(SUN_X, sy);
		ctx.scale(2.4, 1);
		const glow = ctx.createRadialGradient(0, 0, 0, 0, 0, 760);
		const lit = mix(s.mist, SUNLIT, 0.45);
		gradientStops(
			glow,
			() => lit,
			16,
			(k) => 0.75 * sunUp * (1 - smoother(0, 1, k)) ** 1.4
		);
		ctx.fillStyle = glow;
		ctx.fillRect(-W, -H * 2, W * 2, H * 4);
		ctx.restore();
		// grazing light down the flanks that face the sun: soft slivers under the ridge, deepest
		// where the slope turns toward the sun and the sun is near
		const band = 26 + 110 * Math.sqrt(s.p);
		const lit2 = mix(s.mist, SUNLIT, 0.7);
		const L = top.map(([x], i) => {
			const a = top[Math.max(0, i - 2)];
			const b = top[Math.min(top.length - 1, i + 2)];
			const slope = (b[1] - a[1]) / Math.max(1e-6, b[0] - a[0]);
			// the flank that faces the sun swaps sides under it: blended, so there is no seam
			const facing = slope * Math.tanh((SUN_X - x) / 160);
			return smooth(0.02, 0.55, facing) * Math.exp(-(((x - SUN_X) / 1000) ** 2));
		});
		const passes = 6;
		ctx.save();
		// thin spits of sand are shallower than the band: keep the light on the land
		ctx.clip(path);
		ctx.fillStyle = css(lit2, (1.1 / passes) * sunUp);
		for (let k = 1; k <= passes; k++) {
			const d = (band * k) / passes;
			ctx.beginPath();
			ctx.moveTo(top[0][0], top[0][1]);
			for (const [x, y] of top) ctx.lineTo(x, y);
			for (let i = top.length - 1; i >= 0; i--) ctx.lineTo(top[i][0], top[i][1] + d * L[i]);
			ctx.closePath();
			ctx.fill();
		}
		ctx.restore();
		// and the crest itself catches a hairline of light
		const rim = ctx.createLinearGradient(SUN_X - 900, 0, SUN_X + 900, 0);
		gradientStops(
			rim,
			() => mix(SUNLIT, PAL.warm, 0.25),
			16,
			(k) => 0.55 * sunUp * Math.exp(-(((k - 0.5) * 3.4) ** 2))
		);
		ctx.strokeStyle = rim;
		ctx.lineWidth = 1.3;
		ctx.lineJoin = 'round';
		ctx.beginPath();
		top.forEach(([x, y], i) => (i ? ctx.lineTo(x, y) : ctx.moveTo(x, y)));
		ctx.stroke();
	}
}

// --- the city --------------------------------------------------------------------------------

interface Win {
	x: number;
	y: number;
	w: number;
	h: number;
	warm: boolean;
	b: number;
	on: number;
	off: number;
}

interface Tower {
	u: number;
	w: number;
	h: number;
	r: number;
	spire: number;
	windows: Win[];
	beacon: boolean;
}

interface CityLayer {
	p: number;
	dusk: RGB;
	night: RGB;
	towers: Tower[];
	/** Window size. */
	win: number;
}

/**
 * One layer of the far shore's skyline: towers in the logo's language (rounded tops, a few
 * domes and spires), their windows lit in a few deliberate patterns rather than at random.
 */
function city(
	seed: number,
	p: number,
	from: number,
	to: number,
	[hLo, hHi]: [number, number],
	[wLo, wHi]: [number, number],
	win: number,
	dusk: string,
	nightTone: string,
	density: number,
	gap = 0.5
): CityLayer {
	const r = rng(seed);
	const towers: Tower[] = [];
	let u = from;
	while (u < to) {
		const w = wLo + (wHi - wLo) * r();
		// the skyline climbs toward its middle
		const k = Math.sin(Math.min(1, (u - from) / (to - from)) * Math.PI) ** 0.8;
		const h = (hLo + (hHi - hLo) * (0.15 + 0.85 * r() * k)) * (r() < 0.12 ? 1.3 : 1);
		const r0 = r();
		const radius = r0 < 0.3 ? w / 2 : r0 < 0.55 ? Math.min(w * 0.22, 18) : Math.min(3, w * 0.1);
		const spire = r() < 0.14 ? h * (0.1 + r() * 0.1) : 0;
		const windows: Win[] = [];
		const pitchX = win * 2.8;
		const pitchY = win * 3.1;
		const cols = Math.max(1, Math.floor((w - win * 2.6) / pitchX) + 1);
		const rows = Math.floor((h - radius - win * 4) / pitchY);
		const x0 = (w - (cols - 1) * pitchX - win) / 2;
		// each tower is lit its own way: a sparse scatter, a lit spine, bands of lit floors, a
		// lit crown, or dark
		const style = r();
		const band = 3 + Math.floor(r() * 3);
		const spineA = Math.floor(cols / 2);
		const spineB = cols > 4 ? Math.floor(cols / 4) : -1;
		const lit = (i: number, j: number) => {
			if (style < 0.36) return r() < density * (0.5 + r());
			if (style < 0.52) return (j === spineA || j === spineB) && r() < 0.85;
			if (style < 0.68) return i % band === 1 && r() < 0.9;
			if (style < 0.86) return i < 3 && r() < 0.8;
			return false;
		};
		for (let i = 0; i < rows; i++)
			for (let j = 0; j < cols; j++)
				if (lit(i, j))
					windows.push({
						x: x0 + j * pitchX,
						y: radius + win * 2.6 + i * pitchY,
						w: win,
						h: win * 1.25,
						warm: style < 0.68 ? r() < 0.82 : r() < 0.55,
						b: 0.5 + r() * 0.5,
						on: r(),
						off: r()
					});
		towers.push({ u, w, h, r: radius, spire, windows, beacon: h > hHi * 0.82 && w > wLo * 1.2 });
		u += w + (r() < 0.22 ? win * 2 + r() * win * 7 : win * gap);
	}
	return { p, dusk: hex(dusk), night: hex(nightTone), towers, win };
}

/** The city's left edge reaches screen x at time t in every layer, so it arrives as one. */
const cityAt = (p: number, x: number, t: number) => x + truck(t) * p;

const CITY: CityLayer[] = [
	city(
		41,
		0.035,
		cityAt(0.035, 1010, 3.2),
		cityAt(0.035, 2600, 3.2),
		[8, 34],
		[10, 26],
		1.25,
		'#6c050b',
		'#150306',
		0.12,
		0.2
	),
	city(
		43,
		0.08,
		cityAt(0.08, 1080, 3.2),
		cityAt(0.08, 2700, 3.2),
		[26, 110],
		[16, 40],
		1.6,
		'#55040a',
		'#100204',
		0.08,
		0.3
	),
	city(
		47,
		0.15,
		cityAt(0.15, 1180, 3.2),
		cityAt(0.15, 2900, 3.2),
		[70, 250],
		[26, 62],
		2.1,
		'#3e0308',
		'#0c0203',
		0.06
	),
	city(
		53,
		0.26,
		cityAt(0.26, 1320, 3.2),
		cityAt(0.26, 3100, 3.2),
		[180, 430],
		[44, 92],
		2.7,
		'#290206',
		'#080203',
		0.05
	)
];

const WINDOW_WARM = mix(PAL.warm, hex('#ffd7b8'), 0.4);
/** Lights seen in dark water take on its red. */
const REFLECT_WARM = mix(WINDOW_WARM, PAL.red, 0.35);

/** The lights come on through the dusk, a few at a time, and go out with everything else. */
function windowLit(t: number, w: Win, p: number) {
	const a = 2.45 + w.on * 1.5;
	// the near towers go dark first, as they sink out of the frame
	const b = 4.38 + w.off * 0.42 + (0.3 - p) * 0.5;
	return smoother(a, a + 0.28, t) * (1 - smoother(b, b + 0.25, t));
}

function towerPath(path: Path2D, x: number, base: number, tw: Tower) {
	const top = base - tw.h;
	const r = Math.min(tw.r, tw.w / 2);
	path.moveTo(x, base + 4);
	path.lineTo(x, top + r);
	if (r >= tw.w / 2 - 0.01) path.arc(x + r, top + r, r, Math.PI, 0);
	else {
		path.arcTo(x, top, x + r, top, r);
		path.lineTo(x + tw.w - r, top);
		path.arcTo(x + tw.w, top, x + tw.w, top + r, r);
	}
	path.lineTo(x + tw.w, base + 4);
	path.closePath();
	if (tw.spire > 0) {
		const sw = Math.max(1.6, tw.w * 0.05);
		path.rect(x + tw.w / 2 - sw / 2, top - tw.spire, sw, tw.spire + r + 1);
	}
}

/** The skyline is composed here first while it condenses out of the haze, then faded in whole. */
let haze: CanvasRenderingContext2D | null = null;

function drawCity(ctx: C2D, cam: Camera, t: number) {
	// the towers condense out of the dusk haze as the light goes; composed opaque first, so
	// overlapping towers never show through each other
	const e = smoother(1.0, 2.9, t);
	if (e <= 0.001) return;
	if (e >= 0.999) {
		drawSkyline(ctx, cam, t);
		return;
	}
	if (!haze) {
		const c = document.createElement('canvas');
		c.width = W;
		c.height = H;
		haze = c.getContext('2d') as CanvasRenderingContext2D;
	}
	haze.setTransform(ctx.getTransform());
	haze.save();
	haze.setTransform(1, 0, 0, 1, 0, 0);
	haze.clearRect(0, 0, W, H);
	haze.restore();
	drawSkyline(haze, cam, t);
	ctx.save();
	ctx.setTransform(1, 0, 0, 1, 0, 0);
	ctx.globalAlpha = e;
	ctx.drawImage(haze.canvas, 0, 0);
	ctx.restore();
}

function drawSkyline(ctx: C2D, cam: Camera, t: number) {
	const n = night(t);
	const out = dim(t);
	for (const L of CITY) {
		const off = cam.x * L.p;
		const base = cam.hz + cam.h * L.p;
		if (base - 700 > H) continue;
		const path = new Path2D();
		let any = false;
		for (const tw of L.towers) {
			const x = tw.u - off;
			if (x > W + 10 || x + tw.w < -10) continue;
			towerPath(path, x, base, tw);
			any = true;
		}
		if (!any) continue;
		ctx.fillStyle = css(fade(mix(L.dusk, L.night, n), out));
		ctx.fill(path);
		for (const tw of L.towers) {
			const x = tw.u - off;
			if (x > W + 10 || x + tw.w < -10) continue;
			for (const w of tw.windows) {
				const lit = windowLit(t, w, L.p);
				if (lit <= 0.002) continue;
				const y = base - tw.h + w.y;
				if (y > H || y < -10) continue;
				ctx.fillStyle = css(w.warm ? WINDOW_WARM : PAL.red, lit * w.b);
				ctx.fillRect(x + w.x, y, w.w, w.h);
			}
			if (tw.beacon) {
				const pulse = 0.55 + 0.45 * Math.sin(t * 1.5 * Math.PI + tw.u * 0.013);
				const on = smooth(2.9, 3.5, t) * (1 - smoother(4.45, 4.8, t));
				if (on > 0.002) {
					ctx.fillStyle = css(PAL.red, on * pulse);
					ctx.beginPath();
					ctx.arc(x + tw.w / 2, base - tw.h - tw.spire - L.win * 1.4, L.win * 0.85, 0, Math.PI * 2);
					ctx.fill();
				}
			}
		}
	}
}

/**
 * The city's lights on the water: each window mirrored below the shore, broken by the swell
 * into short dashes that shimmer (the reflection lies flatter than the city: 0.8 of it).
 */
function drawCityReflections(ctx: C2D, cam: Camera, t: number) {
	if (night(t) <= 0.01) return;
	ctx.lineCap = 'round';
	for (const L of CITY) {
		const off = cam.x * L.p;
		const base = cam.hz + cam.h * L.p;
		if (base > H) continue;
		for (const tw of L.towers) {
			const x = tw.u - off;
			if (x > W + 10 || x + tw.w < -10) continue;
			for (const w of tw.windows) {
				// only the brighter lights carry across the water
				if (w.b < 0.62) continue;
				const lit = windowLit(t, w, L.p) * w.b;
				if (lit <= 0.01) continue;
				const above = tw.h - w.y;
				const y = base + 2 + above * 0.66;
				if (y > H + 2) continue;
				const s = 0.5 + 0.5 * Math.sin(t * 2.4 + w.x * 0.9 + w.y * 0.37 + tw.u * 0.05);
				// fainter the deeper the reflection reaches
				const fall = Math.exp(-above / (180 * L.p + 50));
				const a = lit * 0.65 * fall * (0.3 + 0.7 * s);
				if (a < 0.01) continue;
				ctx.strokeStyle = css(w.warm ? REFLECT_WARM : PAL.red, a);
				ctx.lineWidth = Math.max(0.8, L.win * 0.42);
				const len = L.win * (2 + 4 * s);
				const cx = x + w.x + w.w / 2 + Math.sin(t * 1.7 + w.y) * L.win * 0.8;
				stroke(ctx, cx, y, len);
			}
		}
	}
}

// --- the couch and the popcorn ---------------------------------------------------------------

let paths: { couch: Path2D; popcorn: Path2D } | null = null;

/** The glyphs as paths, made on first use (the module also loads where there is no canvas). */
function glyphs() {
	if (!paths) paths = { couch: new Path2D(COUCH), popcorn: new Path2D(POPCORN) };
	return paths;
}

/** The couch's visible box centre, logo units. */
const CX = 523;
const CY = 456.5;

function drawCouch(ctx: C2D, t: number, dy: number) {
	const pose = couchPose(t);
	ctx.save();
	ctx.translate(pose.x, pose.y + dy);
	ctx.rotate((pose.rot * Math.PI) / 180);
	ctx.scale(pose.k, pose.k);
	ctx.translate(-CX, -CY);
	ctx.fillStyle = css(PAL.red);
	ctx.fill(glyphs().couch);
	ctx.restore();
}

/** The popcorn: dropped from above the frame, it falls with gravity and rebounds once. */
export function popcornOffset(t: number): { dy: number; rot: number } | null {
	if (t < DROP) return null;
	const fall = (t - DROP) / (LAND - DROP);
	if (fall < 1) {
		const p = fall * fall;
		return { dy: -(1 - p) * 760, rot: 14 * (1 - p) };
	}
	// one small rebound (a short parabola), then still
	const q = (t - LAND) / 0.3;
	if (q < 1)
		return {
			dy: -26 * 4 * q * (1 - q) * (1 - q * 0.35),
			rot: -2.5 * Math.sin(q * Math.PI) * (1 - q)
		};
	return { dy: 0, rot: 0 };
}

function drawPopcorn(ctx: C2D, t: number) {
	const o = popcornOffset(t);
	if (!o) return;
	const k = LOCK.w / 1046;
	ctx.save();
	ctx.translate(LOCK.x, LOCK.y + o.dy);
	ctx.scale(k, k);
	ctx.translate(500, 600);
	ctx.rotate((o.rot * Math.PI) / 180);
	ctx.translate(-500, -600);
	// the knockout: a halo in the field's black that cuts the couch where they meet
	ctx.lineJoin = 'round';
	ctx.lineWidth = GAP * 2;
	ctx.strokeStyle = css(PAL.black);
	ctx.fillStyle = css(PAL.black);
	const { popcorn } = glyphs();
	ctx.stroke(popcorn);
	ctx.fill(popcorn);
	ctx.fillStyle = css(PAL.red);
	ctx.fill(popcorn);
	ctx.restore();
}

/** The landing presses the frame down 7 px and lets it spring back (damped, about 0.4 s). */
export function frameGive(t: number): number {
	const s = t - LAND;
	if (s <= 0 || s >= GIVE) return 0;
	// e^-6.5s sin 14s peaks at 0.535 (s = 0.081): scaled so that first dip is 7 px. The window
	// takes the last hundredths of a pixel away smoothly: a canvas's anti-aliasing steps at
	// quarter pixels, so even 0.02 px of drift would flicker the logo's edges
	return (7 / 0.535) * Math.exp(-6.5 * s) * Math.sin(14 * s) * (1 - smoother(0.32, GIVE, s));
}

/** How long the landing's give lasts; nothing in the canvas moves after LAND + GIVE. */
export const GIVE = 0.5;

/** The couch is pressed down too, a hair behind the frame. */
function couchGive(t: number): number {
	const s = t - LAND;
	if (s < 0 || s > 0.5) return 0;
	return 6 * Math.sin(Math.min(1, s / 0.22) * Math.PI) * (1 - s / 0.5);
}

// --- the frame -------------------------------------------------------------------------------

/** Everything at time t. */
export function drawWorld(ctx: C2D, t: number) {
	const cam = camera(t);
	ctx.save();
	ctx.fillStyle = css(PAL.black);
	ctx.fillRect(0, 0, W, H);
	ctx.translate(0, frameGive(t));
	const world = t < ALONE + 0.3;
	if (world) {
		drawSky(ctx, cam, t);
		drawStars(ctx, cam, t);
		drawSun(ctx, cam, t);
		drawSea(ctx, cam, t);
		drawCity(ctx, cam, t);
		for (const s of STRIPS) drawStrip(ctx, cam, t, s);
	}
	drawCouch(ctx, t, couchGive(t));
	if (world) for (const s of FRONT) drawStrip(ctx, cam, t, s);
	drawPopcorn(ctx, t);
	ctx.restore();
}
