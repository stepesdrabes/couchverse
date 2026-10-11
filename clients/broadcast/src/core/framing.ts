// Artwork framing: an image laid out to cover a view, moved by transforms only.

import type { Crop } from '../spec';
import { css, div, img } from './dom';

export const W = 1920;
export const H = 1080;

/** An image the assets loader has fetched and decoded. */
export interface Picture {
	url: string;
	width: number;
	height: number;
}

export interface Framed {
	el: HTMLDivElement;
	img: HTMLImageElement;
	/** The transform that fills the view with `crop` (clamped so no edge shows), times `zoom`. */
	tf: (crop?: Crop, zoom?: number) => string;
}

export function framed(parent: Element, pic: Picture, view = { w: W, h: H }, cls = 'shot'): Framed {
	const box = div(cls, parent);
	const image = img(null, box, pic.url);
	const nw = pic.width || 16;
	const nh = pic.height || 9;
	const c = Math.max(view.w / nw, view.h / nh);
	const bw = nw * c;
	const bh = nh * c;
	css(image, { width: bw + 'px', height: bh + 'px' });
	const tf = (r: Crop = [0, 0, 1, 1], zoom = 1) => {
		const [x, y, w, h] = r;
		const k = Math.max(view.w / (w * bw), view.h / (h * bh), 1) * zoom;
		let tx = view.w / 2 - (x + w / 2) * bw * k;
		let ty = view.h / 2 - (y + h / 2) * bh * k;
		tx = Math.min(0, Math.max(view.w - bw * k, tx));
		ty = Math.min(0, Math.max(view.h - bh * k, ty));
		return `translate(${tx.toFixed(1)}px, ${ty.toFixed(1)}px) scale(${k.toFixed(4)})`;
	};
	image.style.transform = tf();
	return { el: box, img: image, tf };
}

/** Shrink a crop around its centre (a push-in target for a slow move). */
export function inset(r: Crop, f = 0.9, dx = 0, dy = 0): Crop {
	const [x, y, w, h] = r;
	return [x + (w * (1 - f)) / 2 + dx, y + (h * (1 - f)) / 2 + dy, w * f, h * f];
}
