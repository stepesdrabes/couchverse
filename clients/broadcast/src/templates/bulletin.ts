// What the bulletins share: a quiet screen (static glows, no grain, so held frames render as
// one capture), the header, the credit line and a soft bed that is over before the clip ends.
// The bulletins air in a slot of their own, so nothing of theirs may ring past their last frame.

import { SPARKLE } from '../audio/kit';
import type { Kit } from '../audio/kit';
import { div, fadeUp, rise, words } from '../core/dom';
import { glow, outToBlack, type TL } from './shared';

/** The header: the eyebrow, the title and the time the data is of, right-aligned. */
export function bulletinHead(
	tl: TL,
	root: HTMLElement,
	eyebrow: string,
	title: string,
	asOf: string
) {
	const head = div('bl-head', root);
	const left = div(null, head);
	const eb = div('eyebrow', left, eyebrow);
	const hero = words(div('hero bl-title', left), title);
	const time = div('bl-asof tnum', head, `as of ${asOf}`);
	fadeUp(tl, eb, 0.15, { y: 10 });
	rise(tl, hero.items, 0.25, { stagger: 0.08, dur: 0.8 });
	fadeUp(tl, time, 0.45, { y: 10 });
	return head;
}

/** The backdrop: two glows that settle in and stay still. */
export function bulletinBackdrop(tl: TL, root: HTMLElement) {
	const glows = [glow(root, 'red', 260, 160, 620), glow(root, 'violet', 1700, 980, 760)];
	for (const g of glows)
		tl.add(
			g,
			[
				{ opacity: 0, transform: 'scale(0.9)' },
				{ opacity: 1, transform: 'scale(1)' }
			],
			0,
			1.2,
			'out'
		);
}

/** A credit line along the bottom edge. */
export function credit(tl: TL, parent: HTMLElement, text: string, at = 0.6) {
	const c = div('bl-credit', parent, text);
	fadeUp(tl, c, at, { y: 8 });
	return c;
}

/** The ending: black over the last half second, which the next part of the break cuts from. */
export function bulletinOut(tl: TL, root: HTMLElement) {
	outToBlack(tl, root, 0.5);
}

/**
 * The bed: the kit's ambient chords, over 1.5 s before the clip ends so the hall has rung
 * out by its last frame, and a soft tick at each of `ticks`.
 */
export function bulletinSound(tl: TL, sound: Kit, ticks: number[]) {
	const D = tl.duration;
	tl.sound(0, (w, b, skip) => sound.ambient(b, w, D - 1.5, skip, { gain: 0.07 }), D - 1.5);
	ticks.forEach((at, i) =>
		tl.sound(at, (w, b) =>
			sound.tick(b, w, {
				freq: SPARKLE[(i * 2) % SPARKLE.length],
				gain: 0.03,
				pan: 0.2,
				decay: 0.08
			})
		)
	);
}
