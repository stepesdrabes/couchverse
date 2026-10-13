// Scene A: dusk. The couch on a hilltop with the sun going down behind it, ranges of hills in
// the haze, the sky darkening toward the east where the camera will turn.

import { type Cam, layer, linear, radial, ridge, smooth } from './art';
import { REF_A } from './camera';
import { couchAt, drawCouch, scratch } from './couch';
import { W } from './film';

const SUN = { x: 960, y: 712, r: 150 };
const CREST = 806;

let ranges: Record<'far' | 'mid' | 'near' | 'hill' | 'front', Path2D> | null = null;

/** The ranges of hills as paths, made on first use (the module also loads without a canvas). */
const hills = () =>
	(ranges ??= {
		far: ridge(
			[
				[-900, 660],
				[-560, 618],
				[-260, 640],
				[120, 596],
				[420, 636],
				[700, 690],
				[960, 712],
				[1240, 686],
				[1520, 612],
				[1760, 640],
				[2100, 622],
				[2500, 662],
				[2900, 690],
				[3500, 700]
			],
			1500
		),
		mid: ridge(
			[
				[-900, 742],
				[-380, 722],
				[160, 748],
				[560, 736],
				[900, 752],
				[1280, 744],
				[1680, 724],
				[2150, 750],
				[2700, 742],
				[3500, 760]
			],
			1500
		),
		near: ridge(
			[
				[-900, 800],
				[-260, 784],
				[340, 806],
				[700, 818],
				[1240, 816],
				[1700, 792],
				[2250, 812],
				[2900, 826],
				[3600, 834]
			],
			1500
		),
		hill: ridge(
			[
				[-900, 1010],
				[-300, 930],
				[300, 852],
				[700, 814],
				[960, CREST],
				[1220, 813],
				[1640, 846],
				[2200, 920],
				[2900, 1020]
			],
			1600
		),
		front: ridge(
			[
				[-900, 900],
				[-300, 930],
				[80, 960],
				[360, 1060],
				[700, 1190],
				[1300, 1200],
				[1640, 1080],
				[1900, 1000],
				[2300, 980],
				[2800, 1060],
				[3600, 1120]
			],
			1800
		)
	});

export function drawSky(ctx: CanvasRenderingContext2D, cam: Cam) {
	ctx.save();
	ctx.setTransform(1, 0, 0, 1, -cam.pan, -cam.tilt);
	const x0 = cam.pan - 10;
	const x1 = cam.pan + W + 10;
	// dusk: near-black overhead, deep red, then a hot band along the horizon
	ctx.fillStyle = linear(ctx, 0, -150, 0, 760, [
		[0, '#050101'],
		[0.22, '#140103'],
		[0.42, '#360308'],
		[0.6, '#72060e'],
		[0.75, '#b80b15'],
		[0.86, '#e5241f'],
		[0.94, '#f8482c'],
		[1, '#ff6236']
	]);
	ctx.fillRect(x0, -400, x1 - x0, 1900);
	// the glow around the sun, wide along the horizon
	ctx.save();
	ctx.translate(SUN.x, SUN.y);
	ctx.scale(2.4, 1);
	ctx.fillStyle = radial(ctx, 0, 0, 760, [
		[0, '#ffe2cc', 0.9],
		[0.16, '#ff8a5c', 0.62],
		[0.36, '#ff4a2a', 0.34],
		[0.64, '#d8121a', 0.12],
		[1, '#d8121a', 0]
	]);
	ctx.fillRect(-900, -900, 1800, 1800);
	ctx.restore();
	// night to the east: the same sky with the light gone
	const g = scratch();
	g.setTransform(1, 0, 0, 1, -cam.pan, -cam.tilt);
	g.fillStyle = linear(g, 0, -150, 0, 820, [
		[0, '#030101'],
		[0.5, '#090102'],
		[0.8, '#180204'],
		[1, '#2e0307']
	]);
	g.fillRect(x0, -400, x1 - x0, 1900);
	g.globalCompositeOperation = 'destination-in';
	g.fillStyle = linear(g, 1500, 0, 3300, 0, [
		[0, '#000', 0],
		[0.5, '#000', 0.7],
		[1, '#000', 1]
	]);
	g.fillRect(x0, -400, x1 - x0, 1900);
	ctx.setTransform(1, 0, 0, 1, 0, 0);
	ctx.drawImage(g.canvas, 0, 0);
	ctx.restore();
}

/** Long soft bands of cloud low over the horizon: a shade darker than the sky, lit underneath. */
const CLOUDS: [number, number, number, number][] = [
	// x, y, length, thickness (sky pixels)
	[700, 600, 980, 15],
	[1420, 566, 760, 11],
	[300, 544, 640, 9]
];

export function drawClouds(ctx: CanvasRenderingContext2D, cam: Cam, t: number) {
	ctx.save();
	ctx.setTransform(1, 0, 0, 1, -cam.pan - 8 * t, -cam.tilt);
	ctx.filter = 'blur(2.2px)';
	for (const [x, y, len, th] of CLOUDS) {
		const p = new Path2D();
		// a long lens, its belly flatter than its back
		p.moveTo(x - len / 2, y);
		p.bezierCurveTo(
			x - len / 4,
			y - th * 1.2,
			x + len / 4,
			y - th * 1.2,
			x + len / 2,
			y - th * 0.15
		);
		p.bezierCurveTo(x + len / 4, y + th * 0.45, x - len / 4, y + th * 0.5, x - len / 2, y);
		p.closePath();
		const near = Math.max(0, 1 - Math.abs(x - SUN.x) / 800);
		ctx.fillStyle = linear(ctx, 0, y - th, 0, y + th * 0.5, [
			[0, '#7a0710', 0.5],
			[0.6, '#a00c14', 0.5],
			[1, near > 0.3 ? '#ffc4a8' : '#e0402a', 0.35 + 0.4 * near]
		]);
		ctx.fill(p);
	}
	ctx.restore();
}

function sunY(t: number) {
	// it sinks while we watch, and night follows it east
	return SUN.y + 30 * smooth(0, 2.7, t);
}

export function drawSun(ctx: CanvasRenderingContext2D, cam: Cam, t: number) {
	const y = sunY(t);
	ctx.save();
	ctx.setTransform(1, 0, 0, 1, -cam.pan, -cam.tilt);
	ctx.fillStyle = radial(ctx, SUN.x, y, SUN.r * 1.7, [
		[0.55, '#ffd8c4', 0.32],
		[0.75, '#ff9a72', 0.12],
		[1, '#ff6a40', 0]
	]);
	ctx.fillRect(SUN.x - SUN.r * 2, y - SUN.r * 2, SUN.r * 4, SUN.r * 4);
	ctx.fillStyle = radial(ctx, SUN.x, y, SUN.r, [
		[0, '#fffaf5'],
		[0.86, '#fff3ea'],
		[0.985, '#ffe6d6'],
		[1, '#ffd2bc', 0]
	]);
	ctx.beginPath();
	ctx.arc(SUN.x, y, SUN.r, 0, Math.PI * 2);
	ctx.fill();
	ctx.restore();
}

/**
 * A land layer: dark along its ridge, paler where haze pools at its foot, and falling into night
 * toward the east, where the camera turns.
 */
function land(
	ctx: CanvasRenderingContext2D,
	cam: Cam,
	d: number,
	path: Path2D,
	top: number,
	bottom: number,
	ridgeColor: string,
	hazeColor: string,
	blur = 0
) {
	const m = layer(cam, REF_A, d);
	if (!m) return;
	ctx.save();
	ctx.setTransform(m);
	if (blur > 0) ctx.filter = `blur(${blur}px)`;
	ctx.fillStyle = linear(ctx, 0, top, 0, bottom, [
		[0, ridgeColor],
		[1, hazeColor]
	]);
	ctx.fill(path);
	ctx.filter = 'none';
	ctx.clip(path);
	ctx.fillStyle = linear(ctx, 1500, 0, 3200, 0, [
		[0, '#050101', 0],
		[0.6, '#050101', 0.7],
		[1, '#050101', 0.9]
	]);
	ctx.fillRect(1400, -200, 4000, 1800);
	ctx.restore();
}

/** Sunlight grazing a ridge: a thin bright line along its crest, strongest toward the sun. */
function crestLight(
	ctx: CanvasRenderingContext2D,
	m: DOMMatrix,
	path: Path2D,
	width: number,
	reach: number,
	color: string,
	alpha: number
) {
	const g = scratch();
	g.setTransform(m);
	g.fillStyle = radial(g, SUN.x, SUN.y, reach, [
		[0, color, 1],
		[0.5, color, 0.45],
		[1, color, 0]
	]);
	g.save();
	g.translate(SUN.x, SUN.y);
	g.scale(1, 0.3);
	g.translate(-SUN.x, -SUN.y);
	g.fillRect(SUN.x - reach, SUN.y - reach * 4, reach * 2, reach * 8);
	g.restore();
	g.globalCompositeOperation = 'destination-in';
	g.fill(path);
	g.globalCompositeOperation = 'destination-out';
	g.filter = `blur(${Math.max(0.3, width * 0.35 * m.a)}px)`;
	g.translate(0, width);
	g.fill(path);
	ctx.save();
	ctx.setTransform(1, 0, 0, 1, 0, 0);
	ctx.globalAlpha = alpha;
	ctx.drawImage(g.canvas, 0, 0);
	ctx.restore();
}

/** The distant ranges, behind everything in town. */
/** The distant ranges and the hills in the haze, behind the town. */
export function drawDuskFar(ctx: CanvasRenderingContext2D, cam: Cam) {
	land(ctx, cam, 9000, hills().far, 600, 740, '#8f0a13', '#d8302c');
	land(ctx, cam, 3600, hills().mid, 720, 800, '#4c050c', '#8c0e15');
	// haze lit by the low sun, over the distant layers
	ctx.save();
	ctx.setTransform(1, 0, 0, 1, -cam.pan, -cam.tilt);
	ctx.globalCompositeOperation = 'screen';
	ctx.translate(SUN.x, SUN.y + 30);
	ctx.scale(3, 1);
	ctx.fillStyle = radial(ctx, 0, 0, 260, [
		[0, '#ff7a50', 0.5],
		[0.5, '#c8201c', 0.18],
		[1, '#600408', 0]
	]);
	ctx.fillRect(-300, -300, 600, 600);
	ctx.restore();
}

/** The nearer hills, the couch's hilltop and the foreground. */
export function drawDuskNear(ctx: CanvasRenderingContext2D, cam: Cam) {
	land(ctx, cam, 1800, hills().near, 784, 870, '#220205', '#3c040a');
	const m = layer(cam, REF_A, 1000);
	if (m) {
		ctx.save();
		ctx.setTransform(m);
		ctx.fillStyle = linear(ctx, 0, CREST, 0, 1100, [
			[0, '#120203'],
			[1, '#070101']
		]);
		ctx.fill(hills().hill);
		ctx.restore();
		crestLight(ctx, m, hills().hill, 5, 700, '#ff8a62', 0.85);
		drawCouch(ctx, couchAt(960, CREST + 2, 340).preMultiplySelf(m), {
			top: '#5c0610',
			bottom: '#2c0307',
			rim: { color: '#ffd9c4', width: 9, dx: 0, dy: -1, soft: 2.5, alpha: 1 }
		});
	}
	land(ctx, cam, 380, hills().front, 920, 1300, '#060101', '#020101', 9);
}
