import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { czechiaMap } from '../../src/core/geo';
import { CZECHIA } from '../../src/generated/czechia';
import { condition, degrees, warningTime } from '../../src/templates/weather';

describe('the Czechia map', () => {
	it('is the outline the server credits (npm run outline after a change)', () => {
		const doc = JSON.parse(
			readFileSync(
				resolve(
					import.meta.dirname,
					'../../../../backend/internal/feature/couchtv/czechia.geojson'
				),
				'utf8'
			)
		);
		expect(CZECHIA).toEqual(doc.features[0].geometry.coordinates[0]);
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
	});
});

describe('the weather placeholder', () => {
	it('words codes, degrees and warning times', () => {
		expect(condition(0)).toBe('Sunny');
		expect(condition(0, false)).toBe('Clear');
		expect(condition(80)).toBe('Showers');
		expect(condition(99)).toBe('Thunderstorms with hail');
		expect(degrees(-3)).toBe('−3°');
		expect(degrees(0)).toBe('0°');
		const regions = [{ code: 'CZ-10' as const, name: 'Prague', partial: false }];
		expect(
			warningTime({
				level: 'yellow',
				event: 'Wind',
				active: true,
				from: { time: '16:00' },
				until: { day: 'Wednesday', time: '02:00' },
				regions
			})
		).toBe('Now until Wednesday 02:00');
		expect(
			warningTime({ level: 'red', event: 'Heat', active: false, from: { time: '14:00' }, regions })
		).toBe('From 14:00 until further notice');
	});
});
