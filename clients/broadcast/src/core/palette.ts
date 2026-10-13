// A title's broadcast palette from its artwork accent. The app's derivation (theme.ts, the
// core's theme module) with one deliberate difference: the ink walks HSL lightness instead of
// mixing toward white, so a navy accent becomes a bright blue on air rather than steel grey
// (the approved sample reel's choice; the contrast target is the same token).

import { accent as tokens, colors } from '../generated/tokens';

export type Rgb = [number, number, number];

export interface Palette {
	/** The fill colour: the accent, or its ink when the accent is too dark for the stage. */
	accent: string;
	/** The accent as given. */
	base: string;
	rgb: Rgb;
	strong: string;
	onAccent: string;
	/** The accent as text on the stage's surfaces. */
	ink: string;
	inkRgb: Rgb;
	/** A deep shade for grades and shadows. */
	dark: string;
}

export function parseHex(hex: string | undefined | null): Rgb | null {
	const m = /^#?([0-9a-f]{6})$/i.exec((hex ?? '').trim());
	if (!m) return null;
	const n = parseInt(m[1], 16);
	return [(n >> 16) & 255, (n >> 8) & 255, n & 255];
}

const clamp = (n: number) => Math.max(0, Math.min(255, Math.round(n)));

export const toHex = (rgb: readonly number[]) =>
	'#' + rgb.map((v) => clamp(v).toString(16).padStart(2, '0')).join('');

/** Mix toward black (amount < 0) or white (amount > 0). */
export function mix(rgb: Rgb, amount: number): Rgb {
	const target = amount < 0 ? 0 : 255;
	const t = Math.abs(amount);
	return rgb.map((c) => clamp(c + (target - c) * t)) as Rgb;
}

/** WCAG relative luminance (0 = black, 1 = white). */
export function luminance([r, g, b]: Rgb): number {
	const lin = (c: number) => {
		const s = c / 255;
		return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
	};
	return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

export function contrast(a: Rgb, b: Rgb): number {
	const [la, lb] = [luminance(a), luminance(b)];
	return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
}

export function toHsl([r, g, b]: Rgb): [number, number, number] {
	const [rr, gg, bb] = [r / 255, g / 255, b / 255];
	const max = Math.max(rr, gg, bb);
	const min = Math.min(rr, gg, bb);
	const l = (max + min) / 2;
	if (max === min) return [0, 0, l];
	const d = max - min;
	const s = l > 0.5 ? d / (2 - max - min) : d / (max + min);
	const h =
		max === rr
			? (gg - bb) / d + (gg < bb ? 6 : 0)
			: max === gg
				? (bb - rr) / d + 2
				: (rr - gg) / d + 4;
	return [h / 6, s, l];
}

export function fromHsl([h, s, l]: [number, number, number]): Rgb {
	const f = (n: number) => {
		const k = (n + h * 12) % 12;
		const a = s * Math.min(l, 1 - l);
		return clamp(255 * (l - a * Math.max(-1, Math.min(k - 3, 9 - k, 1))));
	};
	return [f(0), f(8), f(4)];
}

const SURFACE = parseHex(colors['surface-2']) as Rgb;
const STAGE = parseHex(colors.bg) as Rgb;

/** The least HSL lightening that reads at the tokens' contrast on the lightest surface. */
export function ink(rgb: Rgb): Rgb {
	if (contrast(rgb, SURFACE) >= tokens.inkContrast) return rgb;
	const [h, s, l] = toHsl(rgb);
	for (let step = 0; l + step / 100 <= 1.0001; step++) {
		const c = fromHsl([h, s, Math.min(1, l + step / 100)]);
		if (contrast(c, SURFACE) >= tokens.inkContrast) return c;
	}
	return [255, 255, 255];
}

const chroma = (hex: string | undefined) => {
	const c = parseHex(hex);
	return c ? Math.max(...c) - Math.min(...c) : -1;
};

/** The more chromatic of a title's two artwork accents: the one that reads as its colour. */
export function pickAccent(backdrop?: string, poster?: string): string {
	const pick = chroma(poster) > chroma(backdrop) ? poster : backdrop;
	return parseHex(pick) ? (pick as string) : colors.accent;
}

export function palette(accentHex: string): Palette {
	const base = parseHex(accentHex) ?? (parseHex(colors.accent) as Rgb);
	const inkRgb = ink(base);
	// fills (rails, chips, rings, glows) must read on the near-black stage too: an accent
	// darker than that falls back to its ink
	const rgb = contrast(base, STAGE) >= 3 ? base : inkRgb;
	return {
		accent: toHex(rgb),
		base: toHex(base),
		rgb,
		strong: toHex(mix(rgb, tokens.strongShade)),
		onAccent:
			luminance(rgb) > tokens.luminanceThreshold ? tokens.onAccentDark : tokens.onAccentLight,
		ink: toHex(inkRgb),
		inkRgb,
		dark: toHex(mix(base, -0.82))
	};
}

/** Scope a palette to one subtree, as the app's `accentVars` does for a title page. */
export function applyPalette(root: HTMLElement, p: Palette) {
	root.style.setProperty('--accent', p.accent);
	root.style.setProperty('--accent-rgb', p.rgb.join(' '));
	root.style.setProperty('--accent-strong', p.strong);
	root.style.setProperty('--on-accent', p.onAccent);
	root.style.setProperty('--ink', p.ink);
	root.style.setProperty('--glint-rgb', p.inkRgb.join(' '));
}

export const rgbList = (hex: string) => (parseHex(hex) ?? [0, 0, 0]).join(' ');
