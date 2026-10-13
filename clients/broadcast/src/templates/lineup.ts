// "Later on Couch TV": the rows slide in, then each program gets the big panel in turn.

import { SPARKLE } from '../audio/kit';
import { CHANNEL, lockup } from '../brand/lockup';
import { css, div, el, fadeOut, img, rise, words } from '../core/dom';
import { fitText } from '../core/fit';
import { framed } from '../core/framing';
import { applyPalette } from '../core/palette';
import type { LineupSpec } from '../spec';
import { glow, timeline, titlePalette, type Template } from './shared';

export const lineup: Template<LineupSpec> = {
	version: 4,
	build({ root, sound, assets }, spec) {
		const D = spec.duration ?? 12;
		const tl = timeline(root, D);
		root.classList.add('lineup');
		const items = spec.items.slice(0, 5);

		const gV = glow(root, 'violet', 280, 180, 760);
		const gT = glow(root, 'teal', 760, 1100, 700);
		const gR = glow(root, 'red', 1500, 300, 700);
		const panel = div('lu-panel', root);
		const shots = items.map((it) => {
			const s = framed(panel, assets.picture(it.title.backdrop), { w: 1100, h: 1080 });
			s.el.style.background = 'transparent';
			return s;
		});
		div('shade', panel);
		const head = div('lu-head', root);
		const logo = lockup(head, 48, 'lu-wm');
		const title = words(div('hero lu-title', head), `Later on ${CHANNEL}`);
		const list = div('lu-list', root);
		// five rows start higher, so the last one clears the bottom of the frame
		if (items.length > 4) list.classList.add('rows-5');
		const hl = css(div('abs lu-highlight', list), {});
		const rows = items.map((it) => {
			const r = div('row', list);
			applyPalette(r, titlePalette(it.title));
			div('row-bg', r);
			div('row-time tnum', r, it.time);
			const pw = div('row-poster', r);
			img(null, pw, assets.picture(it.title.poster).url);
			const tx = div('row-text', r);
			const name = div('row-name', tx, it.title.name);
			div('row-sub', tx, it.sub);
			let pill: HTMLElement | null = null;
			let fill: HTMLElement | null = null;
			if (it.state === 'live') {
				div('row-bar', r);
				const pr = div('row-prog', tx);
				fill = el('i', null, pr);
				pill = div('pill-live', r);
				el('span', 'live-dot', pill);
				el('span', null, pill, 'Live');
			} else if (it.state === 'next') {
				pill = div('pill-next', r, it.badge ?? 'Next');
			}
			// beside the pill, and within the row's height with what sits under the name
			fitText(name, {
				width: tx.clientWidth,
				min: 28,
				lines: 2,
				leading: 1.1,
				height: 124 - (tx.offsetHeight - name.offsetHeight)
			});
			return { r, pw, pill, fill, it };
		});
		const spot = div('lu-spot', root);
		const captions = items.map((it) => {
			const cpt = css(div('abs', spot), { right: '0', bottom: '0', textAlign: 'right' });
			div('eyebrow', cpt, it.time);
			// right of the rows
			fitText(div('hero lu-spot-name', cpt, it.title.name), { width: 580, min: 40, lines: 3 });
			return cpt;
		});

		// open
		for (const g of [gV, gT, gR])
			tl.add(
				g,
				[
					{ opacity: 0, transform: 'scale(0.85)' },
					{ opacity: 1, transform: 'scale(1)' }
				],
				0,
				1.8,
				'out'
			);
		tl.cue('header', 0.3);
		tl.add(
			logo,
			[
				{ opacity: 0, transform: 'translateY(14px)' },
				{ opacity: 1, transform: 'translateY(0)' }
			],
			0.3,
			0.9,
			'enter'
		);
		rise(tl, title.items, 0.5, { stagger: 0.09, dur: 1.0 });
		tl.add(panel, [{ opacity: 0 }, { opacity: 1 }], 0.4, 1.4, 'sine');

		const rowAt = 1.0;
		tl.cue('rows slide in', rowAt);
		rows.forEach((row, i) => {
			const at = rowAt + i * 0.22;
			tl.add(
				row.r,
				[
					{ opacity: 0, transform: 'translateX(90px)' },
					{ opacity: 1, transform: 'translateX(0)' }
				],
				at,
				0.9,
				'enter'
			);
			tl.add(row.pw, [{ transform: 'scale(1.15)' }, { transform: 'scale(1)' }], at, 1.1, 'enter');
			tl.sound(at, (w, b) =>
				sound.tick(b, w, {
					freq: SPARKLE[[0, 2, 3, 5][i % 4]],
					gain: 0.035,
					pan: 0.3 - i * 0.1,
					decay: 0.08
				})
			);
			if (row.pill) {
				const pa = rowAt + 1.3 + i * 0.05;
				tl.add(
					row.pill,
					[
						{ opacity: 0, transform: 'scale(0.6)' },
						{ opacity: 1, transform: 'scale(1)' }
					],
					pa,
					0.5,
					'pop'
				);
				tl.sound(pa, (w, b) => sound.tick(b, w, { freq: SPARKLE[4], gain: 0.04, pan: 0.5 }));
				const dot = row.pill.querySelector('.live-dot');
				if (dot)
					tl.add(dot, [{ opacity: 1 }, { opacity: 0.3 }, { opacity: 1 }], pa + 0.5, 1.4, 'sine', {
						iterations: Math.ceil(D / 1.4)
					});
			}
			if (row.fill) {
				const p = Math.max(0, Math.min(1, row.it.progress ?? 0));
				tl.add(
					row.fill,
					[{ transform: 'scaleX(0)' }, { transform: `scaleX(${p.toFixed(3)})` }],
					rowAt + 1.5,
					1.2,
					'enter'
				);
			}
		});

		// the spotlight: each program in turn on the big panel
		const s0 = 3.3;
		const step = Math.min(1.75, (D - 1.6 - s0) / items.length);
		tl.cue('spotlight', s0);
		tl.add(
			hl,
			[
				{ opacity: 0, transform: 'translateY(0)' },
				{ opacity: 1, transform: 'translateY(0)' }
			],
			s0 - 0.2,
			0.5,
			'sine'
		);
		items.forEach((it, i) => {
			const at = s0 + i * step;
			const nextAt = i + 1 < items.length ? s0 + (i + 1) * step : D;
			const sh = shots[i];
			tl.show(sh.el, i === 0 ? 0 : at - 0.6, i + 1 < items.length ? nextAt + 0.2 : D + 1);
			if (i > 0) tl.add(sh.el, [{ opacity: 0 }, { opacity: 1 }], at - 0.6, 0.8, 'sine');
			const fr = it.title.crops?.lineup ?? [0.1, 0.05, 0.85, 0.9];
			tl.add(
				sh.img,
				[{ transform: sh.tf(fr, 1.08) }, { transform: sh.tf(fr, 1) }],
				i === 0 ? 0 : at - 0.6,
				nextAt - at + 1.4,
				'cubic-bezier(0.2, 0.3, 0.4, 1)'
			);
			if (i > 0) {
				tl.add(
					hl,
					[
						{ transform: `translateY(${(i - 1) * 152}px)` },
						{ transform: `translateY(${i * 152}px)` }
					],
					at,
					0.6,
					'cubic-bezier(0.6, 0, 0.2, 1)'
				);
				tl.sound(at, (w, b) =>
					sound.tick(b, w, {
						freq: SPARKLE[(i + 1) % SPARKLE.length],
						gain: 0.025,
						pan: 0.2,
						decay: 0.12
					})
				);
			}
			const cp = captions[i];
			tl.add(
				cp,
				[
					{ opacity: 0, transform: 'translateY(20px)' },
					{ opacity: 1, transform: 'translateY(0)' }
				],
				at + 0.1,
				0.7,
				'enter'
			);
			if (i + 1 < items.length) fadeOut(tl, cp, nextAt - 0.25, { dur: 0.3, y: -14 });
		});

		// out
		const out = D - 1.2;
		tl.cue('out', out);
		rows.forEach((row, i) =>
			tl.add(
				row.r,
				[
					{ opacity: 1, transform: 'translateX(0)' },
					{ opacity: 0, transform: 'translateX(-50px)' }
				],
				out + i * 0.06,
				0.45,
				'exit'
			)
		);
		fadeOut(tl, head, out, { dur: 0.5 });
		fadeOut(tl, hl, out, { dur: 0.4, y: 0 });
		fadeOut(tl, spot, out + 0.1, { dur: 0.4, y: 0 });
		tl.add(panel, [{ opacity: 1 }, { opacity: 0 }], out + 0.2, 0.9, 'sine');
		for (const g of [gV, gT, gR])
			tl.add(g, [{ opacity: 1 }, { opacity: 0 }], out + 0.2, 1.0, 'sine');

		tl.sound(0, (w, b, skip) => sound.ambient(b, w, D, skip, { gain: 0.085 }), D);
		tl.sound(0.15, (w, b) => sound.shimmer(b, w, { dur: 1.4, gain: 0.018, count: 8, seed: 31 }));
		return tl;
	}
};
