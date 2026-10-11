// A promo shot on the timeline: a still (artwork crop with a slow move) or footage (25 fps
// frames), with an optional dissolve in and a punch-in for hard cuts.

import type { Assets } from '../assets';
import { css, div, img } from '../core/dom';
import { framed, inset } from '../core/framing';
import type { Shot, Title } from '../spec';
import type { TL } from './shared';

export interface ShotOptions {
	/** Seconds of cross-dissolve centred on the cut (0: a hard cut). */
	dissolve?: number;
	/** A quick zoom settle on the cut (grit). */
	punch?: boolean;
	opacity?: number;
}

export function addShot(
	tl: TL,
	parent: Element,
	title: Title,
	assets: Assets,
	shot: Shot,
	a: number,
	b: number,
	{ dissolve = 0, punch = false, opacity }: ShotOptions = {}
) {
	const inT = a > 0 && dissolve ? a - dissolve / 2 : a;
	const outT = b + (dissolve ? dissolve / 2 : 0);
	let box: HTMLElement;
	if (shot.kind === 'footage') {
		box = div('shot footage', parent);
		const frameImg = img(null, box);
		if (shot.focus) {
			const [x, y, w, h] = shot.focus;
			frameImg.style.objectPosition = `${((x + w / 2) * 100).toFixed(1)}% ${((y + h / 2) * 100).toFixed(1)}%`;
		}
		tl.footage(frameImg, shot.frames, inT);
		if (punch) {
			tl.add(frameImg, [{ transform: 'scale(1.16)' }, { transform: 'scale(1)' }], a, 0.24, 'snap');
			tl.add(
				frameImg,
				[{ transform: 'scale(1)' }, { transform: 'scale(1.03)' }],
				a + 0.24,
				Math.max(0.3, b - a - 0.24),
				'linear'
			);
		} else {
			tl.add(
				frameImg,
				[{ transform: 'scale(1)' }, { transform: 'scale(1.045)' }],
				inT,
				outT - inT,
				'linear'
			);
		}
	} else {
		const pic = assets.picture(shot.image === 'poster' ? title.poster : title.backdrop);
		const s = framed(parent, pic);
		box = s.el;
		const to = shot.to ?? inset(shot.from, 0.9);
		if (punch) {
			tl.add(
				s.img,
				[{ transform: s.tf(shot.from, 1.16) }, { transform: s.tf(shot.from) }],
				a,
				0.24,
				'snap'
			);
			tl.add(
				s.img,
				[{ transform: s.tf(shot.from) }, { transform: s.tf(to) }],
				a + 0.24,
				Math.max(0.3, b - a - 0.24),
				'linear'
			);
		} else {
			tl.add(
				s.img,
				[{ transform: s.tf(shot.from) }, { transform: s.tf(to) }],
				inT,
				outT - inT,
				'linear'
			);
		}
	}
	if (opacity != null) css(box, { opacity: String(opacity) });
	tl.show(box, inT, outT + 0.001);
	if (dissolve && a > 0)
		tl.add(box, [{ opacity: 0 }, { opacity: opacity ?? 1 }], inT, dissolve, 'sine');
	return box;
}
