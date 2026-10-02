// Runtime accent theming. The admin picks a single accent colour; the strong
// and soft variants are derived from it so the whole palette shifts together.

import { accent as tokens } from '$lib/generated/tokens';

function clampByte(n: number): number {
	return Math.max(0, Math.min(255, Math.round(n)));
}

function parseHex(hex: string): [number, number, number] | null {
	const m = /^#?([0-9a-f]{6})$/i.exec(hex.trim());
	if (!m) return null;
	const int = parseInt(m[1], 16);
	return [(int >> 16) & 255, (int >> 8) & 255, int & 255];
}

function toHex(r: number, g: number, b: number): string {
	return '#' + [r, g, b].map((v) => clampByte(v).toString(16).padStart(2, '0')).join('');
}

/** mix toward black (amount < 0) or white (amount > 0) */
function shade(rgb: [number, number, number], amount: number): string {
	const target = amount < 0 ? 0 : 255;
	const t = Math.abs(amount);
	return toHex(...(rgb.map((c) => c + (target - c) * t) as [number, number, number]));
}

// WCAG relative luminance of an sRGB colour (0 = black, 1 = white).
function luminance([r, g, b]: [number, number, number]): number {
	const lin = (c: number) => {
		const s = c / 255;
		return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
	};
	return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

/** Pick the readable foreground (near-white or near-black) for text on `accent`. */
export function readableTextOn(accent: string): string {
	const rgb = parseHex(accent);
	if (!rgb) return tokens.onAccentLight;
	return luminance(rgb) > tokens.luminanceThreshold ? tokens.onAccentDark : tokens.onAccentLight;
}

/** Apply an accent colour by setting the palette CSS variables on :root. */
export function applyAccent(accent: string) {
	const rgb = parseHex(accent);
	if (!rgb) return;
	const root = document.documentElement.style;
	root.setProperty('--color-accent', toHex(...rgb));
	root.setProperty('--color-accent-strong', shade(rgb, tokens.strongShade));
	// soft tint over the dark background - low-alpha accent
	root.setProperty(
		'--color-accent-soft',
		`rgba(${rgb[0]}, ${rgb[1]}, ${rgb[2]}, ${tokens.softAlpha})`
	);
	root.setProperty('--color-on-accent', readableTextOn(accent));
	// the site accent, never overridden by a scoped accent (e.g. the player's
	// banner accent), so app-wide chrome like the couch can keep the site colour
	root.setProperty('--color-site-accent', toHex(...rgb));
}

/**
 * The same accent palette as an inline `style` string, so a subtree (e.g. a
 * title page accented by its banner) can override the accent without touching
 * :root. Includes the contrast-aware text colour. Returns '' for a bad colour.
 */
export function accentVars(accent: string): string {
	const rgb = parseHex(accent);
	if (!rgb) return '';
	return (
		`--color-accent:${toHex(...rgb)};` +
		`--color-accent-strong:${shade(rgb, tokens.strongShade)};` +
		`--color-accent-soft:rgba(${rgb[0]}, ${rgb[1]}, ${rgb[2]}, ${tokens.softAlpha});` +
		`--color-on-accent:${readableTextOn(accent)}`
	);
}
