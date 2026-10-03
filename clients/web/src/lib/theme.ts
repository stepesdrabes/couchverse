// Runtime accent theming. An accent is a single colour; the strong and soft variants are
// derived from it so the whole palette shifts together. The site accent comes from the core's
// session; accents scoped to a subtree (a title's banner) are still derived here, the same way.

import type { AccentPalette } from '$lib/generated/core';
import { accent as tokens, colors } from '$lib/generated/tokens';

function clampByte(n: number): number {
	return Math.max(0, Math.min(255, Math.round(n)));
}

function parseHex(hex: string): [number, number, number] | null {
	const m = /^#?([0-9a-f]{6})$/i.exec(hex.trim());
	if (!m) return null;
	const int = parseInt(m[1], 16);
	return [(int >> 16) & 255, (int >> 8) & 255, int & 255];
}

function toHex(...bytes: number[]): string {
	return '#' + bytes.map((v) => clampByte(v).toString(16).padStart(2, '0')).join('');
}

/** mix toward black (amount < 0) or white (amount > 0) */
function mix(rgb: [number, number, number], amount: number): [number, number, number] {
	const target = amount < 0 ? 0 : 255;
	const t = Math.abs(amount);
	return rgb.map((c) => clampByte(c + (target - c) * t)) as [number, number, number];
}

function shade(rgb: [number, number, number], amount: number): string {
	return toHex(...mix(rgb, amount));
}

// WCAG relative luminance of an sRGB colour (0 = black, 1 = white).
function luminance([r, g, b]: [number, number, number]): number {
	const lin = (c: number) => {
		const s = c / 255;
		return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
	};
	return 0.2126 * lin(r) + 0.7152 * lin(g) + 0.0722 * lin(b);
}

function contrast(a: [number, number, number], b: [number, number, number]): number {
	const [la, lb] = [luminance(a), luminance(b)];
	return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
}

/** The accent as text on the app's surfaces: lightened in 1% steps until it reaches the
 * tokens' contrast ratio on the lightest one, as the core derives it. */
function ink(rgb: [number, number, number]): string {
	const surface = parseHex(colors['surface-2']) ?? [0, 0, 0];
	for (let step = 0; step <= 100; step++) {
		const c = mix(rgb, step / 100);
		if (contrast(c, surface) >= tokens.inkContrast) return toHex(...c);
	}
	return '#ffffff';
}

/** Pick the readable foreground (near-white or near-black) for text on `accent`. */
export function readableTextOn(accent: string): string {
	const rgb = parseHex(accent);
	if (!rgb) return tokens.onAccentLight;
	return luminance(rgb) > tokens.luminanceThreshold ? tokens.onAccentDark : tokens.onAccentLight;
}

/** The palette for `accent`, as the core derives the site's; null for an unreadable colour. */
export function palette(accent: string): AccentPalette | null {
	const rgb = parseHex(accent);
	if (!rgb) return null;
	return {
		accent: toHex(...rgb),
		strong: shade(rgb, tokens.strongShade),
		// soft tint over the dark background - low-alpha accent
		soft: toHex(...rgb, tokens.softAlpha * 255),
		onAccent: readableTextOn(accent),
		ink: ink(rgb)
	};
}

/** Apply a palette as the site accent: the palette CSS variables on :root. */
export function applyPalette(p: AccentPalette) {
	const root = document.documentElement.style;
	root.setProperty('--color-accent', p.accent);
	root.setProperty('--color-accent-strong', p.strong);
	root.setProperty('--color-accent-soft', p.soft);
	root.setProperty('--color-on-accent', p.onAccent);
	root.setProperty('--color-accent-ink', p.ink);
	// the site accent, never overridden by a scoped accent (e.g. the player's
	// banner accent), so app-wide chrome like the couch can keep the site colour
	root.setProperty('--color-site-accent', p.accent);
}

/**
 * The same accent palette as an inline `style` string, so a subtree (e.g. a
 * title page accented by its banner) can override the accent without touching
 * :root. Includes the contrast-aware text colour. Returns '' for a bad colour.
 */
export function accentVars(accent: string): string {
	const p = palette(accent);
	return p ? paletteVars(p) : '';
}

/** `accentVars` for a palette the core already derived (a title page's). */
export function paletteVars(p: AccentPalette): string {
	return (
		`--color-accent:${p.accent};` +
		`--color-accent-strong:${p.strong};` +
		`--color-accent-soft:${p.soft};` +
		`--color-on-accent:${p.onAccent};` +
		`--color-accent-ink:${p.ink}`
	);
}
