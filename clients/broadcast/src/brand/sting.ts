// A sting: the gap filler in the brand's language, any whole number of frames from 2 to 6 s.
// The mark opens, the lockup composes on the mnemonic, the hold absorbs the rest.

import { div } from '../core/dom';
import { SPRING } from '../core/motion';
import { frame } from '../core/timeline';
import type { StingSpec } from '../spec';
import { glow, timeline, type Template } from '../templates/shared';
import { lockup, springFrames } from './lockup';
import { LOGO, themeBells, themeTag } from './sonic';

export const sting: Template<StingSpec> = {
	version: 1,
	build({ root, sound }, spec) {
		const D = frame(Math.min(6, Math.max(2, spec.duration)));
		const tl = timeline(root, D);
		root.classList.add('sting');
		const short = D < 3;
		const at = short ? 0.12 : 0.3;

		const light = glow(root, 'red', 960, 540, 900);
		const center = div('center', root);
		const lk = lockup(center, short ? 120 : 132);

		tl.add(
			light,
			[
				{ opacity: 0, transform: 'scale(0.8)' },
				{ opacity: 0.7, transform: 'scale(1)' }
			],
			0,
			at + 0.6,
			'out'
		);
		tl.add(light, [{ opacity: 0.7 }, { opacity: 0.45 }], at + 0.6, D - at - 0.6, 'sine');
		tl.cue('the mark', at);
		tl.add(
			lk.mark,
			[
				{ clipPath: 'inset(50% 50% 50% 50% round 30%)' },
				{ clipPath: 'inset(0% 0% 0% 0% round 30%)' }
			],
			at,
			0.3,
			'snap'
		);
		const pop = springFrames(1.08, 1, SPRING.snappy);
		tl.add(lk.mark, pop.frames, at, pop.dur, 'linear');
		const nameAt = at + LOGO.tagRhythm[2];
		tl.cue('the name', nameAt);
		tl.add(
			lk.name,
			[
				{ opacity: 0, transform: 'translateX(-24px)' },
				{ opacity: 1, transform: 'translateX(0)' }
			],
			nameAt,
			0.7,
			'enter'
		);
		tl.add(lk.root, [{ transform: 'scale(1)' }, { transform: 'scale(1.025)' }], at, D - at, 'sine');
		tl.add(center, [{ opacity: 1 }, { opacity: 0 }], D - 0.45, 0.45, 'exit');
		tl.add(light, [{ opacity: 0.45 }, { opacity: 0 }], D - 0.5, 0.5, 'sine');

		tl.sound(at, (w, b) =>
			short ? themeBells(sound, b, w, { gain: 0.5 }) : themeTag(sound, b, w, { gain: 0.5 })
		);
		tl.sound(at, (w, b) =>
			sound.whoosh(b, w, { dur: 0.5, pan: [-0.3, 0.3], gain: 0.03, lo: 600, hi: 3600, body: 0.2 })
		);
		return tl;
	}
};
