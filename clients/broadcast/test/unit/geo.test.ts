import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { czechiaMap, krajeEdge } from '../../src/core/geo';
import { CZECHIA } from '../../src/generated/czechia';
import { KRAJE } from '../../src/generated/kraje';
import type { NewsStory, WeatherWarning } from '../../src/spec';
import { holds, reach } from '../../src/templates/news';
import {
	degrees,
	range,
	regionsPhrase,
	warnedKraje,
	warningTime
} from '../../src/templates/weather';
import { skyOf } from '../../src/templates/weather-icons';

const data = (name: string) =>
	JSON.parse(
		readFileSync(
			resolve(import.meta.dirname, `../../../../backend/internal/feature/couchtv/${name}`),
			'utf8'
		)
	);

describe('the Czechia map', () => {
	it('is the outline the server credits (npm run outline after a change)', () => {
		expect(CZECHIA).toEqual(data('czechia.geojson').features[0].geometry.coordinates[0]);
	});

	it('has the kraje of their source (npm run outline after a change)', () => {
		const doc = data('kraje.geojson');
		expect(Object.keys(KRAJE)).toHaveLength(14);
		for (const f of doc.features)
			expect(KRAJE[f.properties.iso as keyof typeof KRAJE], f.properties.iso).toEqual(
				f.geometry.coordinates
			);
	});

	it('has kraje that share their borders: no gaps, no overlaps', () => {
		const key = (p: readonly number[]) => `${p[0]},${p[1]}`;
		const count = new Map<string, number>();
		for (const rings of Object.values(KRAJE))
			for (const r of rings)
				for (let i = 1; i < r.length; i++) {
					const [a, b] = [key(r[i - 1]), key(r[i])];
					const s = a < b ? `${a};${b}` : `${b};${a}`;
					count.set(s, (count.get(s) ?? 0) + 1);
				}
		// a border is had by two kraje, the country's edge by one, nothing by more
		expect(Math.max(...count.values())).toBe(2);
		// and the edge is one closed ring: a border simplified apart would leave a sliver
		const edge = krajeEdge();
		const single = [...count.values()].filter((c) => c === 1).length;
		expect(edge.length - 1).toBe(single);
		expect(key(edge[0])).toBe(key(edge[edge.length - 1]));
		// it follows the outline the server credits within a few hundred metres
		const lons = edge.map((p) => p[0]);
		const lats = edge.map((p) => p[1]);
		const cl = CZECHIA.map((p) => p[0]);
		const ca = CZECHIA.map((p) => p[1]);
		expect(Math.abs(Math.min(...lons) - Math.min(...cl))).toBeLessThan(0.01);
		expect(Math.abs(Math.max(...lons) - Math.max(...cl))).toBeLessThan(0.01);
		expect(Math.abs(Math.min(...lats) - Math.min(...ca))).toBeLessThan(0.01);
		expect(Math.abs(Math.max(...lats) - Math.max(...ca))).toBeLessThan(0.01);
	});

	it('fits the box, keeps the aspect and places the cities inside the border', () => {
		const map = czechiaMap({ x: 100, y: 200, width: 900, height: 900 });
		expect(map.box.width).toBeCloseTo(900, 6);
		expect(map.box.height).toBeLessThan(900);
		// centred where the box is taller than the map
		expect(map.box.y - 200).toBeCloseTo((900 - map.box.height) / 2, 6);
		// about 1.74 wide per high at the middle latitude
		expect(map.box.width / map.box.height).toBeGreaterThan(1.7);
		expect(map.box.width / map.box.height).toBeLessThan(1.78);
		const [px, py] = map.point(50.088, 14.4208);
		const [bx, by] = map.point(49.1952, 16.608);
		const [ox] = map.point(49.8347, 18.282);
		// Prague north-west of Brno, Ostrava east of both
		expect(px).toBeLessThan(bx);
		expect(py).toBeLessThan(by);
		expect(ox).toBeGreaterThan(bx);
		for (const [x, y] of [map.point(50.088, 14.4208), map.point(48.9745, 14.4743)]) {
			expect(x).toBeGreaterThan(map.box.x);
			expect(x).toBeLessThan(map.box.x + map.box.width);
			expect(y).toBeGreaterThan(map.box.y);
			expect(y).toBeLessThan(map.box.y + map.box.height);
		}
		expect(map.path.startsWith('M')).toBe(true);
		expect(map.path.endsWith('Z')).toBe(true);
		// Prague inside Central Bohemia's box, and its hole cut into it
		const prague = map.kraje.find((k) => k.code === 'CZ-10')!;
		const central = map.kraje.find((k) => k.code === 'CZ-20')!;
		expect(prague.box.x).toBeGreaterThan(central.box.x);
		expect(prague.box.x + prague.box.width).toBeLessThan(central.box.x + central.box.width);
		expect(central.path.match(/M/g)).toHaveLength(2);
		expect(map.edge.endsWith('Z')).toBe(true);
		expect(map.inner.startsWith('M')).toBe(true);
	});
});

const regions = (...r: [string, string, boolean][]) =>
	r.map(([code, name, partial]) => ({ code, name, partial }) as WeatherWarning['regions'][0]);

describe('the weather', () => {
	it('reads WMO codes as skies', () => {
		expect([0, 1, 2, 3, 45, 51, 56, 61, 66, 71, 80, 85, 95, 99].map(skyOf)).toEqual([
			'clear',
			'partly',
			'partly',
			'cloudy',
			'fog',
			'drizzle',
			'sleet',
			'rain',
			'sleet',
			'snow',
			'showers',
			'snow',
			'thunder',
			'thunder'
		]);
	});

	it('words degrees, ranges and warning times', () => {
		expect(degrees(-3)).toBe('−3°');
		expect(degrees(0)).toBe('0°');
		expect(range([12, 16, 14])).toBe('12–16°');
		expect(range([-3, 2])).toBe('−3° to 2°');
		expect(range([5, 5])).toBe('5°');
		const r = regions(['CZ-10', 'Prague', false]);
		expect(
			warningTime({
				level: 'yellow',
				event: 'Wind',
				active: true,
				from: { time: '16:00' },
				until: { day: 'Wednesday', time: '02:00' },
				regions: r
			})
		).toBe('Now until Wednesday 02:00');
		expect(
			warningTime({
				level: 'red',
				event: 'Heat',
				active: false,
				from: { time: '14:00' },
				regions: r
			})
		).toBe('From 14:00 until further notice');
	});

	it("names a warning's kraje whole, then those it covers in part", () => {
		const w = (r: WeatherWarning['regions']): WeatherWarning => ({
			level: 'yellow',
			event: 'Wind',
			active: true,
			from: { time: '16:00' },
			regions: r
		});
		expect(
			regionsPhrase(
				w(
					regions(
						['CZ-31', 'South Bohemian Region', true],
						['CZ-63', 'Vysočina Region', false],
						['CZ-71', 'Olomouc Region', true]
					)
				)
			)
		).toBe('Vysočina Region and parts of the South Bohemian and Olomouc Regions');
		expect(regionsPhrase(w(regions(['CZ-20', 'Central Bohemian Region', true])))).toBe(
			'parts of the Central Bohemian Region'
		);
		expect(
			regionsPhrase(
				w(regions(['CZ-10', 'Prague', false], ['CZ-52', 'Hradec Králové Region', false]))
			)
		).toBe('Prague and Hradec Králové Region');
	});

	it('fills each warned kraj at its most severe level', () => {
		const base = { active: true, event: 'x', from: { time: '10:00' } };
		const warned = warnedKraje([
			{ ...base, level: 'orange', regions: regions(['CZ-20', 'Central', true]) },
			{
				...base,
				level: 'yellow',
				regions: regions(['CZ-20', 'Central', false], ['CZ-31', 'South', true])
			},
			{ ...base, level: 'orange', regions: regions(['CZ-20', 'Central', false]) }
		]);
		expect(warned.get('CZ-20')).toEqual({ level: 'orange', partial: false });
		expect(warned.get('CZ-31')).toEqual({ level: 'yellow', partial: true });
		expect(warned.has('CZ-10')).toBe(false);
	});
});

describe('the news', () => {
	const story = (headline: string, outlets = 1): NewsStory => ({
		headline,
		outlet: 'ČT24',
		outlets
	});

	it('gives each story its reading time, filling the bulletin on the half beat', () => {
		const slots = holds(
			[story('a'.repeat(60)), story('b'.repeat(120)), story('c'.repeat(30))],
			0.6,
			19.5
		);
		expect(slots[0][0]).toBe(0.6);
		expect(slots[2][1]).toBe(19.5);
		for (let i = 1; i < slots.length; i++) {
			expect(slots[i][0]).toBe(slots[i - 1][1]);
			expect(Math.abs(slots[i][0] / 0.4 - Math.round(slots[i][0] / 0.4))).toBeLessThan(1e-9);
		}
		// the longest headline reads longest
		const len = slots.map(([a, b]) => b - a);
		expect(len[1]).toBeGreaterThan(len[0]);
		expect(len[0]).toBeGreaterThan(len[2] - 0.4);
	});

	it('says how many outlets carry a story', () => {
		expect(reach(story('x', 4))).toBe('Reported by 4 outlets');
		expect(reach(story('x'))).toBe('');
	});
});
