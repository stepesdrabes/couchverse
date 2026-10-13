// Scene B and C: a building at the edge of town at night, one window lit, and the room behind
// it. The room is real geometry in B's world (the camera flies into it): a back wall, floor,
// ceiling and side walls, a rug, the couch, a floor lamp, and a television out of sight to the
// left whose light falls across the couch and throws its shadow up the wall.

import {
	type Cam,
	F,
	layer,
	linear,
	project,
	radial,
	type RGB,
	rgba,
	smooth,
	type Stop
} from './art';
import { REF_B, ROOM } from './camera';
import { couchAt, drawCouch, glyphs, LOGO_H, LOGO_W } from './couch';
import { core, falloff, glow, lit, linearOf } from './light';
import { RED, T } from './time';

type V3 = [number, number, number];

const R = ROOM;
/** The television: low, left of the couch and in front of it, facing it. */
const TV: V3 = [-240, 92, 760];
const NEAR = 6;

const PLASTER = linearOf('#f6eee8');
const WOOD = linearOf('#2a0f0b');
const RUG = linearOf('#7a1a1c');
const METAL = linearOf('#2a0a08');

/** The camera's offsets from B's reference. */
function rel(cam: Cam) {
	return {
		dx: cam.x - REF_B.x,
		dy: cam.y - REF_B.y,
		dz: cam.z - REF_B.z,
		dpan: cam.pan - REF_B.pan,
		dtilt: cam.tilt - REF_B.tilt
	};
}

/** A polygon in B's world on the screen, clipped to the near plane; null when behind us. */
function poly(cam: Cam, pts: V3[]): Path2D | null {
	const zn = cam.z - REF_B.z + NEAR;
	const out: V3[] = [];
	for (let i = 0; i < pts.length; i++) {
		const a = pts[i];
		const b = pts[(i + 1) % pts.length];
		const ina = a[2] >= zn;
		const inb = b[2] >= zn;
		if (ina) out.push(a);
		if (ina !== inb) {
			const k = (zn - a[2]) / (b[2] - a[2]);
			out.push([a[0] + (b[0] - a[0]) * k, a[1] + (b[1] - a[1]) * k, zn]);
		}
	}
	if (out.length < 3) return null;
	const p = new Path2D();
	out.forEach((v, i) => {
		const [x, y] = project(cam, REF_B, v[0], v[1], v[2]);
		if (i) p.lineTo(x, y);
		else p.moveTo(x, y);
	});
	p.closePath();
	return p;
}

const pt = (cam: Cam, x: number, y: number, z: number) => project(cam, REF_B, x, y, z);
/** Screen pixels per world unit at depth Z. */
const scaleAt = (cam: Cam, z: number) => F / Math.max(1e-3, z - (cam.z - REF_B.z));

/** A flat card at depth z, drawn in world units (x, y) about the world origin. */
function card(cam: Cam, z: number, x0 = 0, y0 = 0): DOMMatrix {
	const r = rel(cam);
	const k = scaleAt(cam, z);
	return new DOMMatrix([
		k,
		0,
		0,
		k,
		960 + (x0 - r.dx) * k - r.dpan,
		540 + (y0 - r.dy) * k - r.dtilt
	]);
}

/** The shadow, on the plane z = `wall`, of a card at depth `z0` drawn in world units. */
function shadowCard(cam: Cam, z0: number, wall: number): DOMMatrix {
	const s = (wall - TV[2]) / (z0 - TV[2]);
	const r = rel(cam);
	const k = F / Math.max(1e-3, wall - r.dz);
	// world (x, y) on the card -> TV + ((x, y) - TV) * s on the wall
	const ox = TV[0] * (1 - s);
	const oy = TV[1] * (1 - s);
	return new DOMMatrix([
		s * k,
		0,
		0,
		s * k,
		960 + (ox - r.dx) * k - r.dpan,
		540 + (oy - r.dy) * k - r.dtilt
	]);
}

// --- the couch in the room -----------------------------------------------------------------

const FEET = 540 + (F * R.floor) / R.couchZ;
/** World units per logo unit on the couch. */
const WSC = (R.couchPx * R.couchZ) / F / LOGO_W;
/** The couch in world units on its card (logo units -> world x, y). */
const couchWorld = () =>
	new DOMMatrix([WSC, 0, 0, WSC, -(LOGO_W / 2) * WSC, R.floor - LOGO_H * WSC]);

/** The couch's matrix on the screen (it is a card at the couch's depth). */
export function couchMatrix(cam: Cam): DOMMatrix | null {
	const m = layer(cam, REF_B, R.couchZ);
	return m ? m.multiply(couchAt(960, FEET, R.couchPx)) : null;
}

// --- props -----------------------------------------------------------------------------------

const LAMP = { x: 158, z: 1046 };

let lampPath: Path2D | null = null;

/** A floor lamp in world units about its foot: a round base, a thin pole, a tapered drum shade. */
function lamp(): Path2D {
	if (lampPath) return lampPath;
	const p = new Path2D();
	const foot = R.floor;
	p.ellipse(0, foot - 2.2, 16, 3.6, 0, 0, Math.PI * 2);
	p.rect(-1.5, -30, 3, foot + 28);
	p.moveTo(-19, -70);
	p.lineTo(19, -70);
	p.lineTo(24, -30);
	p.lineTo(-24, -30);
	p.closePath();
	lampPath = p;
	return p;
}

// --- light -----------------------------------------------------------------------------------

/** How lit the room is (1) as it falls dark around the couch (0). */
export const roomLight = (t: number) => 1 - smooth(T.darkFrom, T.darkTo, t);

function levels(t: number) {
	const l = roomLight(t);
	// the television's light goes first, the room's red glow follows it down
	return { key: 1.45 * l * l, amb: 1.05 * l ** 0.8, warm: 1 - l * l };
}

/** A surface lit by an elliptical pool of the key (screen space), over the ambient. */
function pooled(
	ctx: CanvasRenderingContext2D,
	path: Path2D,
	albedo: RGB,
	cx: number,
	cy: number,
	rx: number,
	ry: number,
	key: number,
	amb: number,
	warm: number,
	keyProfile = core,
	ambProfile = glow
) {
	ctx.save();
	ctx.fillStyle = rgba(lit(albedo, 0, 0));
	ctx.fill(path);
	ctx.clip(path);
	ctx.translate(cx, cy);
	ctx.scale(1, ry / rx);
	ctx.fillStyle = radial(
		ctx,
		0,
		0,
		rx,
		falloff(albedo, key, amb, keyProfile, ambProfile, warm) as Stop[]
	);
	ctx.fillRect(-rx, -rx, rx * 2, rx * 2);
	ctx.restore();
}

export function drawRoom(ctx: CanvasRenderingContext2D, cam: Cam, t: number) {
	const dz = cam.z - REF_B.z;
	const { key, amb, warm } = levels(t);
	const q = (pts: V3[]) => poly(cam, pts);
	const back = q([
		[R.x0, R.ceil, R.back],
		[R.x1, R.ceil, R.back],
		[R.x1, R.floor, R.back],
		[R.x0, R.floor, R.back]
	]);
	const floor = q([
		[R.x0, R.floor, R.wallIn],
		[R.x1, R.floor, R.wallIn],
		[R.x1, R.floor, R.back],
		[R.x0, R.floor, R.back]
	]);
	const ceil = q([
		[R.x0, R.ceil, R.wallIn],
		[R.x1, R.ceil, R.wallIn],
		[R.x1, R.ceil, R.back],
		[R.x0, R.ceil, R.back]
	]);
	const left = q([
		[R.x0, R.ceil, R.wallIn],
		[R.x0, R.ceil, R.back],
		[R.x0, R.floor, R.back],
		[R.x0, R.floor, R.wallIn]
	]);
	const right = q([
		[R.x1, R.ceil, R.wallIn],
		[R.x1, R.ceil, R.back],
		[R.x1, R.floor, R.back],
		[R.x1, R.floor, R.wallIn]
	]);
	ctx.save();
	ctx.setTransform(1, 0, 0, 1, 0, 0);
	const kb = scaleAt(cam, R.back);
	const kc = scaleAt(cam, R.couchZ);
	if (back) {
		// the television's light across the wall, strongest left of the couch
		const [cx, cy] = pt(cam, -55, 52, R.back);
		const rx = 390 * kb;
		const ry = 250 * kb;
		pooled(ctx, back, PLASTER, cx, cy, rx, ry, key, amb, warm);
		// shadows: the same wall with the key taken away and less of the bounce
		const dark = falloff(PLASTER, 0, amb * 0.42) as Stop[];
		const pool = new DOMMatrix().translate(cx, cy).scale(1, ry / rx);
		const inv = pool.inverse();
		ctx.save();
		ctx.clip(back);
		ctx.setTransform(pool);
		ctx.fillStyle = radial(ctx, 0, 0, rx, dark);
		const lampShadow = new Path2D();
		lampShadow.addPath(lamp(), inv.multiply(shadowCard(cam, LAMP.z, R.back).translate(LAMP.x, 0)));
		ctx.filter = `blur(${(6 * kb).toFixed(2)}px)`;
		ctx.globalAlpha = 0.55;
		ctx.fill(lampShadow);
		const couchShadow = new Path2D();
		couchShadow.addPath(
			glyphs().couch,
			inv.multiply(shadowCard(cam, R.couchZ, R.back)).multiply(couchWorld())
		);
		ctx.filter = `blur(${(8 * kb).toFixed(2)}px)`;
		ctx.globalAlpha = 1;
		ctx.fill(couchShadow);
		ctx.restore();
	}
	const side = (path: Path2D | null, x: number, near: RGB, far: RGB) => {
		if (!path) return;
		const [a, b] = pt(cam, x, -30, R.back);
		const [c] = pt(cam, x, -30, Math.max(R.wallIn, dz + 40));
		ctx.fillStyle = linear(ctx, a, b, c, b, [
			[0, far],
			[1, near]
		]);
		ctx.fill(path);
	};
	side(left, R.x0, lit(PLASTER, 0, amb * 0.01), lit(PLASTER, key * 0.002, amb * 0.07, warm));
	side(right, R.x1, lit(PLASTER, 0, amb * 0.01), lit(PLASTER, key * 0.004, amb * 0.085, warm));
	if (ceil) {
		const [a, b] = pt(cam, 0, R.ceil, R.back);
		const [, c] = pt(cam, 0, R.ceil, Math.max(R.wallIn, dz + 40));
		ctx.fillStyle = linear(ctx, a, b, a, c, [
			[0, lit(PLASTER, 0, amb * 0.05)],
			[1, lit(PLASTER, 0, amb * 0.006)]
		]);
		ctx.fill(ceil);
	}
	if (floor) {
		const [fx, fy] = pt(cam, -50, R.floor, 940);
		pooled(ctx, floor, WOOD, fx, fy, 560 * kc, 130 * kc, key * 1.6, amb * 1.4, warm);
		const rug = q([
			[-150, R.floor, 905],
			[150, R.floor, 905],
			[150, R.floor, 1062],
			[-150, R.floor, 1062]
		]);
		if (rug) pooled(ctx, rug, RUG, fx, fy, 560 * kc, 130 * kc, key * 0.5, amb * 0.9, warm);
		// its woven border, a shade darker
		const field = q([
			[-138, R.floor, 913],
			[138, R.floor, 913],
			[138, R.floor, 1054],
			[-138, R.floor, 1054]
		]);
		if (rug && field) {
			const ring = new Path2D();
			ring.addPath(rug);
			ring.addPath(field);
			ctx.save();
			ctx.fillStyle = rgba(lit(RUG, 0, amb * 0.15), 0.55);
			ctx.fill(ring, 'evenodd');
			ctx.restore();
		}
		// the couch's contact shadow on the rug
		ctx.save();
		ctx.clip(floor);
		const [sx, sy] = pt(cam, 16, R.floor, R.couchZ + 4);
		ctx.translate(sx, sy);
		ctx.scale(1, 0.16);
		ctx.filter = `blur(${(4 * kc).toFixed(2)}px)`;
		ctx.fillStyle = rgba(lit(RUG, 0, amb * 0.12), 0.85);
		ctx.beginPath();
		ctx.ellipse(0, 0, 74 * kc, 46 * kc, 0, 0, Math.PI * 2);
		ctx.fill();
		ctx.restore();
	}
	// soft occlusion where the back wall meets the floor, ceiling and side walls (all at one
	// depth, so one width fits the whole edge)
	const crease = (a: V3, b: V3, width: number, alpha: number) => {
		if (R.back - dz < NEAR * 4) return;
		const p = pt(cam, a[0], a[1], a[2]);
		const q2 = pt(cam, b[0], b[1], b[2]);
		ctx.save();
		ctx.filter = `blur(${(width * 0.5 * kb).toFixed(2)}px)`;
		ctx.strokeStyle = `rgba(4, 0, 1, ${alpha})`;
		ctx.lineWidth = width * kb;
		ctx.lineCap = 'round';
		ctx.beginPath();
		ctx.moveTo(p[0], p[1]);
		ctx.lineTo(q2[0], q2[1]);
		ctx.stroke();
		ctx.restore();
	};
	crease([R.x0, R.floor, R.back], [R.x1, R.floor, R.back], 8, 0.5);
	crease([R.x0, R.ceil, R.back], [R.x1, R.ceil, R.back], 10, 0.45);
	crease([R.x0, R.ceil, R.back], [R.x0, R.floor, R.back], 10, 0.4);
	crease([R.x1, R.ceil, R.back], [R.x1, R.floor, R.back], 10, 0.4);
	// the lamp, its side toward the television catching the light
	if (LAMP.z - dz > NEAR) {
		const m = card(cam, LAMP.z, LAMP.x);
		ctx.setTransform(m);
		ctx.fillStyle = linear(ctx, -24, 0, 24, 0, [
			[0, lit(METAL, key * 0.6, amb * 0.5, warm)],
			[1, lit(METAL, 0, amb * 0.15)]
		]);
		ctx.fill(lamp());
	}
	ctx.restore();
}

/** The couch lit by the television from the left, flattening to the logo's red as it goes dark. */
export function drawRoomCouch(ctx: CanvasRenderingContext2D, cam: Cam, t: number, m: DOMMatrix) {
	const flat = 1 - roomLight(t);
	drawCouch(ctx, m, {
		top: mixHex('#f0121a', RED, flat),
		bottom: mixHex('#96060e', RED, flat),
		pool: { x: 260, y: 330, r: 780, color: '#ff4a3a', alpha: 0.4 * (1 - flat) },
		rim: { color: '#ffc0b0', width: 4, dx: -0.95, dy: -0.3, soft: 3, alpha: 0.22 * (1 - flat) }
	});
	void cam;
}

function mixHex(a: string, b: string, k: number): RGB {
	const x = parseInt(a.slice(1), 16);
	const y = parseInt(b.slice(1), 16);
	return [16, 8, 0].map((s) => {
		const u = (x >> s) & 255;
		const v = (y >> s) & 255;
		return u + (v - u) * k;
	}) as RGB;
}

// --- the town at night (outside) -----------------------------------------------------------

/** The façade in screen pixels from B's reference camera. */
const WIN = {
	x0: 960 + (F * R.win.x0) / R.wallZ,
	x1: 960 + (F * R.win.x1) / R.wallZ,
	y0: 540 + (F * R.win.y0) / R.wallZ,
	y1: 540 + (F * R.win.y1) / R.wallZ
};
const WW = WIN.x1 - WIN.x0;
const WH = WIN.y1 - WIN.y0;
const CORNER = 500;
/** The roofline, a parapet's height above the lit window (it is on the top floor). */
const ROOF = 180;
const PITCH = { x: WW + 250, y: WH + 190 };

export function drawMoon(ctx: CanvasRenderingContext2D, cam: Cam) {
	const x = 330 - (cam.pan - REF_B.pan);
	const y = 150 - (cam.tilt - REF_B.tilt);
	ctx.save();
	ctx.setTransform(1, 0, 0, 1, 0, 0);
	ctx.fillStyle = radial(ctx, x, y, 220, [
		[0, '#ffe2d6', 0.2],
		[0.25, '#ff9a86', 0.07],
		[1, '#ff5a4a', 0]
	]);
	ctx.fillRect(x - 230, y - 230, 460, 460);
	ctx.fillStyle = radial(ctx, x, y, 40, [
		[0, '#fff8f4'],
		[0.92, '#fff0e8'],
		[1, '#ffe0d2', 0]
	]);
	ctx.beginPath();
	ctx.arc(x, y, 40, 0, Math.PI * 2);
	ctx.fill();
	// its seas, faint
	ctx.clip();
	for (const [mx, my, mr, a] of [
		[-12, -10, 15, 0.09],
		[9, -14, 9, 0.07],
		[6, 8, 13, 0.08],
		[-16, 14, 7, 0.05]
	]) {
		ctx.fillStyle = radial(ctx, x + mx, y + my, mr, [
			[0, '#c8a49a', a],
			[1, '#c8a49a', 0]
		]);
		ctx.fillRect(x + mx - mr, y + my - mr, mr * 2, mr * 2);
	}
	ctx.restore();
}

let skyline: { path: Path2D; lights: [number, number][] } | null = null;

function makeSkyline() {
	const p = new Path2D();
	const lights: [number, number][] = [];
	let x = -1100;
	let seed = 11;
	const rnd = () => {
		seed = (seed * 16807) % 2147483647;
		return seed / 2147483647;
	};
	p.moveTo(x, 1600);
	p.lineTo(x, 900);
	while (x < 1200) {
		const w = 36 + rnd() * 80;
		// the town rises out of the hills toward the building: taller blocks nearer it
		const near = Math.max(0, Math.min(1, (x + 600) / 1400));
		const from = Math.max(0, Math.min(1, (x + 1100) / 600));
		const h = (24 + rnd() * rnd() * (90 + 130 * near)) * from;
		const top = 812 - h;
		p.lineTo(x, top);
		p.lineTo(x + w, top);
		if (rnd() < 0.55)
			for (let i = 0; i < 1 + Math.floor(rnd() * 3); i++)
				lights.push([x + 6 + rnd() * (w - 12), top + 8 + rnd() * Math.max(4, h - 14)]);
		x += w;
	}
	p.lineTo(x, 1600);
	p.lineTo(-1100, 1600);
	p.closePath();
	return { path: p, lights };
}

export function drawSkyline(ctx: CanvasRenderingContext2D, cam: Cam) {
	const m = layer(cam, REF_B, 6000);
	if (!m) return;
	skyline ??= makeSkyline();
	ctx.save();
	ctx.setTransform(m);
	ctx.fillStyle = linear(ctx, 0, 640, 0, 860, [
		[0, '#0b0102'],
		[1, '#150203']
	]);
	ctx.fill(skyline.path);
	ctx.fillStyle = 'rgba(255, 206, 186, 0.62)';
	for (const [x, y] of skyline.lights) ctx.fillRect(x, y, 2.2, 2.8);
	ctx.restore();
}

/** The façade with its windows; the lit one shows the room. */
export function drawFacade(ctx: CanvasRenderingContext2D, cam: Cam, t: number) {
	const m = layer(cam, REF_B, R.wallZ);
	if (!m) return;
	// the room first, seen through the opening
	const hole = new Path2D();
	hole.rect(WIN.x0, WIN.y0, WW, WH);
	ctx.save();
	ctx.setTransform(m);
	ctx.clip(hole);
	ctx.setTransform(1, 0, 0, 1, 0, 0);
	drawRoom(ctx, cam, t);
	const cm = couchMatrix(cam);
	if (cm) drawRoomCouch(ctx, cam, t, cm);
	ctx.restore();
	drawReveal(ctx, cam, t);
	// the wall, with the opening cut out
	const wall = new Path2D();
	wall.rect(CORNER, ROOF, 5000, 3000);
	wall.rect(WIN.x0, WIN.y0, WW, WH);
	ctx.save();
	ctx.setTransform(m);
	ctx.fillStyle = linear(ctx, 0, ROOF, 0, 1300, [
		[0, '#1e0204'],
		[0.45, '#120203'],
		[1, '#070101']
	]);
	ctx.fill(wall, 'evenodd');
	ctx.save();
	ctx.clip(wall, 'evenodd');
	// the moon's light across the wall from the upper left, the town's red glow from below
	ctx.fillStyle = linear(ctx, CORNER, ROOF, CORNER + 1300, ROOF + 900, [
		[0, '#3a050a', 0.55],
		[0.5, '#1a0204', 0.15],
		[1, '#000000', 0]
	]);
	ctx.fillRect(CORNER, ROOF, 4000, 2400);
	ctx.fillStyle = linear(ctx, 0, 1300, 0, 640, [
		[0, '#5a060c', 0.55],
		[1, '#5a060c', 0]
	]);
	ctx.fillRect(CORNER, 600, 4000, 900);
	ctx.restore();
	// the parapet's coping, its top edge catching the moon
	ctx.fillStyle = '#240306';
	ctx.fillRect(CORNER - 8, ROOF - 14, 5000, 14);
	ctx.fillStyle = 'rgba(150, 30, 36, 0.8)';
	ctx.fillRect(CORNER - 8, ROOF - 14, 5000, 2);
	ctx.save();
	ctx.clip(wall, 'evenodd');
	// floor slabs: a faint ledge under each row of windows
	for (let j = 0; j <= 2; j++) {
		const y = WIN.y1 + j * PITCH.y + 26;
		ctx.fillStyle = linear(ctx, 0, y, 0, y + 14, [
			[0, '#2a0306'],
			[1, '#0c0102']
		]);
		ctx.fillRect(CORNER, y, 5000, 14);
	}
	// pilasters between the windows, their left faces catching the moon
	for (let i = 0; i <= 3; i++) {
		const cx = WIN.x0 - (PITCH.x - WW) / 2 + i * PITCH.x;
		const pw = 86;
		if (cx - pw / 2 < CORNER) continue;
		ctx.fillStyle = linear(ctx, 0, ROOF, 0, 1300, [
			[0, '#260306'],
			[0.5, '#160203'],
			[1, '#090101']
		]);
		ctx.fillRect(cx - pw / 2, ROOF, pw, 1500);
		ctx.fillStyle = linear(ctx, 0, ROOF, 0, 1100, [
			[0, '#8a1218', 0.7],
			[1, '#8a1218', 0.05]
		]);
		ctx.fillRect(cx - pw / 2, ROOF, 2.5, 1500);
		ctx.fillStyle = 'rgba(3, 0, 1, 0.6)';
		ctx.fillRect(cx + pw / 2 - 3, ROOF, 3, 1500);
	}
	// the dark windows around it: the night sky in their glass, the moon on their reveals
	for (let i = -1; i <= 3; i++)
		for (let j = 0; j <= 2; j++) {
			if (i === 0 && j === 0) continue;
			const x = WIN.x0 + i * PITCH.x;
			const y = WIN.y0 + j * PITCH.y;
			if (x < CORNER + 60) continue;
			ctx.fillStyle = linear(ctx, x, y, x + WW * 0.5, y + WH, [
				[0, '#3a050a'],
				[0.45, '#120203'],
				[1, '#050101']
			]);
			ctx.fillRect(x, y, WW, WH);
			// a soft diagonal sheen across the glass
			ctx.fillStyle = linear(ctx, x, y, x + WW, y + WH, [
				[0.3, '#ff6a5a', 0],
				[0.42, '#ff6a5a', 0.05],
				[0.5, '#ff6a5a', 0]
			]);
			ctx.fillRect(x, y, WW, WH);
			ctx.fillStyle = 'rgba(120, 18, 24, 0.55)';
			ctx.fillRect(x + WW - 6, y, 6, WH);
			ctx.fillStyle = 'rgba(150, 26, 30, 0.5)';
			ctx.fillRect(x, y + WH - 4, WW, 4);
			ctx.fillStyle = 'rgba(2, 0, 0, 0.7)';
			ctx.fillRect(x, y, WW, 5);
		}
	// the lit window's light on the wall around it, and in the air
	const cx = WIN.x0 + WW / 2;
	const cy = WIN.y0 + WH / 2;
	ctx.translate(cx, cy);
	ctx.scale(1.3, 1);
	const glow = levels(t).key / 2.6;
	ctx.fillStyle = radial(ctx, 0, 0, 520, [
		[0, '#ff6a50', 0.3 * glow],
		[0.4, '#c0141c', 0.12 * glow],
		[1, '#600408', 0]
	]);
	ctx.fillRect(-600, -600, 1200, 1200);
	ctx.restore();
	// the corner of the building in the moonlight
	ctx.fillStyle = linear(ctx, 0, -300, 0, 1100, [
		[0, '#7a0e14', 0.85],
		[1, '#7a0e14', 0.1]
	]);
	ctx.fillRect(CORNER, ROOF, 3, 1400);
	ctx.restore();
	drawBalcony(ctx, cam, t);
}

/** A balcony slab under the lit window: the room's light spills out across it. */
function drawBalcony(ctx: CanvasRenderingContext2D, cam: Cam, t: number) {
	const { key, amb } = levels(t);
	const y = R.win.y1 + 3;
	const z0 = R.wallZ - 38;
	const z1 = R.wallZ;
	const x0 = R.win.x0 - 18;
	const x1 = R.win.x1 + 18;
	const top = poly(cam, [
		[x0, y, z0],
		[x1, y, z0],
		[x1, y, z1],
		[x0, y, z1]
	]);
	const front = poly(cam, [
		[x0, y, z0],
		[x1, y, z0],
		[x1, y + 6, z0],
		[x0, y + 6, z0]
	]);
	ctx.save();
	ctx.setTransform(1, 0, 0, 1, 0, 0);
	if (front) {
		ctx.fillStyle = '#0c0102';
		ctx.fill(front);
	}
	if (top) {
		const [ax, ay] = pt(cam, 0, y, z1);
		const [, by] = pt(cam, 0, y, z0);
		ctx.fillStyle = linear(ctx, ax, ay, ax, by, [
			[0, lit(PLASTER, key * 0.03, amb * 0.2)],
			[0.45, lit(PLASTER, 0, amb * 0.07)],
			[1, lit(PLASTER, 0, amb * 0.025)]
		]);
		ctx.fill(top);
		// its front edge, catching a little of the light
		const edge = poly(cam, [
			[x0, y - 0.6, z0],
			[x1, y - 0.6, z0],
			[x1, y + 1.2, z0],
			[x0, y + 1.2, z0]
		]);
		if (edge) {
			ctx.fillStyle = rgba(lit(PLASTER, 0, amb * 0.12));
			ctx.fill(edge);
		}
	}
	ctx.restore();
}

/** The window's reveal (the wall's thickness, lit from inside) and the transom. */
function drawReveal(ctx: CanvasRenderingContext2D, cam: Cam, t: number) {
	const w = R.win;
	const a = R.wallZ;
	const b = R.wallIn;
	const { amb } = levels(t);
	const ty = R.win.y0 + (R.win.y1 - R.win.y0) * 0.24;
	const sides: { pts: V3[]; c: RGB }[] = [
		{
			pts: [
				[w.x0, w.y0, a],
				[w.x0, w.y0, b],
				[w.x0, w.y1, b],
				[w.x0, w.y1, a]
			],
			c: lit(PLASTER, 0, amb * 0.16)
		},
		{
			pts: [
				[w.x1, w.y0, a],
				[w.x1, w.y0, b],
				[w.x1, w.y1, b],
				[w.x1, w.y1, a]
			],
			c: lit(PLASTER, 0, amb * 0.24)
		},
		{
			pts: [
				[w.x0, w.y0, a],
				[w.x1, w.y0, a],
				[w.x1, w.y0, b],
				[w.x0, w.y0, b]
			],
			c: lit(PLASTER, 0, amb * 0.06)
		},
		{
			pts: [
				[w.x0, w.y1, a],
				[w.x1, w.y1, a],
				[w.x1, w.y1, b],
				[w.x0, w.y1, b]
			],
			c: lit(PLASTER, 0, amb * 0.2)
		},
		// the transom, a slim bar seen against the light
		{
			pts: [
				[w.x0, ty - 2.2, b - 4],
				[w.x1, ty - 2.2, b - 4],
				[w.x1, ty + 2.2, b - 4],
				[w.x0, ty + 2.2, b - 4]
			],
			c: [26, 3, 5]
		}
	];
	ctx.save();
	ctx.setTransform(1, 0, 0, 1, 0, 0);
	for (const s of sides) {
		const p = poly(cam, s.pts);
		if (!p) continue;
		ctx.fillStyle = rgba(s.c);
		ctx.fill(p);
	}
	ctx.restore();
}

/** Whether the camera is still outside the window. */
export const outside = (cam: Cam) => cam.z - REF_B.z < R.wallZ - NEAR;
