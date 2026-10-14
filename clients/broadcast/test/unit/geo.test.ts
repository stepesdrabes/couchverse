import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { czechiaMap, krajeEdge } from '../../src/core/geo';
import { CZECHIA } from '../../src/generated/czechia';
import { KRAJE } from '../../src/generated/kraje';
import type { NewsStory, WeatherWarning } from '../../src/spec';
import { BEAT, share } from '../../src/templates/bulletin';
import { outletsOf, reach, running } from '../../src/templates/news';
import {
	degrees,
	precipitation,
	range,
	regionsPhrase,
	schedule,
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

const onBeat = (t: number) => Math.abs(t / BEAT - Math.round(t / BEAT)) < 1e-9;

describe('the bulletins', () => {
	const part = (w: number, min: number, max: number) => ({ w, min, max });

	it('share whole beats among parts, within their bounds, exactly to the total', () => {
		const parts = [part(4, 3, 6), part(10, 9, 16), part(10, 9, 16), part(12, 9, 16)];
		for (const total of [34, 45, 54]) {
			const out = share(parts, total);
			expect(out.reduce((a, b) => a + b, 0)).toBe(total);
			out.forEach((b, i) => {
				expect(Number.isInteger(b)).toBe(true);
				expect(b).toBeGreaterThanOrEqual(parts[i].min);
				expect(b).toBeLessThanOrEqual(parts[i].max);
			});
		}
		// a part that wants more gets more
		const [a, b] = share([part(4, 1, 20), part(8, 1, 20)], 30);
		expect(b).toBeGreaterThan(a);
	});

	it('give way when the bounds cannot hold', () => {
		const parts = [part(4, 3, 6), part(10, 9, 16)];
		// under the minimums: both shrink alike
		expect(share(parts, 6)).toEqual([2, 4]);
		// over the maximums: both stretch alike
		const out = share(parts, 44);
		expect(out.reduce((a, b) => a + b, 0)).toBe(44);
		expect(out[1]).toBeGreaterThan(16);
	});
});

describe('the news', () => {
	const story = (headline: string, outlets = 1, outlet = 'ČT24'): NewsStory => ({
		headline,
		outlet,
		outlets
	});
	const D = 54.4;

	it('opens, gives each story its reading time and recaps, all on the beat to the last frame', () => {
		for (const n of [3, 4, 5]) {
			const stories = Array.from({ length: n }, (_, i) => story('a'.repeat(50 + 20 * i)));
			const run = running(stories, D);
			expect(run.stories).toHaveLength(n);
			expect(run.stories[0][0]).toBe(run.open);
			expect(run.stories[n - 1][1]).toBe(run.recap);
			for (let i = 0; i < n; i++) {
				const [a, b] = run.stories[i];
				// a story reads for 7 to 13 s, whatever the count
				expect(b - a).toBeGreaterThanOrEqual(7.2 - 1e-9);
				expect(b - a).toBeLessThanOrEqual(12.8 + 1e-9);
				expect(onBeat(a) && onBeat(b)).toBe(true);
				if (i) expect(a).toBe(run.stories[i - 1][1]);
			}
			// the open takes a few seconds, the recap and sign-off at least 7
			expect(run.open).toBeGreaterThanOrEqual(2.4);
			expect(run.open).toBeLessThanOrEqual(4.8 + 1e-9);
			expect(D - run.recap).toBeGreaterThanOrEqual(7.2);
		}
	});

	it('gives a longer headline longer', () => {
		const run = running([story('a'.repeat(40)), story('b'.repeat(130)), story('c'.repeat(40))], D);
		const len = run.stories.map(([a, b]) => b - a);
		expect(len[1]).toBeGreaterThan(len[0]);
		expect(len[1]).toBeGreaterThan(len[2]);
	});

	it('holds fewer stories longer and fills the same bulletin', () => {
		const stories = Array.from({ length: 5 }, () => story('a'.repeat(80)));
		const hold = (n: number) => {
			const run = running(stories.slice(0, n), D);
			return (run.recap - run.open) / n;
		};
		expect(hold(3)).toBeGreaterThan(hold(5));
		for (const n of [1, 2, 3, 4, 5]) expect(running(stories.slice(0, n), D).recap).toBeLessThan(D);
	});

	it('says how many outlets carry a story and which outlets the stories come from', () => {
		expect(reach(story('x', 4))).toBe('Reported by 4 outlets');
		expect(reach(story('x'))).toBe('');
		expect(
			outletsOf([story('a', 1, 'ČT24'), story('b', 1, 'iROZHLAS'), story('c', 1, 'ČT24')])
		).toEqual(['ČT24', 'iROZHLAS']);
	});
});

describe('the weather scenes', () => {
	const D = 38.4;

	it('follow one another on the beat, the sign-off in the last scene', () => {
		for (const shown of [null, 0, 1, 2, 3]) {
			const s = schedule(D, shown, 5);
			const starts = [0, s.warnings, s.today, s.tomorrow, s.days].filter(
				(t): t is number => t != null
			);
			expect(starts).toEqual([...starts].sort((a, b) => a - b));
			expect(starts.every(onBeat)).toBe(true);
			expect(s.warnings != null).toBe(shown != null);
			// every scene reads for at least four seconds, the last long enough to hold the tag
			starts.forEach((t, i) => expect((starts[i + 1] ?? D) - t).toBeGreaterThanOrEqual(3.2));
			expect(s.tag).toBeGreaterThan(s.days! + 2.4);
			expect(s.tag).toBeLessThan(D - 4);
		}
	});

	it('give each warning its turn within the warnings scene', () => {
		const s = schedule(D, 3, 5);
		expect(s.each).toHaveLength(3);
		expect(s.each[0]).toBeGreaterThan(s.warnings!);
		expect(s.each[2]).toBeLessThan(s.today);
		// each warning is on screen for 4.5 s at least, whatever else the bulletin holds (the
		// regions line of a long warning took longer to read than the 3.4 s it had)
		for (const shown of [1, 2, 3]) {
			const t = schedule(D, shown, 5);
			const stops = [...t.each.slice(1), t.today];
			t.each.forEach((at, k) =>
				expect(stops[k] - at, `${shown} shown`).toBeGreaterThanOrEqual(4.5)
			);
		}
		// more warnings take longer, a quiet day shortest
		const len = (shown: number) => {
			const t = schedule(D, shown, 5);
			return t.today - t.warnings!;
		};
		expect(len(3)).toBeGreaterThan(len(1));
		expect(len(1)).toBeGreaterThan(len(0));
	});

	it('keep to what the forecast has: one day ends on today', () => {
		const s = schedule(D, null, 1);
		expect(s.warnings).toBeUndefined();
		expect(s.tomorrow).toBeUndefined();
		expect(s.days).toBeUndefined();
		expect(s.tag).toBeGreaterThan(s.today + 2.4);
	});

	it('words the day chance of precipitation', () => {
		expect(precipitation([20, 85, 40])).toBe('Precipitation chance up to 85%');
		expect(precipitation([0, 0])).toBe('No precipitation expected');
		expect(precipitation([undefined, 30])).toBe('Precipitation chance up to 30%');
		expect(precipitation([undefined])).toBe('');
	});
});
