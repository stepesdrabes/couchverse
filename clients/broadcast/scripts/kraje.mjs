// Writes backend/internal/feature/couchtv/kraje.geojson, the kraje the weather map fills for
// ČHMÚ's warnings, from geoBoundaries' CZE ADM1 download (gbOpen, release 9469f09; boundary
// data by ČÚZK, CC BY 4.0):
//
//   node scripts/kraje.mjs geoBoundaries-CZE-ADM1_simplified.geojson
//
// Neighbours share their border's vertices in the source, so the borders are cut into arcs
// where the lines branch (two kraje and the outside, or three kraje) and each arc is
// simplified once: a border drawn from either side is the same line, so the map has no gaps or
// overlaps. Douglas-Peucker in the outline's frame (the country fitted to 1600x900 px, x
// scaled by cos 49.8°) with each arc's ends kept; `npm run outline` then writes the module.
import { readFileSync, writeFileSync } from 'node:fs';
import { resolve } from 'node:path';

const TOLERANCE = 2;
const source = process.argv[2];
if (!source) throw new Error('usage: node scripts/kraje.mjs <geoBoundaries CZE ADM1 geojson>');
const doc = JSON.parse(readFileSync(source, 'utf8'));

// each kraj's rings without their closing point, in code order
const kraje = doc.features
	.map((f) => {
		const g = f.geometry;
		if (g.type !== 'Polygon') throw new Error(`${f.properties.shapeISO}: ${g.type}`);
		return {
			code: f.properties.shapeISO,
			name: f.properties.shapeName,
			rings: g.coordinates.map((r) => r.slice(0, -1))
		};
	})
	.sort((a, b) => a.code.localeCompare(b.code));

const key = (p) => `${p[0]},${p[1]}`;
const all = kraje.flatMap((k) => k.rings.flat());
const lons = all.map((p) => p[0]);
const lats = all.map((p) => p[1]);
const K = Math.cos((49.8 * Math.PI) / 180);
const west = Math.min(...lons);
const north = Math.max(...lats);
const scale = Math.min(1600 / ((Math.max(...lons) - west) * K), 900 / (north - Math.min(...lats)));
const px = (p) => [(p[0] - west) * K * scale, (north - p[1]) * scale];

// a vertex is a junction where the lines through it branch: more than two neighbours over all
// the rings that have it
const neighbours = new Map();
for (const k of kraje)
	for (const r of k.rings)
		r.forEach((p, i) => {
			const set = neighbours.get(key(p)) ?? new Set();
			set.add(key(r[(i + r.length - 1) % r.length]));
			set.add(key(r[(i + 1) % r.length]));
			neighbours.set(key(p), set);
		});
const junction = (p) => neighbours.get(key(p)).size > 2;

function simplify(points) {
	if (points.length < 3) return points;
	const xy = points.map(px);
	const keep = points.map((_, i) => i === 0 || i === points.length - 1);
	const stack = [[0, points.length - 1]];
	while (stack.length) {
		const [a, b] = stack.pop();
		const [ax, ay] = xy[a];
		const [bx, by] = xy[b];
		const len = Math.hypot(bx - ax, by - ay);
		let far = -1;
		let at = -1;
		for (let i = a + 1; i < b; i++) {
			const [x, y] = xy[i];
			const d = len
				? Math.abs((bx - ax) * (ay - y) - (ax - x) * (by - ay)) / len
				: Math.hypot(x - ax, y - ay);
			if (d > far) [far, at] = [d, i];
		}
		if (far > TOLERANCE) {
			keep[at] = true;
			stack.push([a, at], [at, b]);
		}
	}
	return points.filter((_, i) => keep[i]);
}

// each arc simplified once, by whichever ring meets it first; its neighbour reads it reversed
const arcs = new Map();
function arc(points) {
	const id = points.map(key).join(';');
	const back = points.map(key).reverse().join(';');
	if (arcs.has(id)) return arcs.get(id);
	if (arcs.has(back)) return [...arcs.get(back)].reverse();
	const s = simplify(points);
	arcs.set(id, s);
	return s;
}

const round = (v) => Math.round(v * 1e4) / 1e4;
let vertices = 0;
const features = kraje.map((k) => {
	const rings = k.rings.map((ring) => {
		let cuts = ring.map((p, i) => (junction(p) ? i : -1)).filter((i) => i >= 0);
		// a ring that meets no other line (Prague, and the hole it leaves) is cut at two vertices
		// both sides pick alike, whichever way they walk it
		if (!cuts.length) {
			const by = (better) => ring.reduce((m, p, i) => (better(key(p), key(ring[m])) ? i : m), 0);
			cuts = [by((a, b) => a < b), by((a, b) => a > b)].sort((a, b) => a - b);
		}
		const out = [];
		cuts.forEach((c, j) => {
			const next = cuts[(j + 1) % cuts.length];
			const pts = [];
			for (let i = c; ; i = (i + 1) % ring.length) {
				pts.push(ring[i]);
				if (i === next && pts.length > 1) break;
			}
			out.push(...arc(pts).slice(0, -1));
		});
		const closed = [...out, out[0]].map((p) => [round(p[0]), round(p[1])]);
		vertices += closed.length - 1;
		return closed;
	});
	return {
		type: 'Feature',
		properties: { iso: k.code, name: k.name },
		geometry: { type: 'Polygon', coordinates: rings }
	};
});

const out = resolve(import.meta.dirname, '../../../backend/internal/feature/couchtv/kraje.geojson');
const meta = {
	source:
		'geoBoundaries CZE ADM1 (gbOpen, release 9469f09), boundary source: Czech Office for Surveying, Mapping and Cadastre (ČÚZK)',
	license: 'CC BY 4.0',
	simplification: `Ramer-Douglas-Peucker on the borders between kraje (each simplified once, so neighbours share it), ${TOLERANCE} px tolerance with the country fitted to 1600x900 px (equirectangular, x scaled by cos 49.8 deg), coordinates rounded to 4 decimals`,
	vertices
};
writeFileSync(
	out,
	JSON.stringify({ type: 'FeatureCollection', properties: meta, features }) + '\n'
);
console.log(`wrote ${out}: ${features.length} kraje, ${vertices} vertices`);
