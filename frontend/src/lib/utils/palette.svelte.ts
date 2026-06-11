import { Vibrant } from 'node-vibrant/browser';
import { accentVars } from '$lib/theme';

/** Extract a vibrant accent hex from an image URL, or null on failure. */
export async function extractAccent(url: string): Promise<string | null> {
	try {
		const palette = await Vibrant.from(url).getPalette();
		const swatch = palette.Vibrant ?? palette.LightVibrant ?? palette.Muted ?? palette.LightMuted;
		return swatch?.hex ?? null;
	} catch {
		// same-origin artwork, but ignore decode/quantize failures
		return null;
	}
}

/**
 * Reactive banner accent. Pass a getter for the backdrop image URL and get back
 * a reactive `style` string of accent CSS variables (incl. the contrast text
 * colour) to spread on a subtree. Call at component top level.
 */
export function bannerAccent(url: () => string | null | undefined) {
	let accent = $state<string | null>(null);
	$effect(() => {
		const u = url();
		accent = null;
		if (!u) return;
		let cancelled = false;
		extractAccent(u).then((hex) => {
			if (!cancelled) accent = hex;
		});
		return () => {
			cancelled = true;
		};
	});
	return {
		get style() {
			return accent ? accentVars(accent) : '';
		}
	};
}

/**
 * Lazy per-element accent for hover effects: extracts the colour from an image
 * the first time `load()` is called (e.g. on pointerenter), so a grid of cards
 * pays nothing until hovered. `accent` is null until resolved.
 */
export function hoverAccent(url: () => string | null | undefined) {
	let accent = $state<string | null>(null);
	let started = false;
	return {
		get accent() {
			return accent;
		},
		load() {
			if (started) return;
			started = true;
			const u = url();
			if (u) extractAccent(u).then((hex) => (accent = hex));
		}
	};
}
