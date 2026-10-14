import { describe, expect, it } from 'vitest';
import { lift, reading, reads, SEEN, type Pixels } from '../../src/core/logo';
import { contrast, parseHex, type Rgb } from '../../src/core/palette';

const NIGHT = parseHex('#050505') as Rgb;
const PAPER = parseHex('#f4ede4') as Rgb;
const RED = parseHex('#e50914') as Rgb;

type Block = { x: number; y: number; w: number; h: number; c: Rgb };

/** A 40 x 20 logo of coloured blocks on transparency. */
function logo(blocks: Block[]): Pixels {
	const width = 40;
	const height = 20;
	const data = new Uint8ClampedArray(width * height * 4);
	for (const { x, y, w, h, c } of blocks)
		for (let j = y; j < y + h; j++)
			for (let i = x; i < x + w; i++) data.set([...c, 255], (j * width + i) * 4);
	return { data, width, height };
}

const solid = (c: Rgb) => logo([{ x: 0, y: 0, w: 40, h: 20, c }]);
const px = (p: Pixels, i: number): Rgb => [p.data[i * 4], p.data[i * 4 + 1], p.data[i * 4 + 2]];

describe('reading', () => {
	it('reads a light logo on night and a dark one on paper', () => {
		expect(reads(reading(solid([255, 213, 28]), NIGHT))).toBe(true);
		expect(reads(reading(solid([138, 28, 34]), PAPER))).toBe(true);
		expect(reads(reading(solid([255, 213, 28]), PAPER))).toBe(false);
		expect(reads(reading(solid([138, 28, 34]), NIGHT))).toBe(false);
	});

	it('reads a logo outlined in black on paper by its outline', () => {
		const outlined = logo([
			{ x: 0, y: 0, w: 40, h: 20, c: [20, 20, 20] },
			{ x: 2, y: 2, w: 36, h: 16, c: [255, 213, 28] }
		]);
		const r = reading(outlined, PAPER);
		expect(r.fill).toBeLessThan(0.55);
		expect(r.rim).toBe(1);
		expect(reads(r)).toBe(true);
	});

	it('does not read a light logo on paper, nor a dark red one on red', () => {
		expect(reads(reading(solid([207, 227, 255]), PAPER))).toBe(false);
		expect(reads(reading(solid([138, 28, 34]), RED))).toBe(false);
		expect(reads(reading(solid([255, 213, 28]), RED))).toBe(true);
	});

	it('counts a logo with nothing opaque as reading', () => {
		const empty = { data: new Uint8ClampedArray(40 * 20 * 4), width: 40, height: 20 };
		expect(reads(reading(empty, NIGHT))).toBe(true);
	});
});

describe('lift', () => {
	it('brightens a dark red until it reads, its hue kept', () => {
		const p = solid([138, 28, 34]);
		expect(lift(p, NIGHT)).toBe(true);
		const [r, g, b] = px(p, 0);
		expect(contrast([r, g, b], NIGHT)).toBeGreaterThanOrEqual(4.5);
		expect(r).toBeGreaterThan(g * 2);
		expect(reads(reading(p, NIGHT))).toBe(true);
	});

	it('leaves greys, blacks and whites as drawn: a logo made for paper is not inverted', () => {
		const p = logo([
			{ x: 0, y: 0, w: 20, h: 20, c: [20, 20, 20] },
			{ x: 20, y: 0, w: 20, h: 20, c: [120, 120, 120] }
		]);
		const before = [...p.data];
		expect(lift(p, NIGHT)).toBe(false);
		expect([...p.data]).toEqual(before);
		expect(reading(p, NIGHT).fill).toBeLessThan(SEEN + 0.3);
	});

	it('keeps the outline of a coloured logo and lifts only its colour', () => {
		const p = logo([
			{ x: 0, y: 0, w: 40, h: 20, c: [20, 20, 20] },
			{ x: 2, y: 2, w: 36, h: 16, c: [138, 28, 34] }
		]);
		lift(p, NIGHT);
		expect(px(p, 0)).toEqual([20, 20, 20]);
		expect(contrast(px(p, 3 * 40 + 3), NIGHT)).toBeGreaterThanOrEqual(4.5);
	});

	it('is a pure function of the pixels', () => {
		const a = solid([138, 28, 34]);
		const b = solid([138, 28, 34]);
		lift(a, NIGHT);
		lift(b, NIGHT);
		expect([...a.data]).toEqual([...b.data]);
	});
});
