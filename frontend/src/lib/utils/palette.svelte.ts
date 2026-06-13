import { Vibrant } from 'node-vibrant/browser';
import { accentVars } from '$lib/theme';

// Two cache layers so an accent is extracted at most once per image, and so a
// revisited banner paints its colour on the first frame instead of flashing the
// theme default while node-vibrant runs:
//   - an in-memory promise map dedupes concurrent/repeat lookups this session;
//   - localStorage persists the resolved colour across reloads.
// Both are keyed by the full artwork URL; the ?v= token busts them when art is
// replaced.
const STORAGE_PREFIX = 'cv.accent.';

// a plain Map: this is a memo store, not reactive state (reactivity comes from
// the $state accent in the helpers below)
// eslint-disable-next-line svelte/prefer-svelte-reactivity
const cache = new Map<string, Promise<string | null>>();

function storedAccent(url: string): string | null {
	try {
		return localStorage.getItem(STORAGE_PREFIX + url);
	} catch {
		return null;
	}
}

function storeAccent(url: string, hex: string | null) {
	if (!hex) return;
	try {
		localStorage.setItem(STORAGE_PREFIX + url, hex);
	} catch {
		// storage full/unavailable - the in-memory cache still helps this session
	}
}

function quantize(url: string): Promise<string | null> {
	return Vibrant.from(url)
		.getPalette()
		.then((palette) => {
			const swatch = palette.Vibrant ?? palette.LightVibrant ?? palette.Muted ?? palette.LightMuted;
			return swatch?.hex ?? null;
		})
		.catch(() => null); // same-origin artwork, but ignore decode/quantize failures
}

/** Extract a vibrant accent hex from an image URL, or null on failure. Cached per URL. */
export function extractAccent(url: string): Promise<string | null> {
	let pending = cache.get(url);
	if (!pending) {
		pending = quantize(url).then((hex) => {
			storeAccent(url, hex);
			return hex;
		});
		cache.set(url, pending);
	}
	return pending;
}

// read the persisted accent for the current URL, guarding against a getter that
// throws (e.g. before its source is ready) so seeding never breaks init
function seedAccent(url: () => string | null | undefined): string | null {
	try {
		const u = url();
		return u ? storedAccent(u) : null;
	} catch {
		return null;
	}
}

/**
 * Reactive banner accent. Pass a getter for the backdrop image URL and get back
 * a reactive `style` string of accent CSS variables (incl. the contrast text
 * colour) to spread on a subtree. Call at component top level. The accent is
 * seeded synchronously from the persisted cache, so a banner seen before paints
 * its colour immediately with no flash from the theme default.
 */
export function bannerAccent(url: () => string | null | undefined) {
	let accent = $state<string | null>(seedAccent(url));
	$effect(() => {
		const u = url();
		// seed (or reset) synchronously on every URL change, then refine once the
		// fresh extraction resolves - a failed extraction keeps the seeded colour
		accent = u ? storedAccent(u) : null;
		if (!u) return;
		let cancelled = false;
		extractAccent(u).then((hex) => {
			if (!cancelled && hex) accent = hex;
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
 * pays nothing until hovered. Seeds from the persisted cache so a card seen
 * before colours its hover ring instantly. `accent` is null until resolved.
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
			if (!u) return;
			accent = storedAccent(u);
			extractAccent(u).then((hex) => {
				if (hex) accent = hex;
			});
		}
	};
}
