// "Technical difficulties": the couch alone on the night horizon under the stars, waiting (the
// popcorn has not dropped: the show is not back). A seamless loop, picture and sound: the stars
// twinkle a whole number of times a loop, the dusk behind the horizon breathes once, the pad's
// partials make whole cycles in a loop and its filter opens and closes once with the dusk, so
// the last frame and sample flow into the first. The sonic logo opens each loop on the felt
// piano, softly.

import type { Bus } from '../audio/engine';
import type { Kit } from '../audio/kit';
import { glass, house, piano } from '../audio/voices';
import { hz } from '../audio/pitch';
import { COUCH, COUCH_BOX, LOGO_W } from '../brand/glyph';
import { SONIC } from '../brand/sonic';
import { css, div, span, svg } from '../core/dom';
import { rng } from '../core/rng';
import type { StandbySpec } from '../spec';
import { INK } from './onair';
import { timeline, type Template } from './shared';

/** The horizon's height, and the couch on it (its width, its feet on the line). */
const HORIZON = 700;
const COUCH_W = 320;

export const standby: Template<StandbySpec> = {
	version: 4,
	build({ root, sound }, spec) {
		const D = spec.duration ?? 12;
		const tl = timeline(root, D);
		tl.loop = true;
		root.classList.add('standby');

		// the sky: night, warming toward the horizon, the dusk rising behind it
		const glowEl = div('fill sb-dusk', root);
		tl.add(glowEl, [{ opacity: 0.78 }, { opacity: 1 }, { opacity: 0.78 }], 0, D, 'sine');

		// the stars: seeded squares, each twinkling one to three whole times a loop
		const stars = div('fill', root);
		const r = rng(29);
		for (let i = 0; i < 90; i++) {
			const x = Math.round(r() * 1916);
			const y = Math.round(r() ** 1.6 * (HORIZON - 140));
			const s = r() < 0.7 ? 2 : 3;
			const a = 0.25 + r() * 0.6;
			const star = css(div('abs sb-star', stars), {
				left: `${x}px`,
				top: `${y}px`,
				width: `${s}px`,
				height: `${s}px`
			});
			const k = 1 + Math.floor(r() * 3);
			const dip = 0.25 + r() * 0.5;
			tl.add(
				star,
				[
					{ opacity: a, offset: 0 },
					{ opacity: a * 0.35, offset: dip },
					{ opacity: a, offset: 1 }
				],
				0,
				D / k,
				'sine',
				{ iterations: k }
			);
		}

		// the ground and the line of light on the horizon
		css(div('abs sb-ground', root), { top: `${HORIZON}px` });
		css(div('abs sb-line', root), { top: `${HORIZON - 1}px` });

		// the couch on the horizon, alone
		const [x0, y0, x1, y1] = COUCH_BOX;
		const k = COUCH_W / LOGO_W;
		const w = (x1 - x0) * k;
		const h = (y1 - y0) * k;
		const couch = svg(
			'svg',
			{
				class: 'sb-couch',
				width: w.toFixed(2),
				height: h.toFixed(2),
				viewBox: `${x0} ${y0} ${x1 - x0} ${y1 - y0}`
			},
			root
		);
		couch.setAttribute('aria-hidden', 'true');
		Object.assign(couch.style, {
			left: `${(960 - w / 2).toFixed(2)}px`,
			top: `${(HORIZON - h).toFixed(2)}px`
		});
		svg('path', { d: COUCH, fill: INK.red }, couch);

		// what happened, under the horizon
		const col = div('sb-col', root);
		div('sb-label', col, 'Technical difficulties');
		div('sb-title', col, 'We’ll be right back');
		const sub = div('sb-sub', col);
		const wm = span('sb-wm', sub);
		span(null, wm, 'couch');
		span('tv', wm, 'tv');
		span(null, sub, ' will continue shortly');

		tl.cue('loop', 0);
		tl.cue('the sonic logo', 1.0);
		tl.sound(0, (when, b) => loopPad(sound, b, when, D), D);
		tl.sound(1.0, (when, b) => softLogo(sound, b, when));
		return tl;
	}
};

/**
 * The loop's pad: D2 A2 D3 F#3 A3 on detuned saws whose every partial makes a whole number of
 * cycles in the loop, under a low-pass that opens and closes once a loop (the dusk's breath):
 * the loop's end meets its start in phase.
 */
function loopPad(k: Kit, b: Bus, when: number, period: number) {
	const h = house(k, b);
	const ctx = k.ctx;
	const snap = (f: number) => Math.round(f * period) / period;
	const lp = k.filter('lowpass', 520, 0.6);
	// -cos: the filter is closed as the loop starts and widest half way through
	const lfo = k.osc(b, 'sine', 1 / period, when, period);
	lfo.setPeriodicWave(ctx.createPeriodicWave([0, -1], [0, 0], { disableNormalization: true }));
	lfo.connect(k.gain(420)).connect(lp.frequency);
	lp.frequency.value = 940;
	const level = k.gain(0.05);
	lp.connect(k.filter('highpass', 60, 0.6))
		.connect(level)
		.connect(h.out);
	level.connect(k.gain(0.5)).connect(h.hall);
	(['D2', 'A2', 'D3', 'F#3', 'A3'] as const).forEach((note, i) => {
		const f = hz(note);
		const p = k.pan([-0.1, 0.25, -0.3, 0.35, -0.2][i]);
		p.connect(lp);
		for (const cents of [-7, 0, 6]) {
			const o = k.osc(b, 'sawtooth', snap(f * 2 ** (cents / 1200)), when, period);
			o.connect(k.gain(i < 2 ? 0.45 : 0.3)).connect(p);
		}
	});
}

/** The sonic logo on the felt piano, softly, glass an octave over its notes like a halo. */
function softLogo(k: Kit, b: Bus, when: number) {
	const h = house(k, b);
	SONIC.notes.forEach((name, i) => {
		const t = when + SONIC.at[i];
		const last = i === SONIC.notes.length - 1;
		piano(h, t, name, {
			vel: 0.42 + 0.04 * i,
			dur: last ? 1.8 : 1.1,
			pan: -0.1 + 0.1 * i,
			verb: 0.7,
			gain: 0.85,
			seed: 700 + i
		});
		glass(h, t + 0.01, hz(name) * 2, {
			attack: 0.03,
			tone: 0.7,
			decay: last ? 2.2 : 1.6,
			gain: 0.05,
			pan: 0.15,
			verb: 0.8,
			seed: 710 + i
		});
	});
}
