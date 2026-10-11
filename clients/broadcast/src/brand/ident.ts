// The channel ident, full (11.5 s) and short (5 s): a placeholder until the remade ident lands
// (a horizon line, the mark irising open on the hit, the lockup composing on the mnemonic). One
// cue table per cut drives picture and score alike.

import { css, div, fadeUp, span } from '../core/dom';
import { SPRING, spring } from '../core/motion';
import type { IdentSpec } from '../spec';
import { timeline, type Template } from '../templates/shared';
import { springFrames } from './lockup';
import { LOGO, themeScore, type IdentCut } from './sonic';

interface Cut extends IdentCut {
	horizon: [number, number] | null;
	tag: number;
}

const CUTS: Record<IdentSpec['cut'], Cut> = {
	full: {
		D: 11.52,
		horizon: [0.3, 1.1],
		swell: 0,
		b1: 1.1,
		b2: 2.6,
		ost: [1.1, 4.6],
		drums: [
			[2.1, 0.26],
			[3.1, 0.3],
			[3.6, 0.34],
			[3.85, 0.28],
			[4.1, 0.38],
			[4.225, 0.28],
			[4.35, 0.34],
			[4.425, 0.28],
			[4.5, 0.36]
		],
		riser: 3.1,
		hit: 4.6,
		mn: 5.25,
		final: 6.15,
		tag: 7.0
	},
	short: {
		D: 5.0,
		horizon: null,
		swell: 0,
		b1: null,
		b2: null,
		ost: null,
		drums: [
			[0.35, 0.26],
			[0.55, 0.28],
			[0.68, 0.28],
			[0.76, 0.3],
			[0.83, 0.32],
			[0.89, 0.34]
		],
		riser: 0.02,
		hit: 0.95,
		mn: 1.42,
		final: 2.32,
		tag: 2.75
	}
};

export const ident: Template<IdentSpec> = {
	version: 1,
	build({ root, sound }, spec) {
		const c = CUTS[spec.cut === 'short' ? 'short' : 'full'];
		const { D, hit: H, final: F, mn: M } = c;
		const R = LOGO.rhythm;
		const tl = timeline(root, D);
		root.classList.add('ident');

		const light = div('id-light', root);
		const horizon = css(div('horizon', root), { top: '540px', opacity: '0.32' });
		const lock = div('id-plane', root);
		const markWrap = div('id-markwrap', lock);
		const square = div('id-mark', markWrap);
		const couch = span('id-couch', square);
		const row = div('idl-row', lock);
		const slot = div('idl-slot', row);
		const word = span('idl-word', div('idl-mask', row), 'Couch');
		const tv = span('idl-word idl-tv', div('idl-mask idl-tvmask', row), 'TV');
		const tag = div('idl-tag', lock, 'One channel. Every couch.');
		const slotX = slot.offsetLeft + 90;

		if (c.horizon) {
			tl.cue('horizon line', c.horizon[0]);
			tl.add(
				horizon,
				[{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }],
				c.horizon[0],
				c.horizon[1] - c.horizon[0] + 0.4,
				'enter'
			);
		}
		tl.add(
			horizon,
			[
				{ transform: 'scaleX(1)', opacity: 0.32 },
				{ transform: 'scaleX(0.06)', opacity: 0 }
			],
			H - 0.9,
			0.85,
			'in'
		);
		tl.add(
			light,
			[
				{ opacity: 0, transform: 'scale(0.8)' },
				{ opacity: 0.38, transform: 'scale(0.88)' }
			],
			0,
			H,
			'in'
		);
		tl.add(
			light,
			[
				{ opacity: 0.38, transform: 'scale(0.88)' },
				{ opacity: 1, transform: 'scale(1)' }
			],
			H,
			0.4,
			'out'
		);
		tl.add(light, [{ opacity: 1 }, { opacity: 0.55 }], H + 0.4, D - H - 0.4, 'sine');

		tl.cue('hit: the mark', H);
		tl.add(
			square,
			[
				{ clipPath: 'inset(50% 50% 50% 50% round 58px)' },
				{ clipPath: 'inset(0% 0% 0% 0% round 58px)' }
			],
			H,
			0.34,
			'snap'
		);
		const pop = springFrames(1.07, 1, SPRING.snappy);
		tl.add(square, pop.frames, H, pop.dur, 'linear');
		tl.add(
			couch,
			[
				{ clipPath: 'inset(100% 0% 0% 0%)', transform: 'translateY(16px)' },
				{ clipPath: 'inset(0% 0% 0% 0%)', transform: 'translateY(0px)' }
			],
			H + 0.1,
			0.65,
			'enter'
		);

		tl.cue('mnemonic: the mark steps aside', M);
		const glide = spring(SPRING.smooth);
		const dx = slotX - 960;
		const k = 180 / 230;
		tl.add(
			markWrap,
			glide.values.map((v) => ({
				transform: `translateX(${(dx * v).toFixed(1)}px) scale(${(1 + (k - 1) * v).toFixed(4)})`
			})),
			M,
			glide.duration,
			'linear'
		);
		tl.add(
			word,
			[
				{ transform: 'translateX(-34%) scale(1.04)', opacity: 0 },
				{ transform: 'translateX(0%) scale(1)', opacity: 1 }
			],
			M + R[1],
			1.0,
			'enter'
		);
		tl.cue('final chord: TV lands', F);
		const land = springFrames(1, 0, SPRING.snappy, (v) => `translateY(${(v * 130).toFixed(1)}%)`);
		tl.add(tv, land.frames, F, land.dur, 'linear');
		tl.cue('tagline, hold', c.tag);
		fadeUp(tl, tag, c.tag, { dur: 1.1, y: 10 });
		tl.add(lock, [{ transform: 'scale(1)' }, { transform: 'scale(1.02)' }], M, D - M, 'sine');

		for (const ev of themeScore(sound, c)) tl.sound(ev.at, ev.fn, ev.dur);
		return tl;
	}
};
