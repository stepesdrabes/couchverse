// Czechia on the stage: the border (src/generated/czechia.ts), its kraje (src/generated/kraje.ts)
// and the weather's cities placed on it by one projection. Equirectangular with the longitude
// scaled by the cosine of 49.8° (the country's middle latitude), the projection the outlines
// were simplified in: at Czechia's size it is indistinguishable from a conformal one.

import { CZECHIA } from '../generated/czechia';
import { KRAJE } from '../generated/kraje';
import type { KrajCode } from '../spec';

/** Where the map goes on the stage, px. */
export interface MapBox {
	x: number;
	y: number;
	width: number;
	height: number;
}

/** A kraj on the stage. */
export interface KrajShape {
	code: KrajCode;
	/** Its area as a closed SVG path, its holes included (fill it even-odd). */
	path: string;
	/** Its own box on the stage. */
	box: MapBox;
}

export interface CzechiaMap {
	/** The border as an SVG path in stage pixels, closed. */
	path: string;
	/** The border's own box on the stage: the map keeps its aspect, centred in the box asked for. */
	box: MapBox;
	/** A place (degrees north, degrees east) on the stage, px. */
	point(lat: number, lon: number): [number, number];
	/** Every kraj, in code order. */
	kraje: KrajShape[];
	/** The borders between kraje, each drawn once, as open SVG lines. */
	inner: string;
	/** The country's edge as the kraje meet it, closed. */
	edge: string;
}

const K = Math.cos((49.8 * Math.PI) / 180);
const lons = CZECHIA.map((p) => p[0]);
const lats = CZECHIA.map((p) => p[1]);
const WEST = Math.min(...lons);
const NORTH = Math.max(...lats);
/** The outline's size in projected degrees. */
const W = (Math.max(...lons) - WEST) * K;
const H = NORTH - Math.min(...lats);

type Point = readonly [number, number];
const key = (p: Point) => `${p[0]},${p[1]}`;
const segment = (a: Point, b: Point) => {
	const [x, y] = [key(a), key(b)];
	return x < y ? `${x};${y}` : `${y};${x}`;
};

/**
 * The kraje's segments by how many kraje have them: two for a border between kraje, one on the
 * country's edge (the kraje share their borders' vertices, see scripts/kraje.mjs).
 */
function segments() {
	const count = new Map<string, number>();
	for (const rings of Object.values(KRAJE))
		for (const r of rings)
			for (let i = 1; i < r.length; i++) {
				const s = segment(r[i - 1], r[i]);
				count.set(s, (count.get(s) ?? 0) + 1);
			}
	return count;
}

let edgeRing: Point[] | null = null;

/**
 * The kraje's outline: the segments only one kraj has, chained into one ring (the kraje tile
 * the country without holes, so every vertex on it has two such segments).
 */
export function krajeEdge(): Point[] {
	if (edgeRing) return edgeRing;
	const count = segments();
	const links = new Map<string, Point[]>();
	for (const rings of Object.values(KRAJE))
		for (const r of rings)
			for (let i = 1; i < r.length; i++) {
				if (count.get(segment(r[i - 1], r[i])) !== 1) continue;
				for (const [a, b] of [
					[r[i - 1], r[i]],
					[r[i], r[i - 1]]
				])
					links.set(key(a), [...(links.get(key(a)) ?? []), b]);
			}
	const [start] = links.values().next().value ?? [];
	if (!start) return [];
	const ring: Point[] = [start];
	let prev: Point | null = null;
	for (let at = start; ;) {
		const [a, b] = links.get(key(at)) ?? [];
		const next: Point = prev && key(a) === key(prev) ? b : a;
		if (key(next) === key(start)) break;
		ring.push(next);
		[prev, at] = [at, next];
		if (ring.length > links.size) throw new Error('the kraje leave a gap');
	}
	edgeRing = [...ring, start];
	return edgeRing;
}

/** Czechia fitted into `into`, as large as it fits. */
export function czechiaMap(into: MapBox): CzechiaMap {
	const s = Math.min(into.width / W, into.height / H);
	const box = {
		x: into.x + (into.width - W * s) / 2,
		y: into.y + (into.height - H * s) / 2,
		width: W * s,
		height: H * s
	};
	const point = (lat: number, lon: number): [number, number] => [
		box.x + (lon - WEST) * K * s,
		box.y + (NORTH - lat) * s
	];
	const xy = ([lon, lat]: Point) => {
		const [x, y] = point(lat, lon);
		return `${x.toFixed(1)} ${y.toFixed(1)}`;
	};
	const ringPath = (r: readonly Point[]) =>
		r.map((p, i) => `${i ? 'L' : 'M'}${xy(p)}`).join('') + 'Z';
	const path = ringPath(CZECHIA);

	const kraje = (Object.keys(KRAJE) as KrajCode[]).map((code) => {
		const rings = KRAJE[code];
		const pts = rings[0].map(([lon, lat]) => point(lat, lon));
		const xs = pts.map((p) => p[0]);
		const ys = pts.map((p) => p[1]);
		const x = Math.min(...xs);
		const y = Math.min(...ys);
		return {
			code,
			path: rings.map(ringPath).join(''),
			box: { x, y, width: Math.max(...xs) - x, height: Math.max(...ys) - y }
		};
	});

	// the borders between kraje: runs of shared segments, each drawn once
	const count = segments();
	const drawn = new Set<string>();
	let inner = '';
	for (const rings of Object.values(KRAJE))
		for (const r of rings) {
			let open = false;
			for (let i = 1; i < r.length; i++) {
				const s = segment(r[i - 1], r[i]);
				if (count.get(s) !== 2 || drawn.has(s)) {
					open = false;
					continue;
				}
				drawn.add(s);
				if (!open) inner += `M${xy(r[i - 1])}`;
				inner += `L${xy(r[i])}`;
				open = true;
			}
		}
	return { path, box, point, kraje, inner, edge: ringPath(krajeEdge()) };
}
