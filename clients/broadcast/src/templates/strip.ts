// A strip promo ("Every weekday at 18:00"): the time, then the days light up on the beat.

import { beatsIn, snapper } from '../audio/onsets';
import { SPARKLE } from '../audio/kit';
import { channelTag } from '../brand/lockup';
import { chars, css, div, el, fadeUp, rise, span, words } from '../core/dom';
import { fitText } from '../core/fit';
import { framed } from '../core/framing';
import { applyPalette } from '../core/palette';
import type { StripPromoSpec } from '../spec';
import { glow, outToBlack, timeline, titlePalette, type Template } from './shared';

const DAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
const DAY_NAMES = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];

export const stripPromo: Template<StripPromoSpec> = {
	version: 3,
	build({ root, sound, assets }, spec) {
		const t = spec.title;
		const D = spec.duration ?? 16;
		const dark = spec.mood === 'dark';
		const tl = timeline(root, D);
		applyPalette(root, titlePalette(t));
		root.classList.add('strip', dark ? 'mood-dark' : 'mood-bright');
		const bed = assets.bed(spec.bed);
		const onsets = bed?.onsets ?? [];

		const gA = glow(root, 'accent', 380, 300, 760);
		const gV = glow(root, dark ? 'red' : 'violet', 300, 1000, 640);
		const panel = div('sp-panel', root);
		const shot = framed(panel, assets.picture(t.backdrop), { w: 1180, h: 1080 });
		const fr = t.crops?.strip ?? [
			[0.3, 0, 0.7, 1],
			[0.36, 0.05, 0.6, 0.86]
		];
		tl.add(
			shot.img,
			[{ transform: shot.tf(fr[0]) }, { transform: shot.tf(fr[1]) }],
			0,
			D,
			'cubic-bezier(0.3, 0.2, 0.6, 1)'
		);
		if (dark) {
			shot.img.style.filter = 'grayscale(0.45) contrast(1.15) brightness(0.62)';
			css(div('fill', panel), {
				background:
					'radial-gradient(ellipse 70% 80% at 65% 45%, transparent 30%, rgb(10 3 5 / 0.8) 100%), linear-gradient(0deg, rgb(70 4 8 / 0.35), rgb(70 4 8 / 0.35))'
			});
		} else {
			css(div('fill', panel), {
				background:
					'linear-gradient(0deg, rgb(7 8 13 / 0.6), transparent 42%), linear-gradient(90deg, rgb(7 8 13 / 0.35), transparent 60%)'
			});
		}

		const col = div('sp-col', root);
		const ebRow = div('un-eyebrow', col);
		const rail = css(div('rail', ebRow), { width: '56px' });
		const eb = div('eyebrow', ebRow, spec.airing.eyebrow);
		const title = words(div('hero sp-title', col), t.name);
		fitText(title.root.parentElement as HTMLElement, { width: 960, min: 76, lines: 3 });
		const timeWrap = div('sp-time tnum', col);
		const tc = chars(span('mask', timeWrap), spec.airing.time);
		const daysRow = div('sp-days', col);
		const lit = new Set(spec.airing.days);
		const chips = DAYS.map((name, d) => {
			const c = div('day', daysRow);
			const on = div('on', c);
			span(null, c, name);
			const lo = span('lbl-on', c, name);
			if (dark) on.style.transformOrigin = '50% 0%';
			return { c, on, lo, d, lit: lit.has(d) };
		});
		let foot: HTMLDivElement | null = null;
		let countEl: HTMLElement | null = null;
		const f = spec.foot;
		if (f?.kind === 'count') {
			foot = div('sp-count', col);
			countEl = el('b', null, foot, '0');
			foot.append(' episodes, in order');
		} else if (f?.kind === 'from') {
			foot = div('sp-count', col);
			foot.append(`Season ${f.season} from `);
			el('b', 'ink', foot, DAY_NAMES[f.day]);
		}

		// open
		tl.add(gA, [{ opacity: 0 }, { opacity: 1 }], 0, 1.6, 'sine');
		tl.add(gV, [{ opacity: 0 }, { opacity: 0.8 }], 0.2, 1.6, 'sine');
		tl.add(
			panel,
			[
				{ opacity: 0, transform: 'translateX(120px)' },
				{ opacity: 1, transform: 'translateX(0)' }
			],
			0,
			1.4,
			'enter'
		);
		tl.cue('eyebrow', 0.45);
		tl.add(rail, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], 0.45, 0.7, 'enter');
		tl.add(
			eb,
			[
				{ opacity: 0, transform: 'translateX(-14px)' },
				{ opacity: 1, transform: 'translateX(0)' }
			],
			0.55,
			0.8,
			'enter'
		);
		tl.cue('title', 0.75);
		rise(tl, title.items, 0.75, { stagger: 0.09, dur: 1.0 });
		const timeAt = snapper(onsets)(1.6, 0.4, 0.3);
		tl.cue('time', timeAt);
		rise(tl, tc.items, timeAt, { stagger: 0.06, dur: 0.85, from: '100%', fade: false });
		tl.sound(timeAt, (w, b) =>
			sound.impact(b, w, { gain: dark ? 0.3 : 0.2, size: 0.8, tone: dark ? 40 : 50, click: 0.4 })
		);

		// days: appear, then light up on the music's beats
		const daysAt = timeAt + 0.9;
		chips.forEach((ch, i) => fadeUp(tl, ch.c, daysAt + i * 0.05, { y: 14, dur: 0.6 }));
		const on = chips.filter((c) => c.lit);
		const beats = beatsIn(onsets, daysAt + 0.5, D, dark ? 0.42 : 0.25, 0.3, 0);
		let prev = daysAt + 0.4;
		const litTimes = on.map(() => {
			const tt =
				beats.find((x) => x > prev + (dark ? 0.4 : 0.22) && x < prev + 0.9) ??
				prev + (dark ? 0.5 : 0.32);
			prev = tt;
			return tt;
		});
		if (litTimes.length) tl.cue('days light up', litTimes[0]);
		on.forEach((ch, i) => {
			const at = litTimes[i];
			tl.add(
				ch.on,
				[{ transform: 'scaleY(0)' }, { transform: 'scaleY(1)' }],
				at,
				dark ? 0.5 : 0.32,
				dark ? 'cubic-bezier(0.5, 0, 0.2, 1)' : 'pop'
			);
			tl.add(ch.lo, [{ opacity: 0 }, { opacity: 1 }], at + 0.04, 0.2, 'linear');
			tl.add(
				ch.c,
				[{ transform: 'scale(1)' }, { transform: 'scale(1.1)' }, { transform: 'scale(1)' }],
				at,
				0.36,
				'out'
			);
			tl.sound(at, (w, b) =>
				sound.tick(b, w, {
					freq: dark ? SPARKLE[[0, 2, 1, 3, 2, 4, 0][i % 7]] / 2 : SPARKLE[i % SPARKLE.length],
					gain: dark ? 0.06 : 0.05,
					pan: -0.6 + i * 0.12,
					decay: dark ? 0.16 : 0.07,
					type: dark ? 'triangle' : 'sine'
				})
			);
		});
		const afterDays = (litTimes[litTimes.length - 1] ?? daysAt) + 0.55;

		if (f?.kind === 'from') {
			const startChip = chips[f.day];
			const ring = div('start-ring', startChip.c);
			startChip.c.style.overflow = 'visible';
			startChip.on.style.borderRadius = '18px';
			tl.add(
				ring,
				[
					{ opacity: 0, transform: 'scale(1.3)' },
					{ opacity: 1, transform: 'scale(1)' }
				],
				afterDays,
				0.6,
				'enter'
			);
			tl.add(
				ring,
				[{ opacity: 1 }, { opacity: 0.35 }, { opacity: 1 }],
				afterDays + 0.6,
				1.4,
				'sine',
				{
					iterations: Math.max(1, Math.ceil((D - afterDays) / 1.4))
				}
			);
			tl.sound(afterDays, (w, b) =>
				sound.tick(b, w, { freq: SPARKLE[0] / 2, gain: 0.06, decay: 0.25, type: 'triangle' })
			);
		}
		if (foot) {
			tl.cue(f?.kind === 'count' ? 'episode counter' : 'season line', afterDays + 0.15);
			fadeUp(tl, foot, afterDays + 0.15, { y: 14 });
			if (f?.kind === 'count' && countEl) {
				const n = f.count;
				const c0 = afterDays + 0.2;
				const cd = 1.5;
				tl.text(countEl, (tt) => {
					const p = Math.min(1, Math.max(0, (tt - c0) / cd));
					return String(Math.round(n * (1 - Math.pow(1 - p, 3))));
				});
				// a rolling counter: ticks that slow down with the ease-out
				for (let i = 0; i < 12; i++) {
					const at = c0 + cd * (1 - Math.pow(1 - i / 12, 1 / 3)) * 0.98;
					tl.sound(at, (w, b) =>
						sound.tick(b, w, { freq: 3200 - i * 90, gain: 0.018, decay: 0.02, pan: 0.3 })
					);
				}
			}
		}

		const tagAt = D - 4.0;
		channelTag(tl, root, tagAt, sound);
		outToBlack(tl, root, 0.8);
		tl.sound(
			0,
			(w, b, skip) =>
				sound.bed(b, w, bed, skip, {
					fadeIn: 0.3,
					fadeOut: 2.2,
					end: D,
					gain: 1,
					ducks: [{ at: tagAt, depth: 0.45, hold: 1.4, attack: 0.25 }]
				}),
			D
		);
		tl.sound(0, (w, b) => sound.whoosh(b, w, { dur: 1.2, pan: [0.8, 0.2], gain: 0.045 }));
		return tl;
	}
};
