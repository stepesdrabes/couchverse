import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { pitchClass } from '../../src/audio/pitch';
import { FALL, STAGE } from '../../src/brand/ending';
import { LOGO, LOGO_H, LOGO_W, POPCORN, SILHOUETTE } from '../../src/brand/glyph';
import { SHORT_IDENT, SHORT_IDENT_TIMES } from '../../src/brand/short-ident';
import { BEAT, SONIC } from '../../src/brand/sonic';
import { STING, stingTimes } from '../../src/brand/sting';
import { BRAND, geometry, lockupSvg, PARTS, ROW, WORD } from '../../src/brand/wordmark';
import { edge, valueAt } from '../../src/core/moves';
import { frame } from '../../src/core/timeline';

const onFrame = (t: number) => Math.abs(t * 25 - Math.round(t * 25)) < 1e-9;
const D_MAJOR = ['D', 'E', 'F#', 'G', 'A', 'B', 'C#'].map(pitchClass);

describe('the logo', () => {
	const subpaths = (d: string) => d.split(/(?=M)/).sort();

	it('is the web app’s logo, the couch with the popcorn cut out of it and the popcorn', () => {
		const svg = readFileSync(resolve(import.meta.dirname, '../../../web/static/logo.svg'), 'utf8');
		const d = /<path d="([^"]+)"/.exec(svg)?.[1] ?? '';
		expect(subpaths(LOGO)).toEqual(subpaths(d));
		expect([LOGO_W, LOGO_H]).toEqual([1046, 745]);
	});

	it('knocks the popcorn out along its outer contour', () => {
		expect(POPCORN.startsWith(SILHOUETTE)).toBe(true);
		expect(subpaths(POPCORN)).toContain(SILHOUETTE);
		expect(SILHOUETTE.indexOf('M', 1)).toBe(-1);
	});
});

describe('the lockup', () => {
	it('stacks as every ident ends: the logo 540 px wide at y 222 over a 184 px word at 650', () => {
		expect(STAGE.logo.x).toBeCloseTo(690, 6);
		expect(STAGE.logo.y).toBeCloseTo(222, 6);
		expect(STAGE.logo.k * LOGO_W).toBeCloseTo(540, 6);
		// the word's advance centred on the stage, its line box's top at 650
		expect(STAGE.word.x + (WORD.advance / 2) * 184).toBeCloseTo(960, 6);
		expect(STAGE.word.y - WORD.baseline * 184).toBeCloseTo(650, 6);
	});

	it('rows the word beside the logo as the corner bug draws them', () => {
		// the bug: the logo 36 px tall beside a 34 px word whose line box shares its top
		const bug = geometry(34, 'row');
		expect(bug.height).toBeCloseTo(36, 6);
		expect(bug.word.y - WORD.baseline * 34).toBeCloseTo(bug.logo.y, 6);
		for (const size of [16, 34, 80 / ROW.logo]) {
			const g = geometry(size, 'row');
			expect(g.height / size).toBeCloseTo(ROW.logo, 6);
			expect(g.width / size).toBeCloseTo(5.6266, 3);
			// the c's ink 0.32 sizes from the logo
			expect(g.word.x + WORD.inkLeft * size - LOGO_W * g.logo.k).toBeCloseTo(0.32 * size, 6);
			expect(g.word.x + WORD.inkRight * size).toBeCloseTo(g.width, 6);
		}
	});

	it('lays the word out with the face’s advances and the tracking', () => {
		// couch 3071, t 421 and v 582 units at weight 800, each letter tracked -35
		expect(PARTS.map((p) => p.advance)).toEqual([3071 - 5 * 35, 421 - 35, 582 - 35]);
		expect(PARTS[1].x).toBe(PARTS[0].advance);
		expect(PARTS[2].x).toBe(PARTS[0].advance + PARTS[1].advance);
		expect(PARTS[2].x + PARTS[2].advance).toBe(WORD.advance * 1000);
	});

	it('is a pure SVG string of its options, in paths', () => {
		const svg = lockupSvg({ size: 24, label: 'Couch TV' });
		expect(lockupSvg({ size: 24, label: 'Couch TV' })).toBe(svg);
		expect(svg).toMatch(
			/^<svg xmlns="http:\/\/www.w3.org\/2000\/svg" width="135.04" height="25.41"/
		);
		expect(svg).toContain(`fill="${BRAND.white}"`);
		expect(svg).toContain('role="img" aria-label="Couch TV"');
		expect(svg).not.toContain('<text');
		expect(svg.match(/<path /g)).toHaveLength(4);
		expect(svg).toContain(LOGO);
		expect(lockupSvg({ size: 24 })).toContain('aria-hidden="true"');
		expect(lockupSvg({ size: 24, ink: '"><script>' })).not.toContain('<script>');
		const stack = lockupSvg({ size: 40, layout: 'stack' });
		const g = geometry(40, 'stack');
		expect(stack).toContain(`width="${Math.round(g.width * 100) / 100}"`);
		expect(g.height).toBeGreaterThan(3 * 40);
	});
});

describe('the sonic logo', () => {
	it('is A4, E5, D5 in D major, settling on D add 9', () => {
		expect(SONIC.notes).toEqual(['A4', 'E5', 'D5']);
		for (const n of [...SONIC.notes, ...SONIC.chord])
			expect(D_MAJOR.includes(pitchClass(n)), n).toBe(true);
	});

	it('plays couch on the beat, t and v half a beat apart, on frames', () => {
		expect(BEAT).toBe(0.8);
		expect(SONIC.at).toEqual([0, BEAT / 2, BEAT]);
		for (const t of SONIC.at) expect(onFrame(t), String(t)).toBe(true);
	});
});

describe('the short ident', () => {
	it('lands the popcorn, then types couchtv a beat later, and holds', () => {
		const { land, notes } = SHORT_IDENT_TIMES;
		expect(frame(SHORT_IDENT)).toBe(5.2);
		expect(land).not.toBeNull();
		expect(notes[0]).toBeCloseTo((land ?? 0) + BEAT, 9);
		expect(notes.map((n) => n - notes[0])).toEqual([0, 0.4, 0.8].map((x) => expect.closeTo(x, 9)));
		for (const n of notes) expect(onFrame(n)).toBe(true);
		// the last part types in well before the end, and the chord has two seconds to ring out
		expect(SHORT_IDENT - (notes[2] + 0.26)).toBeGreaterThan(1);
		expect(SHORT_IDENT - notes[2]).toBeGreaterThanOrEqual(2.4);
	});
});

describe('stings', () => {
	it('hold the popcorn drop and the whole sonic logo, with room to ring out, at every length', () => {
		expect([STING.min, STING.max]).toEqual([5, 10]);
		for (let f = STING.min * 25; f <= STING.max * 25; f++) {
			const D = f / 25;
			const { land, notes } = stingTimes(D);
			for (const n of notes) expect(onFrame(n), `${D}: ${n}`).toBe(true);
			// the last note has at least 1.8 s before the sound has to be over
			expect(D - notes[2], String(D)).toBeGreaterThanOrEqual(1.8);
			// the popcorn falls after the couch has come in, and couch types a beat after it lands
			expect(land - FALL).toBeGreaterThan(0.3);
			expect(notes[0] - land).toBeCloseTo(BEAT, 9);
		}
	});
});

describe('moves', () => {
	it('put cuts on frame edges', () => {
		expect(edge(1)).toBeCloseTo(0.98, 9);
		expect(edge(5.125)).toBeCloseTo(5.1, 9);
		expect(edge(0)).toBeLessThan(0);
	});

	it('evaluate like the keyframes they animate', () => {
		const mix = (a: number, b: number, p: number) => a + (b - a) * p;
		const moves = [
			{ at: 1, to: 10 },
			{ at: 2, dur: 0.5, to: 20, ease: 'linear' as const }
		];
		expect(valueAt(0, moves, 4, 0.96, mix)).toBe(0);
		expect(valueAt(0, moves, 4, 1.0, mix)).toBe(10);
		expect(valueAt(0, moves, 4, edge(2) + 0.25, mix)).toBeCloseTo(15, 6);
		expect(valueAt(0, moves, 4, 3, mix)).toBe(20);
	});
});
