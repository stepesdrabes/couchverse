import { describe, expect, it } from 'vitest';
import { contrast, palette, parseHex, pickAccent, type Rgb } from '../../src/core/palette';
import { accent, colors } from '../../src/generated/tokens';

const surface = parseHex(colors['surface-2']) as Rgb;
const stage = parseHex(colors.bg) as Rgb;

function hue(hex: string) {
	const [r, g, b] = (parseHex(hex) as Rgb).map((v) => v / 255);
	const max = Math.max(r, g, b);
	const d = max - Math.min(r, g, b);
	if (!d) return 0;
	const h = max === r ? ((g - b) / d) % 6 : max === g ? (b - r) / d + 2 : (r - g) / d + 4;
	return (h * 60 + 360) % 360;
}

describe('pickAccent', () => {
	it('takes the more chromatic accent', () => {
		expect(pickAccent('#643906', '#d62316')).toBe('#d62316');
		expect(pickAccent('#1a2554', '#777777')).toBe('#1a2554');
	});

	it('falls back to the channel accent', () => {
		expect(pickAccent(undefined, 'nope')).toBe(colors.accent);
	});
});

describe('palette', () => {
	const samples = [
		'#0a2a44',
		'#d62316',
		'#f5cc36',
		'#7a5a3a',
		'#1a2554',
		'#57b7e6',
		'#e50914',
		'#101010'
	];

	it('gives an ink that reads on every surface', () => {
		for (const hex of samples) {
			const p = palette(hex);
			expect(contrast(p.inkRgb, surface)).toBeGreaterThanOrEqual(accent.inkContrast);
		}
	});

	it('keeps the hue when it lightens (navy turns blue, not grey)', () => {
		const p = palette('#0a2a44');
		expect(Math.abs(hue(p.ink) - hue('#0a2a44'))).toBeLessThan(6);
		const [r, g, b] = p.inkRgb;
		expect(b - r).toBeGreaterThan(80);
		expect(g).toBeLessThan(b);
	});

	it('uses the ink as the fill when the accent disappears on the stage', () => {
		const navy = palette('#0a2a44');
		expect(contrast(navy.rgb, stage)).toBeGreaterThanOrEqual(3);
		expect(navy.accent).toBe(navy.ink);
		expect(palette('#e50914').accent).toBe('#e50914');
	});

	it('picks a readable colour on the accent', () => {
		expect(palette('#f5cc36').onAccent).toBe(accent.onAccentDark);
		expect(palette('#e50914').onAccent).toBe(accent.onAccentLight);
	});

	it('survives an unreadable colour', () => {
		expect(palette('garbage').base).toBe(colors.accent);
	});
});
