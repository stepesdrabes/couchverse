// The frame pipeline. A frame is drawn into a float16 canvas, several times across the shutter
// when the picture moves, and the passes are averaged in linear light (motion blur, a 180 degree
// shutter). The result is dithered down to 8 bits, so the dark gradients never band. In the live
// viewer (no `hq`) a frame is drawn once, straight to the screen.

import { rng } from '../../../core/rng';
import { collectGarbage } from '../gc';

export const W = 1920;
export const H = 1080;
const FPS = 25;

export interface Shot {
	draw(ctx: CanvasRenderingContext2D, t: number): void;
	/** How far the fastest visible point travels over one frame at t, in pixels. */
	motion(t: number): number;
}

/** The value of an IEEE half float from its 16 bits. */
function half(bits: number): number {
	const s = bits & 0x8000 ? -1 : 1;
	const e = (bits >> 10) & 0x1f;
	const m = bits & 0x3ff;
	if (e === 0) return s * m * 2 ** -24;
	if (e === 31) return m ? NaN : s * Infinity;
	return s * (1 + m / 1024) * 2 ** (e - 15);
}

const toLinear = (v: number) => (v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4);
const toSrgb = (v: number) => (v <= 0.0031308 ? v * 12.92 : 1.055 * v ** (1 / 2.4) - 0.055);

const ENC = 16384;
const TILE = 256;

export class Film {
	private readonly out: CanvasRenderingContext2D;
	private readonly ctx: CanvasRenderingContext2D;
	private readonly acc = new Float32Array(W * H * 3);
	private readonly lin = new Float32Array(65536);
	private readonly enc = new Float32Array(ENC + 1);
	private readonly dither = new Float32Array(TILE * TILE);
	private readonly img: ImageData;

	constructor(
		canvas: HTMLCanvasElement,
		private readonly shot: Shot,
		private readonly shutter = 0.5
	) {
		this.out = canvas.getContext('2d') as CanvasRenderingContext2D;
		const work = document.createElement('canvas');
		work.width = W;
		work.height = H;
		this.ctx = work.getContext('2d', {
			colorType: 'float16',
			willReadFrequently: true
		} as CanvasRenderingContext2DSettings) as CanvasRenderingContext2D;
		for (let b = 0; b < 65536; b++) {
			const v = half(b);
			this.lin[b] = Number.isFinite(v) ? toLinear(Math.min(1, Math.max(0, v))) : 0;
		}
		for (let i = 0; i <= ENC; i++) this.enc[i] = toSrgb(i / ENC) * 255;
		// triangular dither of one 8-bit step, from a seeded tile
		const r = rng(907);
		for (let i = 0; i < this.dither.length; i++) this.dither[i] = r() + r() - 1;
		this.img = this.out.createImageData(W, H);
	}

	/** Passes for the frame at t: enough that no pass is more than ~1.5 px from the next. */
	samples(t: number) {
		const blur = this.shot.motion(t) * this.shutter;
		return Math.max(1, Math.min(64, Math.ceil(blur / 1.5)));
	}

	frame(t: number, hq: boolean) {
		if (!hq) {
			this.out.setTransform(1, 0, 0, 1, 0, 0);
			this.shot.draw(this.out, t);
			return;
		}
		const n = this.samples(t);
		const acc = this.acc;
		acc.fill(0);
		for (let k = 0; k < n; k++) {
			const tk = n === 1 ? t : t + (this.shutter / FPS) * ((k + 0.5) / n - 0.5);
			this.ctx.setTransform(1, 0, 0, 1, 0, 0);
			this.ctx.globalAlpha = 1;
			this.ctx.globalCompositeOperation = 'source-over';
			this.ctx.filter = 'none';
			this.shot.draw(this.ctx, tk);
			const data = this.ctx.getImageData(0, 0, W, H, {
				pixelFormat: 'rgba-float16'
			} as ImageDataSettings).data as unknown as { buffer: ArrayBuffer; byteOffset: number };
			const bits = new Uint16Array(data.buffer, data.byteOffset, W * H * 4);
			const lin = this.lin;
			for (let i = 0, j = 0; i < bits.length; i += 4, j += 3) {
				acc[j] += lin[bits[i]];
				acc[j + 1] += lin[bits[i + 1]];
				acc[j + 2] += lin[bits[i + 2]];
			}
			collectGarbage(bits.byteLength);
		}
		const px = this.img.data;
		const enc = this.enc;
		const d = this.dither;
		const inv = 1 / n;
		// a different dither every frame, so it reads as the faintest grain rather than a pattern
		const frame = Math.round(t * FPS);
		const ox = (frame * 97) % TILE;
		const oy = (frame * 61) % TILE;
		for (let y = 0; y < H; y++) {
			const row = ((y + oy) % TILE) * TILE;
			for (let x = 0; x < W; x++) {
				const p = y * W + x;
				const j = p * 3;
				const q = p * 4;
				const n0 = row + ((x + ox) % TILE);
				const n1 = row + ((x + ox + 85) % TILE);
				const n2 = row + ((x + ox + 170) % TILE);
				for (let c = 0; c < 3; c++) {
					const v = Math.min(1, Math.max(0, acc[j + c] * inv));
					const e = enc[Math.round(v * ENC)] + d[c === 0 ? n0 : c === 1 ? n1 : n2];
					// the clamped array rounds to the nearest step
					px[q + c] = e;
				}
				px[q + 3] = 255;
			}
		}
		this.out.putImageData(this.img, 0, 0);
	}
}
