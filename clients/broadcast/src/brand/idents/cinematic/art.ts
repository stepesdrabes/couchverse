// Drawing helpers: colours, gradients, smooth ridgelines and the 2.5D camera.

export const F = 1100;

export type RGB = [number, number, number];

export function hex(h: string): RGB {
	const n = parseInt(h.slice(1), 16);
	return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

export function mix(a: RGB | string, b: RGB | string, t: number): RGB {
	const x = typeof a === 'string' ? hex(a) : a;
	const y = typeof b === 'string' ? hex(b) : b;
	return [x[0] + (y[0] - x[0]) * t, x[1] + (y[1] - x[1]) * t, x[2] + (y[2] - x[2]) * t];
}

export function rgba(c: RGB | string, a = 1): string {
	const x = typeof c === 'string' ? hex(c) : c;
	return `rgba(${x[0].toFixed(2)}, ${x[1].toFixed(2)}, ${x[2].toFixed(2)}, ${a.toFixed(4)})`;
}

export type Stop = [number, RGB | string, number?];

export function linear(
	ctx: CanvasRenderingContext2D,
	x0: number,
	y0: number,
	x1: number,
	y1: number,
	stops: Stop[]
): CanvasGradient {
	const g = ctx.createLinearGradient(x0, y0, x1, y1);
	for (const [o, c, a] of stops) g.addColorStop(Math.min(1, Math.max(0, o)), rgba(c, a ?? 1));
	return g;
}

export function radial(
	ctx: CanvasRenderingContext2D,
	x: number,
	y: number,
	r: number,
	stops: Stop[],
	r0 = 0
): CanvasGradient {
	const g = ctx.createRadialGradient(x, y, r0, x, y, r);
	for (const [o, c, a] of stops) g.addColorStop(Math.min(1, Math.max(0, o)), rgba(c, a ?? 1));
	return g;
}

/** A smooth curve through the points (Catmull-Rom as Beziers). */
export function curve(p: Path2D, pts: [number, number][], move = true) {
	if (move) p.moveTo(pts[0][0], pts[0][1]);
	else p.lineTo(pts[0][0], pts[0][1]);
	for (let i = 0; i < pts.length - 1; i++) {
		const a = pts[Math.max(0, i - 1)];
		const b = pts[i];
		const c = pts[i + 1];
		const d = pts[Math.min(pts.length - 1, i + 2)];
		p.bezierCurveTo(
			b[0] + (c[0] - a[0]) / 6,
			b[1] + (c[1] - a[1]) / 6,
			c[0] - (d[0] - b[0]) / 6,
			c[1] - (d[1] - b[1]) / 6,
			c[0],
			c[1]
		);
	}
}

/** A ridgeline through the points, closed down to `bottom`: a land silhouette. */
export function ridge(pts: [number, number][], bottom: number): Path2D {
	const p = new Path2D();
	curve(p, pts);
	p.lineTo(pts[pts.length - 1][0], bottom);
	p.lineTo(pts[0][0], bottom);
	p.closePath();
	return p;
}

export const clamp01 = (v: number) => Math.max(0, Math.min(1, v));
export const lerp = (a: number, b: number, t: number) => a + (b - a) * t;
/** 0 before a, 1 after b, smooth (C2) in between. */
export const smooth = (a: number, b: number, t: number) => {
	const x = clamp01((t - a) / (b - a));
	return x * x * x * (x * (x * 6 - 15) + 10);
};

// --- the camera ------------------------------------------------------------------------------

/**
 * The camera. `pan`/`tilt` turn it (every layer shifts alike, the sky too); `x`/`y` move it
 * sideways and up (near layers shift more: parallax); `z` moves it forward. Units: screen pixels
 * for the turn, and world units (a pixel at depth F) for the moves.
 */
export interface Cam {
	pan: number;
	tilt: number;
	x: number;
	y: number;
	z: number;
}

/**
 * Where a scene was drawn from: its layers are authored as screen pixels seen from this camera,
 * so a layer at distance d shows exactly as drawn when the camera is here.
 */
export type Ref = Cam;

/** The transform that places a layer at distance d (from the scene's reference camera). */
export function layer(cam: Cam, ref: Ref, d: number): DOMMatrix | null {
	const den = d - (cam.z - ref.z);
	if (den <= 1) return null;
	const k = d / den;
	const tx = 960 - 960 * k - ((cam.x - ref.x) * F) / den - (cam.pan - ref.pan);
	const ty = 540 - 540 * k - ((cam.y - ref.y) * F) / den - (cam.tilt - ref.tilt);
	return new DOMMatrix([k, 0, 0, k, tx, ty]);
}

/** A world point (scene reference camera at the origin, looking down +Z) on the screen. */
export function project(cam: Cam, ref: Ref, X: number, Y: number, Z: number): [number, number] {
	const den = Math.max(1e-3, Z - (cam.z - ref.z));
	return [
		960 + (F * (X - (cam.x - ref.x))) / den - (cam.pan - ref.pan),
		540 + (F * (Y - (cam.y - ref.y))) / den - (cam.tilt - ref.tilt)
	];
}

/** How far the camera is in front of a world depth Z (scene reference). */
export const depthOf = (cam: Cam, ref: Ref, Z: number) => Z - (cam.z - ref.z);
