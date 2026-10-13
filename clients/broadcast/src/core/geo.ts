// Czechia on the stage: the border (src/generated/czechia.ts) and the weather's cities placed
// on it by one projection. Equirectangular with the longitude scaled by the cosine of 49.8°
// (the country's middle latitude), the projection the outline was simplified in: at Czechia's
// size it is indistinguishable from a conformal one.

import { CZECHIA } from '../generated/czechia';

/** Where the map goes on the stage, px. */
export interface MapBox {
	x: number;
	y: number;
	width: number;
	height: number;
}

export interface CzechiaMap {
	/** The border as an SVG path in stage pixels, closed. */
	path: string;
	/** The border's own box on the stage: the map keeps its aspect, centred in the box asked for. */
	box: MapBox;
	/** A place (degrees north, degrees east) on the stage, px. */
	point(lat: number, lon: number): [number, number];
}

const K = Math.cos((49.8 * Math.PI) / 180);
const lons = CZECHIA.map((p) => p[0]);
const lats = CZECHIA.map((p) => p[1]);
const WEST = Math.min(...lons);
const NORTH = Math.max(...lats);
/** The outline's size in projected degrees. */
const W = (Math.max(...lons) - WEST) * K;
const H = NORTH - Math.min(...lats);

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
	const path =
		CZECHIA.map(([lon, lat], i) => {
			const [x, y] = point(lat, lon);
			return `${i ? 'L' : 'M'}${x.toFixed(1)} ${y.toFixed(1)}`;
		}).join('') + 'Z';
	return { path, box, point };
}
