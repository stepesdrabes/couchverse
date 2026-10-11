// "Technical difficulties": a calm card that loops seamlessly (every motion has the loop's
// period and ends where it starts; the drone's partials are whole cycles per loop) until the
// next airing.

import { CHANNEL, mark } from '../brand/lockup';
import { themeBells } from '../brand/sonic';
import { div } from '../core/dom';
import type { StandbySpec } from '../spec';
import { glow, timeline, type Template } from './shared';

export const standby: Template<StandbySpec> = {
	version: 1,
	build({ root, sound }, spec) {
		const D = spec.duration ?? 12;
		const tl = timeline(root, D);
		tl.loop = true;
		root.classList.add('standby');

		const orbit = (el: HTMLElement, r: number, phase: number) => {
			const frames: Keyframe[] = [];
			for (let i = 0; i <= 24; i++) {
				const a = phase + (i / 24) * Math.PI * 2;
				frames.push({
					transform: `translate(${(Math.cos(a) * r).toFixed(1)}px, ${(Math.sin(a) * r * 0.6).toFixed(1)}px)`
				});
			}
			tl.add(el, frames, 0, D, 'linear');
		};
		orbit(glow(root, 'violet', 620, 380, 760), 120, 0);
		orbit(glow(root, 'teal', 1300, 760, 700), 140, Math.PI);
		orbit(glow(root, 'red', 960, 540, 520), 60, Math.PI / 2);

		const sweep = div('sb-line', root);
		const band = div('sb-band', sweep);
		tl.add(
			band,
			[{ transform: 'translateX(-360px)' }, { transform: 'translateX(1920px)' }],
			0,
			D,
			'inOut'
		);

		const col = div('sb-col', root);
		const m = mark(col, 132);
		tl.add(
			m,
			[{ transform: 'scale(1)' }, { transform: 'scale(1.035)' }, { transform: 'scale(1)' }],
			0,
			D / 2,
			'sine',
			{
				iterations: 2
			}
		);
		div('eyebrow sb-eyebrow', col, 'Technical difficulties');
		div('hero sb-title', col, 'We’ll be right back');
		div('sb-sub', col, `${CHANNEL} will continue shortly`);

		tl.cue('loop', 0);
		tl.sound(0, (w, b) => sound.loopDrone(b, w, D, D, { gain: 0.06 }), D);
		tl.sound(1.0, (w, b) => themeBells(sound, b, w, { gain: 0.22 }));
		return tl;
	}
};
