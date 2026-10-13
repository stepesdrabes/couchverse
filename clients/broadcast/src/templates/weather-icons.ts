// The weather's icons, drawn in code in the brand's language: flat geometry on one 100-unit
// grid, the sun the brand's red (the idents' dusk sun) and the rest warm paper, with the only
// curves the circles a sun, a moon and a cloud are made of. Everything else is cut square: the
// clouds' flat bases, the rays, rain and fog bars, the bolt. A shape in front is cut out of the
// ones behind by a knockout gap that shows whatever the icon sits on (a mask, not a colour),
// so one icon reads alike on the night, on the map's red and on a warned kraj.

import { svg } from '../core/dom';
import { BRAND } from '../brand/wordmark';

/** The kinds of sky an icon shows. */
export type Sky =
	| 'clear'
	| 'partly'
	| 'cloudy'
	| 'fog'
	| 'drizzle'
	| 'rain'
	| 'sleet'
	| 'snow'
	| 'showers'
	| 'thunder';

/** Open-Meteo's WMO codes as a sky (freezing drizzle and rain read as sleet). */
export function skyOf(code: number): Sky {
	if (code === 0) return 'clear';
	if (code === 1 || code === 2) return 'partly';
	if (code === 45 || code === 48) return 'fog';
	if (code >= 51 && code <= 55) return 'drizzle';
	if (code === 56 || code === 57 || code === 66 || code === 67) return 'sleet';
	if (code >= 61 && code <= 65) return 'rain';
	if ((code >= 71 && code <= 77) || code === 85 || code === 86) return 'snow';
	if (code >= 80 && code <= 82) return 'showers';
	if (code >= 95) return 'thunder';
	return 'cloudy';
}

/** The skies that show the sun by day and the moon by night. */
export const LIT: ReadonlySet<Sky> = new Set<Sky>(['clear', 'partly', 'showers']);

const PAPER = BRAND.paper;
/** The cloud behind a cloud: paper dimmed, solid. */
const BACK_CLOUD = '#9a908a';
/** The knockout between a shape and the ones behind it, in grid units. */
const GAP = 5;

type Attrs = Record<string, string | number>;
interface Shape {
	tag: 'circle' | 'rect' | 'path' | 'line';
	a: Attrs;
}
/** Shapes in one colour; `cut` shapes knock out what is behind them. */
interface Layer {
	shapes: Shape[];
	fill: string;
	cut: boolean;
}

const circle = (cx: number, cy: number, r: number): Shape => ({ tag: 'circle', a: { cx, cy, r } });
const rect = (x: number, y: number, width: number, height: number): Shape => ({
	tag: 'rect',
	a: { x, y, width, height }
});
const bar = (x1: number, y1: number, x2: number, y2: number, w: number): Shape => ({
	tag: 'line',
	a: { x1, y1, x2, y2, 'stroke-width': w }
});
const path = (d: string): Shape => ({ tag: 'path', a: { d } });

/**
 * A cloud `w` wide standing on (x, y): a square-cornered base and three bumps, the outer two
 * meeting its sides where its corners are.
 */
function cloud(x: number, y: number, w: number): Shape[] {
	const k = w / 64;
	return [
		rect(x, y - 16 * k, 64 * k, 16 * k),
		circle(x + 16 * k, y - 16 * k, 16 * k),
		circle(x + 36 * k, y - 24 * k, 19 * k),
		circle(x + 52 * k, y - 16 * k, 12 * k)
	];
}

/** The sun: a disc and eight square-ended rays. */
function sun(cx: number, cy: number, r: number): Shape[] {
	const out = [circle(cx, cy, r)];
	for (let i = 0; i < 8; i++) {
		const a = (i / 8) * Math.PI * 2 + Math.PI / 8;
		const [c, s] = [Math.cos(a), Math.sin(a)];
		out.push(
			bar(cx + c * r * 1.5, cy + s * r * 1.5, cx + c * r * 1.95, cy + s * r * 1.95, r * 0.4)
		);
	}
	return out;
}

/** A crescent: a disc less an offset disc, as one path. */
function moon(cx: number, cy: number, r: number): Shape[] {
	const [dx, dy, ri] = [r * 0.62, -r * 0.42, r * 0.86];
	const d = Math.hypot(dx, dy);
	const a = (d * d + r * r - ri * ri) / (2 * d);
	const h = Math.sqrt(Math.max(0, r * r - a * a));
	const [ux, uy] = [dx / d, dy / d];
	const [px, py] = [cx + ux * a, cy + uy * a];
	const f = (v: number) => v.toFixed(2);
	const [x1, y1, x2, y2] = [px - uy * h, py + ux * h, px + uy * h, py - ux * h];
	return [
		path(
			`M${f(x1)} ${f(y1)}A${r} ${r} 0 1 1 ${f(x2)} ${f(y2)}A${ri} ${ri} 0 0 0 ${f(x1)} ${f(y1)}Z`
		)
	];
}

/** What falls from a cloud whose base is at y. */
function falling(kind: 'rain' | 'drizzle' | 'snow' | 'sleet' | 'showers', y: number): Shape[] {
	const streak = (x: number) => bar(x + 3, y + 3, x - 3, y + 16, 6);
	const flake = (x: number, cy: number): Shape[] =>
		[0, 60, 120].map((deg) => {
			const a = (deg * Math.PI) / 180;
			const [c, s] = [Math.cos(a) * 6.5, Math.sin(a) * 6.5];
			return bar(x - c, cy - s, x + c, cy + s, 3.6);
		});
	switch (kind) {
		case 'drizzle':
			return [34, 50, 66].map((x, i) => rect(x - 3.5, y + 6 + (i % 2) * 7, 7, 7));
		case 'rain':
			return [36, 52, 68].map(streak);
		case 'showers':
			return [44, 60].map(streak);
		case 'snow':
			return [...flake(34, y + 10), ...flake(50, y + 17), ...flake(66, y + 10)];
		case 'sleet':
			return [streak(40), ...flake(62, y + 11)];
	}
}

function layers(sky: Sky, night: boolean): Layer[] {
	const light = (cx: number, cy: number, r: number): Layer =>
		night
			? { shapes: moon(cx, cy, r * 1.2), fill: PAPER, cut: false }
			: { shapes: sun(cx, cy, r), fill: BRAND.red, cut: false };
	const front = (x: number, y: number, w: number): Layer => ({
		shapes: cloud(x, y, w),
		fill: PAPER,
		cut: true
	});
	const rainy = (kind: 'rain' | 'drizzle' | 'snow' | 'sleet'): Layer[] => [
		front(18, 62, 64),
		{ shapes: falling(kind, 62), fill: PAPER, cut: false }
	];
	switch (sky) {
		case 'clear':
			return [light(50, 50, night ? 22 : 17)];
		case 'partly':
			return [night ? light(66, 30, 15) : light(60, 38, 13), front(16, 80, 60)];
		case 'cloudy':
			return [{ shapes: cloud(36, 58, 50), fill: BACK_CLOUD, cut: false }, front(14, 80, 62)];
		case 'fog':
			return [
				{
					shapes: [rect(24, 34, 52, 8), rect(14, 48, 56, 8), rect(30, 62, 56, 8)],
					fill: PAPER,
					cut: false
				}
			];
		case 'drizzle':
		case 'rain':
		case 'snow':
		case 'sleet':
			return rainy(sky);
		case 'showers':
			return [
				night ? light(70, 26, 13) : light(64, 32, 11),
				front(16, 64, 60),
				{ shapes: falling('showers', 64), fill: PAPER, cut: false }
			];
		case 'thunder':
			return [
				front(18, 58, 64),
				{
					shapes: [path('M54 54L37 79H50L43 97L66 69H53L60 54Z')],
					fill: BRAND.red,
					cut: true
				}
			];
	}
}

let masks = 0;

function draw(parent: Element, s: Shape, fill: string) {
	const line = s.tag === 'line';
	return svg(
		s.tag,
		{
			...s.a,
			fill: line ? 'none' : fill,
			...(line ? { stroke: fill, 'stroke-linecap': 'butt' } : {})
		},
		parent
	);
}

export interface IconOptions {
	/** Its size on the stage, px (the grid's 100 units). */
	size: number;
	/** The moon rather than the sun, where the sky shows either. */
	night?: boolean;
	cls?: string;
}

/** The icon for a WMO code as an SVG `size` px square. */
export function weatherIcon(parent: Element, code: number, o: IconOptions): SVGSVGElement {
	const root = svg(
		'svg',
		{ width: o.size, height: o.size, viewBox: '0 0 100 100', 'aria-hidden': 'true' },
		parent
	);
	if (o.cls) root.setAttribute('class', o.cls);
	const all = layers(skyOf(code), o.night ?? false);
	all.forEach((layer, i) => {
		const cutters = all.slice(i + 1).filter((l) => l.cut);
		let into: Element = root;
		if (cutters.length) {
			const id = `wx-knockout-${++masks}`;
			const m = svg(
				'mask',
				{ id, maskUnits: 'userSpaceOnUse', x: -20, y: -20, width: 140, height: 140 },
				root
			);
			svg('rect', { x: -20, y: -20, width: 140, height: 140, fill: '#fff' }, m);
			for (const c of cutters)
				for (const s of c.shapes) {
					const w = s.tag === 'line' ? Number(s.a['stroke-width']) : 0;
					svg(
						s.tag,
						{
							...s.a,
							fill: '#000',
							stroke: '#000',
							'stroke-width': w + 2 * GAP,
							'stroke-linejoin': 'miter'
						},
						m
					);
				}
			into = svg('g', { mask: `url(#${id})` }, root);
		}
		for (const s of layer.shapes) draw(into, s, layer.fill);
	});
	return root;
}
