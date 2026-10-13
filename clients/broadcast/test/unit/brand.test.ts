import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import { pitchClass } from '../../src/audio/pitch';
import { bracketPath } from '../../src/brand/brackets';
import { LOGO, LOGO_H, LOGO_W, POPCORN, SILHOUETTE } from '../../src/brand/glyph';
import { BEAT, SONIC } from '../../src/brand/sonic';
import { CHART, GRID, HOOK, SCALES } from '../../src/brand/theme';
import { BRAND, geometry, lockupSvg, PARTS, ROW, WORD } from '../../src/brand/wordmark';
import { edge, valueAt } from '../../src/core/moves';
import { frame } from '../../src/core/timeline';

/** The points of a bracket path: the outline from top to bottom, then the inner edge back. */
function points(d: string): [number, number][] {
	return d
		.slice(1, -1)
		.split('L')
		.map((p) => p.split(' ').map(Number) as [number, number]);
}

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

describe('the theme', () => {
	it('keeps every chord in its key and the hook in D major', () => {
		const inKey = (note: string, key: keyof typeof SCALES) =>
			SCALES[key].map(pitchClass).includes(pitchClass(note));
		for (const [name, { notes, key }] of Object.entries(CHART))
			for (const n of notes) expect(inKey(n, key), `${name}: ${n} in ${key}`).toBe(true);
		for (const n of HOOK.notes) expect(inKey(n, 'D major'), n).toBe(true);
	});

	it('cuts the idents to whole frames', () => {
		expect(frame(GRID.D)).toBe(4.52);
		// the tag's hook lands on frames wherever the tag starts on one
		for (const r of HOOK.tagAt) expect(Math.abs(r * 25 - Math.round(r * 25))).toBeLessThan(1e-9);
	});
});

describe('the old brackets', () => {
	const em = 190;
	const n = 40;
	const stroke = (k: number) => {
		const p = points(bracketPath(em, k, -1));
		const outer = p.slice(0, 2 * n + 1);
		const inner = p.slice(2 * n + 1).reverse();
		return {
			bar: inner[0][1] - outer[0][1],
			stem: inner[n][0] - outer[n][0],
			height: outer[2 * n][1] - outer[0][1],
			width: -outer[n][0]
		};
	};

	it('grow whole with what they frame: the O of a bigger Saira, stroke and all', () => {
		const rest = stroke(1);
		for (const k of [1.5, 2.2, 3.1]) {
			const s = stroke(k);
			expect(s.height).toBeCloseTo(rest.height * k, 0);
			expect(s.width).toBeCloseTo(rest.width * k, 0);
			expect(s.bar).toBeCloseTo(rest.bar * k, 0);
			expect(s.stem).toBeCloseTo(rest.stem * k, 0);
		}
		// Saira's O: a heavier stem than bar
		expect(rest.stem / em).toBeCloseTo(0.175, 2);
		expect(rest.bar / em).toBeCloseTo(0.132, 2);
	});

	it('open on the cut, mirrored', () => {
		const l = points(bracketPath(em, 1, -1, 10, 20));
		const r = points(bracketPath(em, 1, 1, 10, 20));
		expect(l[0][0]).toBe(10);
		expect(l[0][1]).toBeCloseTo(20 - 0.3495 * em, 1);
		for (let i = 0; i < l.length; i++) {
			expect(l[i][0] - 10).toBeCloseTo(-(r[i][0] - 10), 1);
			expect(l[i][1]).toBe(r[i][1]);
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
