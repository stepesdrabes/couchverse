// "Journey": the Couchverse couch floats through one unbroken dream panorama (dusk dunes running
// down to the sea, the sun setting into it behind the couch, a city lighting up on the far
// shore), the camera cranes up into the stars and they go out like a cinema's lights, leaving
// the couch alone on black. A bucket of popcorn drops onto it and the two become the logo;
// couchtv types itself in on the sonic logo's three notes.
//
// The world is drawn on a canvas, every frame a pure function of t, with a 180-degree shutter:
// each frame averages sub-frame renders in linear light, so the parallax and the popcorn's fall
// blur like film instead of strobing.

import { FONT_FAMILY } from '../../../assets';
import { css as style, div, span } from '../../../core/dom';
import { edge, windows } from '../../../core/moves';
import { frame } from '../../../core/timeline';
import { timeline, type Ctx, type TL } from '../../../templates/shared';
import { collectGarbage } from '../gc';
import { toSrgb } from './color';
import { CUES, D, DROP, LAND, LOCK, NOTES } from './plan';
import { score } from './score';
import { drawWorld, GIVE, H, W } from './world';

const CSS = `
.jr { position: absolute; inset: 0; overflow: hidden; background: #050505; }
.jr canvas { position: absolute; left: 0; top: 0; width: 1920px; height: 1080px; }
.jr-word { position: absolute; left: 0; width: 1920px; display: flex; justify-content: center;
  font-family: '${FONT_FAMILY}'; font-weight: 800; letter-spacing: -0.035em; line-height: 1;
  color: #fff; white-space: nowrap; }
.jr-word span { display: inline-block; }
.jr-word .tv { color: #e50914; }
`;

/** 180 degrees: the shutter is open half of each 1/25 s frame. */
const SHUTTER = 0.5 / 25;
/** After the rebound and the frame's give, nothing in the canvas moves again. */
const STILL = LAND + GIVE + 0.04;

/** Sub-frames per frame: enough that the fastest motion's blur reads as one smooth streak. */
function samples(t: number) {
	if (t >= STILL) return 1;
	if (t > DROP - 0.05 && t < LAND + 0.12) return 28;
	return 14;
}

const LIN = new Float32Array(256).map((_, i) => {
	const v = i / 255;
	return v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
});
const ENC_N = 1 << 16;
const ENC = new Float32Array(ENC_N + 1).map((_, i) => toSrgb(i / ENC_N));

let dither: Float32Array | null = null;

/** A fixed triangular dither (+-1 LSB), so averaged gradients never band; made on first use. */
function ditherTable(): Float32Array {
	if (!dither) {
		const n = W * H;
		const d = new Float32Array(n);
		let s = 0x9e3779b9;
		const r = () => {
			s ^= s << 13;
			s ^= s >>> 17;
			s ^= s << 5;
			return (s >>> 0) / 4294967296;
		};
		for (let i = 0; i < n; i++) d[i] = r() - r();
		dither = d;
	}
	return dither;
}

/**
 * Draws the frame at t: a render (`hq`) averages the shutter's sub-frames, live playback draws
 * one sample straight onto the visible canvas.
 */
function painter(canvas: HTMLCanvasElement) {
	const vis = canvas.getContext('2d') as CanvasRenderingContext2D;
	const DITHER = ditherTable();
	const work = document.createElement('canvas');
	work.width = W;
	work.height = H;
	const off = work.getContext('2d', { willReadFrequently: true }) as CanvasRenderingContext2D;
	const acc = new Float32Array(W * H * 3);
	const out = vis.createImageData(W, H);
	// every frame goes through the same CPU canvas and the same average: the visible canvas may
	// be rasterized on the GPU, whose anti-aliased edges differ by a hair, which would show as a
	// one-frame change in the logo's edges where the blur stops
	return (t: number, hq: boolean) => {
		if (!hq) {
			drawWorld(vis, t);
			return;
		}
		const n = samples(t);
		acc.fill(0);
		for (let i = 0; i < n; i++) {
			drawWorld(off, t + ((i + 0.5) / n - 0.5) * SHUTTER);
			const d = off.getImageData(0, 0, W, H).data;
			for (let p = 0, q = 0; p < d.length; p += 4, q += 3) {
				acc[q] += LIN[d[p]];
				acc[q + 1] += LIN[d[p + 1]];
				acc[q + 2] += LIN[d[p + 2]];
			}
			collectGarbage(d.byteLength);
		}
		const o = out.data;
		const k = ENC_N / n;
		for (let p = 0, q = 0, i = 0; p < o.length; p += 4, q += 3, i++) {
			const dz = DITHER[i];
			o[p] = ENC[Math.min(ENC_N, Math.round(acc[q] * k))] + dz;
			o[p + 1] = ENC[Math.min(ENC_N, Math.round(acc[q + 1] * k))] + dz;
			o[p + 2] = ENC[Math.min(ENC_N, Math.round(acc[q + 2] * k))] + dz;
			o[p + 3] = 255;
		}
		vis.putImageData(out, 0, 0);
	};
}

export function journey({ root, sound }: Ctx): TL {
	const dur = frame(D);
	const tl = timeline(root, dur);
	const css = document.createElement('style');
	css.textContent = CSS;
	root.appendChild(css);
	const scene = div('jr', root);
	const canvas = document.createElement('canvas');
	canvas.width = W;
	canvas.height = H;
	scene.appendChild(canvas);
	const paint = painter(canvas);
	// the canvas changes every frame until it holds still; the value is the frame's time
	tl.dynamic(
		(t) => Math.min(Math.round(t * 1e4) / 1e4, STILL),
		(v, wait) => paint(Number(v), wait !== null)
	);

	// couchtv: couch on the first note of the sonic logo, t and v on the next two
	const word = style(div('jr-word', scene), {
		top: `${LOCK.wordY}px`,
		fontSize: `${LOCK.word}px`
	});
	const parts = [span(null, word, 'couch'), span('tv', word, 't'), span('tv', word, 'v')];
	parts.forEach((e, i) => {
		const t = NOTES[i];
		windows(tl, e, [[t, dur + 1]]);
		tl.add(
			e,
			[
				{ transform: `translateY(${i ? 34 : 40}px) scale(${i ? 1.2 : 1})`, opacity: 0 },
				{ transform: 'translateY(0px) scale(1)', opacity: 1 }
			],
			edge(t),
			i ? 0.26 : 0.34,
			i ? 'snap' : 'out'
		);
	});
	for (const [name, t] of CUES) tl.cue(name, t);

	// one cue for the whole score, built before an offline render starts (as the approved render
	// built it): its voices start slow oscillators up to a period (7.7 s) early, while a render
	// schedules a cue only shortly before it plays. Live playback from part-way skips what is
	// over, as a cue of its own would be.
	const events = score(sound, dur);
	tl.sound(
		0,
		(w, b, skip) => {
			for (const e of events)
				if (e.at >= skip - 0.03 || skip < e.at + e.dur) e.fn(w - skip + e.at, b);
		},
		dur
	);
	return tl;
}
