// Loading everything a spec needs before its timeline is built: artwork (decoded, with its
// size), beds (decoded at the render rate, with their onsets) and the fonts. Templates build
// only after this, because layout (text fitting, glints) and beat-synced cuts depend on it.

import latinExtUrl from './assets/fonts/plus-jakarta-sans-latin-ext-wght-normal.woff2?url';
import latinUrl from './assets/fonts/plus-jakarta-sans-latin-wght-normal.woff2?url';
import displayExtUrl from './assets/fonts/saira-latin-ext-wght-normal.woff2?url';
import displayUrl from './assets/fonts/saira-latin-wght-normal.woff2?url';
import { SAMPLE_RATE, type LoadedBed, type Onset } from './audio/engine';
import { detectOnsets } from './audio/onsets';
import { placeholderBed } from './audio/placeholder';
import { DISPLAY_FAMILY } from './brand/wordmark';
import type { Picture } from './core/framing';
import { rng } from './core/rng';
import type { Bed, Spec } from './spec';

export const FONT_FAMILY = 'Couch TV Sans';

let fontsReady: Promise<void> | null = null;

const LATIN =
	'U+0000-00FF, U+0131, U+0152-0153, U+02BB-02BC, U+02C6, U+02DA, U+02DC, U+0304, U+0308, U+0329, U+2000-206F, U+20AC, U+2122, U+2191, U+2193, U+2212, U+2215, U+FEFF, U+FFFD';
const LATIN_EXT =
	'U+0100-02BA, U+02BD-02C5, U+02C7-02CC, U+02CE-02D7, U+02DD-02FF, U+0304, U+0308, U+0329, U+1D00-1DBF, U+1E00-1E9F, U+1EF2-1EFF, U+2020, U+20A0-20AB, U+20AD-20C0, U+2113, U+2C60-2C7F, U+A720-A7FF';

/**
 * Register the faces on the document (shadow roots ignore @font-face) and load them: Plus
 * Jakarta Sans for text and Saira, the brand's display face (both OFL, latin and latin-ext;
 * Saira's files are @fontsource-variable/saira 5.3.0's weight axis).
 */
export function ensureFonts(): Promise<void> {
	if (!fontsReady) {
		const face = (family: string, url: string, weight: string, unicodeRange: string) =>
			new FontFace(family, `url(${url}) format('woff2')`, {
				weight,
				display: 'block',
				unicodeRange
			});
		const faces = [
			face(FONT_FAMILY, latinUrl, '200 800', LATIN),
			face(FONT_FAMILY, latinExtUrl, '200 800', LATIN_EXT),
			face(DISPLAY_FAMILY, displayUrl, '100 900', LATIN),
			face(DISPLAY_FAMILY, displayExtUrl, '100 900', LATIN_EXT)
		];
		fontsReady = Promise.all(faces.map((f) => f.load())).then(() => {
			for (const f of faces) document.fonts.add(f);
		});
	}
	return fontsReady;
}

interface CachedPicture extends Picture {
	keep: HTMLImageElement;
}

const pictures = new Map<string, Promise<CachedPicture>>();
const buffers = new Map<string, Promise<AudioBuffer>>();
const blurs = new Map<string, string>();

function loadPicture(url: string, missing: string[]): Promise<CachedPicture> {
	let p = pictures.get(url);
	if (!p) {
		p = (async () => {
			const img = new Image();
			img.decoding = 'sync';
			img.src = url;
			try {
				await img.decode();
				return { url, width: img.naturalWidth, height: img.naturalHeight, keep: img };
			} catch {
				missing.push(url);
				pictures.delete(url);
				return stand();
			}
		})();
		pictures.set(url, p);
	}
	return p;
}

/** A dark gradient that stands in for artwork that failed to load. */
function stand(): CachedPicture {
	const c = document.createElement('canvas');
	c.width = 320;
	c.height = 180;
	const g = c.getContext('2d') as CanvasRenderingContext2D;
	const grad = g.createLinearGradient(0, 0, 320, 180);
	grad.addColorStop(0, '#191c27');
	grad.addColorStop(1, '#07080d');
	g.fillStyle = grad;
	g.fillRect(0, 0, 320, 180);
	const url = c.toDataURL('image/png');
	const keep = new Image();
	keep.src = url;
	return { url, width: 320, height: 180, keep };
}

async function decode(url: string): Promise<AudioBuffer> {
	if (url.startsWith('synth:')) return placeholderBed(url);
	const res = await fetch(url);
	if (!res.ok) throw new Error(`${url}: HTTP ${res.status}`);
	// an offline context decodes without a user gesture, at the render rate
	const ctx = new OfflineAudioContext(2, 1, SAMPLE_RATE);
	return ctx.decodeAudioData(await res.arrayBuffer());
}

function loadBuffer(url: string): Promise<AudioBuffer> {
	let p = buffers.get(url);
	if (!p) {
		p = decode(url);
		p.catch(() => buffers.delete(url));
		buffers.set(url, p);
	}
	return p;
}

/** Every image and bed URL a spec refers to (footage frames load per frame instead). */
export function collectUrls(spec: unknown): { images: string[]; beds: string[] } {
	const images = new Set<string>();
	const beds = new Set<string>();
	const walk = (v: unknown, key: string) => {
		if (Array.isArray(v)) {
			if (key !== 'frames') v.forEach((x) => walk(x, key));
			return;
		}
		if (!v || typeof v !== 'object') {
			// an empty URL is a title without that artwork: its stand-in is no failure
			if (
				v &&
				typeof v === 'string' &&
				(key === 'backdrop' || key === 'poster' || key === 'posters')
			)
				images.add(v);
			return;
		}
		const o = v as Record<string, unknown>;
		if (key === 'bed' && typeof o.url === 'string') beds.add(o.url);
		if (key === 'logo' && typeof o.url === 'string' && o.url) images.add(o.url);
		// accents are colours keyed like the images they come from
		if (key !== 'accents') for (const [k, x] of Object.entries(o)) walk(x, k);
	};
	walk(spec, '');
	return { images: [...images], beds: [...beds] };
}

/** The loaded media of one spec. */
export class Assets {
	readonly missing: string[] = [];
	private readonly pics = new Map<string, CachedPicture>();
	private readonly bufs = new Map<string, AudioBuffer>();
	private readonly onsets = new Map<string, Onset[]>();

	static async load(spec: Spec): Promise<Assets> {
		const a = new Assets();
		const { images, beds } = collectUrls(spec);
		await Promise.all([
			ensureFonts(),
			...images.map(async (u) => a.pics.set(u, await loadPicture(u, a.missing))),
			...beds.map(async (u) => {
				try {
					a.bufs.set(u, await loadBuffer(u));
				} catch (err) {
					console.warn('bed unavailable', u, err);
					a.missing.push(u);
				}
			})
		]);
		await Promise.all([
			...[400, 500, 600, 700, 800].map((w) =>
				document.fonts.load(`${w} 100px "${FONT_FAMILY}"`, 'Aa')
			),
			document.fonts.load(`700 100px "${DISPLAY_FAMILY}"`, 'C()UCH TV 20:15')
		]);
		return a;
	}

	picture(url: string): Picture {
		return this.pics.get(url) ?? stand();
	}

	/** A pre-blurred copy (blurred once on a canvas, never animated). */
	blurred(url: string): string {
		let b = blurs.get(url);
		if (!b) {
			const pic = this.pics.get(url) ?? stand();
			const c = document.createElement('canvas');
			c.width = 384;
			c.height = 216;
			const g = c.getContext('2d') as CanvasRenderingContext2D;
			g.filter = 'blur(9px)';
			g.drawImage(pic.keep, -30, -18, 444, 252);
			b = c.toDataURL('image/jpeg', 0.86);
			blurs.set(url, b);
		}
		return b;
	}

	/** The spec's bed, ready to play, or null when there is none (or it failed to load). */
	bed(bed: Bed | undefined): LoadedBed | null {
		if (!bed) return null;
		const buffer = this.bufs.get(bed.url);
		if (!buffer) return null;
		const start = Math.min(Math.max(0, bed.in), buffer.duration);
		const length = Math.max(0, Math.min(bed.out, buffer.duration) - start);
		const key = `${bed.url}|${start}|${length}`;
		let onsets = this.onsets.get(key);
		if (!onsets) {
			onsets = detectOnsets(
				{
					sampleRate: buffer.sampleRate,
					left: buffer.getChannelData(0),
					right: buffer.getChannelData(buffer.numberOfChannels > 1 ? 1 : 0)
				},
				start,
				length
			);
			this.onsets.set(key, onsets);
		}
		return { buffer, in: start, length, gain: bed.gain ?? 1, onsets };
	}
}

let grain: string | null = null;

/** The film-grain tile: seeded noise, made once. */
export function grainTexture(): string {
	if (!grain) {
		const c = document.createElement('canvas');
		c.width = c.height = 256;
		const g = c.getContext('2d') as CanvasRenderingContext2D;
		const d = g.createImageData(256, 256);
		const r = rng(99);
		for (let i = 0; i < d.data.length; i += 4) {
			const v = Math.round(r() * 255);
			d.data[i] = d.data[i + 1] = d.data[i + 2] = v;
			d.data[i + 3] = 255;
		}
		g.putImageData(d, 0, 0);
		grain = c.toDataURL('image/png');
	}
	return grain;
}
