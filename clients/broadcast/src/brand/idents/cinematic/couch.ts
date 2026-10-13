// The couch and the popcorn as canvas paths (logo units, 1046 x 745), and the couch lit by a
// scene: a body gradient, a rim where the light grazes its edges, a pool of light across it.

import { COUCH, COUCH_TOP, GAP, LOGO_H, LOGO_W, POPCORN } from '../../glyph';
import { H, W } from './film';
import { linear, radial, rgba, type RGB } from './art';

export { COUCH_TOP, GAP, LOGO_H, LOGO_W };

let paths: { couch: Path2D; popcorn: Path2D } | null = null;

export function glyphs() {
	if (!paths) paths = { couch: new Path2D(COUCH), popcorn: new Path2D(POPCORN) };
	return paths;
}

/** The couch's bottom centre in logo units (its feet). */
export const FOOT_X = LOGO_W / 2;
export const FOOT_Y = LOGO_H;

/** A matrix placing the couch with its feet centred on (x, y), `width` pixels wide. */
export function couchAt(x: number, y: number, width: number): DOMMatrix {
	const s = width / LOGO_W;
	return new DOMMatrix([s, 0, 0, s, x - FOOT_X * s, y - FOOT_Y * s]);
}

export interface Rim {
	color: RGB | string;
	/** How far the light reaches in from the edge, logo units. */
	width: number;
	/** Direction the light comes from (unit vector, logo space; y down). */
	dx: number;
	dy: number;
	/** Softness of the terminator, logo units. */
	soft: number;
	alpha?: number;
}

export interface Look {
	top: RGB | string;
	bottom: RGB | string;
	rim?: Rim;
	/** A pool of light on the couch (logo units). */
	pool?: { x: number; y: number; r: number; color: RGB | string; alpha: number };
	alpha?: number;
}

let fx: CanvasRenderingContext2D | null = null;

/** A full-frame scratch layer (float16, like the frame) for effects that need isolation. */
export function scratch(): CanvasRenderingContext2D {
	if (!fx) {
		const c = document.createElement('canvas');
		c.width = W;
		c.height = H;
		fx = c.getContext('2d', {
			colorType: 'float16'
		} as CanvasRenderingContext2DSettings) as CanvasRenderingContext2D;
	}
	fx.setTransform(1, 0, 0, 1, 0, 0);
	fx.globalAlpha = 1;
	fx.globalCompositeOperation = 'source-over';
	fx.filter = 'none';
	fx.clearRect(0, 0, W, H);
	return fx;
}

export function drawCouch(ctx: CanvasRenderingContext2D, m: DOMMatrix, look: Look) {
	const { couch } = glyphs();
	const s = m.a;
	ctx.save();
	ctx.globalAlpha = look.alpha ?? 1;
	ctx.setTransform(m);
	ctx.fillStyle = linear(ctx, 0, COUCH_TOP, 0, LOGO_H, [
		[0, look.top],
		[1, look.bottom]
	]);
	ctx.fill(couch);
	if (look.pool) {
		const p = look.pool;
		ctx.save();
		ctx.clip(couch);
		ctx.fillStyle = radial(ctx, p.x, p.y, p.r, [
			[0, p.color, p.alpha],
			[1, p.color, 0]
		]);
		ctx.fillRect(-40, COUCH_TOP - 40, LOGO_W + 80, LOGO_H - COUCH_TOP + 80);
		ctx.restore();
	}
	ctx.restore();
	if (look.rim && (look.rim.alpha ?? 1) > 0.001) {
		const r = look.rim;
		// the rim colour over the couch, minus the couch moved away from the light: what stays
		// is a band along the edges that face the light
		const g = scratch();
		g.setTransform(m);
		g.fillStyle = rgba(r.color);
		g.fill(couch);
		g.globalCompositeOperation = 'destination-out';
		g.filter = r.soft > 0 ? `blur(${Math.max(0.01, r.soft * s)}px)` : 'none';
		g.translate(-r.dx * r.width, -r.dy * r.width);
		g.fill(couch);
		g.filter = 'none';
		g.globalCompositeOperation = 'source-over';
		ctx.save();
		ctx.setTransform(1, 0, 0, 1, 0, 0);
		ctx.globalAlpha = (look.alpha ?? 1) * (r.alpha ?? 1);
		ctx.drawImage(g.canvas, 0, 0);
		ctx.restore();
	}
}

/** The popcorn with its knockout halo in the field colour (the logo's drawing). */
export function drawPopcorn(
	ctx: CanvasRenderingContext2D,
	m: DOMMatrix,
	fill: string,
	field: string,
	alpha = 1
) {
	const { popcorn } = glyphs();
	ctx.save();
	ctx.globalAlpha = alpha;
	ctx.setTransform(m);
	ctx.lineJoin = 'round';
	ctx.lineWidth = GAP * 2;
	ctx.strokeStyle = field;
	ctx.fillStyle = field;
	ctx.stroke(popcorn);
	ctx.fill(popcorn);
	ctx.fillStyle = fill;
	ctx.fill(popcorn);
	ctx.restore();
}
