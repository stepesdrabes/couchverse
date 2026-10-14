import { describe, expect, it } from 'vitest';
import { lift, onNight, reading, reads, SEEN, type Pixels } from '../../src/core/logo';
import { contrast, parseHex, toHsl, type Rgb } from '../../src/core/palette';

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

describe('lift, by chroma and in proportion', () => {
	it('leaves a near-black with a cool tint alone, as it left a black', () => {
		// HSL saturation read rgb(14, 12, 24) as a colour and lifted it to a lavender
		const p = solid([14, 12, 24]);
		const before = [...p.data];
		expect(lift(p, NIGHT)).toBe(false);
		expect([...p.data]).toEqual(before);
	});

	it('leaves the brown shading of a metal logo as drawn', () => {
		const p = logo([
			{ x: 0, y: 0, w: 40, h: 10, c: [60, 38, 26] },
			{ x: 0, y: 10, w: 40, h: 10, c: [150, 150, 156] }
		]);
		const before = [...p.data];
		expect(lift(p, NIGHT)).toBe(false);
		expect([...p.data]).toEqual(before);
	});

	it('lifts a shaded logo in proportion: the shades keep their order and their difference', () => {
		const shades: Rgb[] = [
			[190, 52, 58],
			[138, 28, 34],
			[96, 16, 22]
		];
		const p = logo(shades.map((c, i) => ({ x: 0, y: i * 6, w: 40, h: 6, c })));
		expect(lift(p, NIGHT)).toBe(true);
		const lightness = shades.map((_, i) => toHsl(px(p, i * 6 * 40))[2]);
		const was = shades.map((c) => toHsl(c)[2]);
		// the order is kept and no shade lands on another's colour
		expect(lightness[0]).toBeGreaterThan(lightness[1] + 0.03);
		expect(lightness[1]).toBeGreaterThan(lightness[2] + 0.03);
		// the shades come closer by the same share, not all to one lightness
		expect((lightness[0] - lightness[2]) / (was[0] - was[2])).toBeGreaterThan(0.3);
		expect((lightness[0] - lightness[2]) / (was[0] - was[2])).toBeLessThan(1);
		// and the darkest tenth reads
		expect(contrast(px(p, 2 * 6 * 40), NIGHT)).toBeGreaterThanOrEqual(4.5);
	});
});

describe('how a logo is drawn on night', () => {
	it('draws a logo that reads as it is', () => {
		const p = solid([255, 213, 28]);
		const before = [...p.data];
		expect(onNight(p, NIGHT)).toBe('as is');
		expect([...p.data]).toEqual(before);
	});

	it('lifts a dark red logo, and the lift reads', () => {
		const p = solid([138, 28, 34]);
		expect(onNight(p, NIGHT)).toBe('lifted');
		expect(reads(reading(p, NIGHT))).toBe(true);
	});

	it('sets the name for a black wordmark with a red accent, not the accent alone', () => {
		const p = logo([
			{ x: 0, y: 0, w: 40, h: 20, c: [12, 12, 12] },
			{ x: 30, y: 6, w: 6, h: 8, c: [200, 20, 20] }
		]);
		expect(onNight(p, NIGHT)).toBe('name');
	});

	it('does not take a lift that leaves the logo unread: it is as it is while a quarter shows, else the name', () => {
		// 52 % dark red (a lift is tried) and 48 % black: lifted, only 52 % reads
		const mostly = logo([
			{ x: 0, y: 0, w: 40, h: 20, c: [12, 12, 12] },
			{ x: 0, y: 0, w: 21, h: 20, c: [138, 28, 34] }
		]);
		expect(onNight(mostly, NIGHT)).toBe('name');
		// the same with a light grey carrying 30 % of it: as made
		const metal = logo([
			{ x: 0, y: 0, w: 40, h: 20, c: [60, 38, 26] },
			{ x: 0, y: 0, w: 12, h: 20, c: [200, 200, 205] }
		]);
		const before = [...metal.data];
		expect(onNight(metal, NIGHT)).toBe('as is');
		expect([...metal.data]).toEqual(before);
	});

	it('draws a logo of silver and brown, mostly dark, exactly as it was made', () => {
		// the Harry Potter logo's kind: a quarter of it silver, the rest brown and near-black
		const p = logo([
			{ x: 0, y: 0, w: 40, h: 20, c: [24, 20, 22] },
			{ x: 0, y: 0, w: 40, h: 6, c: [120, 78, 56] },
			{ x: 0, y: 6, w: 16, h: 14, c: [190, 196, 205] }
		]);
		const before = [...p.data];
		expect(onNight(p, NIGHT)).toBe('as is');
		expect([...p.data]).toEqual(before);
	});

	it('works on an ImageData, whose size is a getter on its prototype', () => {
		class Getters implements Pixels {
			constructor(
				private readonly w: number,
				private readonly h: number,
				readonly data: Uint8ClampedArray
			) {}
			get width() {
				return this.w;
			}
			get height() {
				return this.h;
			}
		}
		const flat = solid([138, 28, 34]);
		const p = new Getters(flat.width, flat.height, flat.data);
		expect(onNight(p, NIGHT)).toBe('lifted');
		expect(reads(reading(p, NIGHT))).toBe(true);
	});
});
