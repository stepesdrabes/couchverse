// A short promo with comic energy: the picture rushes in to the bed's first hit, the name slams,
// the airing pops, the channel tag closes.

import { channelTag } from '../brand/lockup';
import { css, div, span } from '../core/dom';
import { fitText } from '../core/fit';
import { framed } from '../core/framing';
import { applyPalette } from '../core/palette';
import type { ShortPromoSpec } from '../spec';
import { grain, rgbOf, shake, timeline, titlePalette, type Template } from './shared';

export const shortPromo: Template<ShortPromoSpec> = {
	version: 3,
	build({ root, sound, assets }, spec) {
		const t = spec.title;
		const D = spec.duration ?? 9;
		const tl = timeline(root, D);
		const pal = titlePalette(t);
		applyPalette(root, pal);
		const backdropAccent = t.accents.backdrop;
		const deep = !backdropAccent || pal.base === backdropAccent ? pal.dark : backdropAccent;
		root.style.setProperty('--shadow-rgb', rgbOf(deep));
		root.classList.add('short');
		const bed = assets.bed(spec.bed);

		// the hit: the strongest onset in the bed's first two seconds
		const hit =
			(bed?.onsets ?? []).filter((o) => o.t > 0.5 && o.t < 2.2).sort((a, b) => b.s - a.s)[0]?.t ??
			1.1;

		const world = div('fill', root);
		const shot = framed(world, assets.picture(t.backdrop));
		const fr = t.crops?.short ?? [
			[0.18, 0.02, 0.66, 0.86],
			[0.26, 0.08, 0.52, 0.66]
		];
		css(div('fill', world), {
			background:
				'linear-gradient(0deg, rgb(7 8 13 / 0.75), rgb(7 8 13 / 0.35) 50%, rgb(7 8 13 / 0.6))'
		});
		const rays = div('rays', world);
		const htA = div('halftone', world);
		const htB = div('halftone tl', world);
		const burst = div('burst', world);
		const c = div('center', world);
		const stack = css(div(null, c), {
			display: 'flex',
			flexDirection: 'column',
			alignItems: 'center'
		});
		const whenRow = div('sh-when', stack);
		const ebMask = span('mask', whenRow);
		const eb = span(
			'eyebrow',
			ebMask,
			spec.airing.day ? `${spec.airing.eyebrow} · ${spec.airing.day}` : spec.airing.eyebrow
		);
		const timeP = span('sh-time tnum', whenRow, spec.airing.time);
		const word = div('sh-word', stack, t.name);
		// under the airing, clear of the channel tag
		fitText(word, { width: 1680, min: 180, lines: 3, height: 640 });
		const flash = div('flash', root);
		grain(tl, root, 0.05, 17);

		tl.add(
			shot.img,
			[{ transform: shot.tf(fr[0], 1.5) }, { transform: shot.tf(fr[0], 1.12) }],
			0,
			hit,
			'cubic-bezier(0.55, 0, 1, 0.45)'
		);
		tl.add(
			shot.img,
			[{ transform: shot.tf(fr[0], 1.12) }, { transform: shot.tf(fr[0], 1.04) }],
			hit,
			0.25,
			'snap'
		);
		tl.add(
			shot.img,
			[{ transform: shot.tf(fr[0], 1.04) }, { transform: shot.tf(fr[1]) }],
			hit + 0.25,
			D - hit - 0.25,
			'out'
		);
		tl.add(shot.el, [{ opacity: 0 }, { opacity: 1 }], 0, 0.5, 'sine');
		tl.add(
			rays,
			[
				{ transform: 'rotate(0deg) scale(0.8)', opacity: 0 },
				{ transform: 'rotate(12deg) scale(1)', opacity: 1 }
			],
			0,
			hit,
			'linear'
		);
		tl.add(
			rays,
			[
				{ transform: 'rotate(12deg) scale(1)', opacity: 1 },
				{ transform: 'rotate(40deg) scale(1.08)', opacity: 0.7 }
			],
			hit,
			D - hit,
			'out'
		);

		tl.cue('the hit', hit);
		tl.add(flash, [{ opacity: 0.9 }, { opacity: 0 }], hit, 0.16, 'linear', { fill: 'forwards' });
		tl.add(
			word,
			[
				{ opacity: 0, transform: 'scale(2.3)' },
				{ opacity: 1, transform: 'scale(1)' }
			],
			hit,
			0.24,
			'snap'
		);
		shake(tl, world, hit, { amp: 26, dur: 0.5, seed: 77 });
		tl.add(
			burst,
			[
				{ opacity: 1, transform: 'scale(0.35)' },
				{ opacity: 0, transform: 'scale(1.7)' }
			],
			hit,
			0.8,
			'out'
		);
		tl.add(htA, [{ opacity: 0 }, { opacity: 1 }], hit, 0.25, 'linear');
		tl.add(htB, [{ opacity: 0 }, { opacity: 0.8 }], hit + 0.05, 0.25, 'linear');

		const wAt = hit + 0.85;
		tl.cue('when', wAt);
		tl.add(
			eb,
			[{ transform: 'translateY(110%)' }, { transform: 'translateY(0)' }],
			wAt,
			0.5,
			'snap'
		);
		tl.add(
			timeP,
			[
				{ opacity: 0, transform: 'scale(0.4) rotate(-6deg)' },
				{ opacity: 1, transform: 'scale(1) rotate(0deg)' }
			],
			wAt + 0.22,
			0.42,
			'pop'
		);
		tl.sound(wAt, (w, b) =>
			sound.whoosh(b, w, { dur: 0.45, pan: [-0.6, 0], gain: 0.05, lo: 700, hi: 4200, body: 0.2 })
		);
		tl.sound(wAt + 0.22, (w, b) => sound.punch(b, w, { gain: 0.25, pan: 0.2 }));

		const tagAt = Math.min(D - 3.3, hit + 3.6);
		tl.add(
			stack,
			[{ transform: 'translateY(0) scale(1)' }, { transform: 'translateY(-60px) scale(0.9)' }],
			tagAt - 0.5,
			0.9,
			'inOut'
		);
		const tag = channelTag(tl, root, tagAt, sound);
		tl.add(world, [{ opacity: 1 }, { opacity: 0 }], D - 0.55, 0.55, 'exit');
		tl.add(stack, [{ opacity: 1 }, { opacity: 0 }], D - 0.6, 0.4, 'exit');
		tl.add(tag, [{ opacity: 1 }, { opacity: 0 }], D - 0.45, 0.45, 'exit');

		tl.sound(
			0,
			(w, b, skip) =>
				sound.bed(b, w, bed, skip, {
					fadeIn: 0.02,
					fadeOut: 1.6,
					end: D,
					gain: 1,
					ducks: [{ at: tagAt, depth: 0.55, hold: 1.2, attack: 0.2 }]
				}),
			D
		);
		tl.sound(0.1, (w, b) => sound.riser(b, w, { dur: hit - 0.1, gain: 0.05, tone: 0.3 }));
		tl.sound(hit, (w, b) => sound.impact(b, w, { gain: 0.38, size: 1.3, tone: 42 }));
		return tl;
	}
};
