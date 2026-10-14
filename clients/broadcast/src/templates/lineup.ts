// "Later on couchtv": the evening as a list on the night field, the program in the spotlight
// lighting the room from the right with scenes from itself. The rows rise in along their
// hairlines, then the spotlight walks down them: a red marker moves to the next row, it lights
// up as the last one dims, and its scenes dissolve in as they slide into place, a camera settling
// on it. Between its moves only the scenes move (none without footage: held frames are free).
//
// Score: the house's D major 9 pad, the felt piano in slow eighths over it, a glass note on each
// step of the spotlight climbing the scale, ending on an open fifth that rings out under the
// rows lifting away, so the sound is over before up next starts.

import type { Bus } from '../audio/engine';
import type { Kit } from '../audio/kit';
import { glass, house, pad, padSweep, piano, sub } from '../audio/voices';
import { END, ringRoom, sonicRingOut } from '../brand/sonic';
import { css, div, span, words } from '../core/dom';
import { fitText } from '../core/fit';
import type { LineupSpec } from '../spec';
import { byLines, conceal, grade, INK, masked, meta, night, reveal, scenes } from './onair';
import { timeline, type Template } from './shared';

/** The list: its left edge, width and first row's top (five rows start higher). */
const LIST = { x: 96, w: 1000, top: 292, top5: 252, foot: 1000, time: 172 };
/** The house tempo: a beat is 20 frames. */
const BEAT = 0.8;

export const lineup: Template<LineupSpec> = {
	version: 7,
	build({ root, sound, assets }, spec) {
		const D = spec.duration ?? 12;
		const tl = timeline(root, D);
		root.classList.add('lineup');
		const items = spec.items.slice(0, 5);
		const n = items.length;
		const five = n > 4;

		// when the spotlight lands on each row: on half beats, the last one held long enough to read
		const s0 = 1.6;
		const out = D - 1.0;
		const even = (out - s0) / n;
		const step = Math.min(Math.round(even / (BEAT / 2)) * (BEAT / 2), (out - s0 - 1.6) / (n - 1));
		const lands = items.map((_, i) => s0 + i * step);

		// the spotlight's pictures: each row's scenes, from the moment it is lit until covered
		const pics = div('fill oa-view', root);
		const shots = items.map((it, i) => {
			const from = i === 0 ? 0 : lands[i] - 0.1;
			const to = i + 1 < n ? lands[i + 1] + 0.8 : D;
			const slide = div('fill lu-pic', pics);
			scenes(tl, slide, it.title, assets, {
				start: from,
				end: to,
				crop: it.title.crops?.lineup ?? [0.04, 0.04, 0.92, 0.92]
			});
			tl.show(slide, from, to + 0.001);
			return slide;
		});
		grade(root, [
			`linear-gradient(90deg, ${night(0.97)} 0%, ${night(0.94)} 36%, ${night(0.55)} 58%, ${night(0.1)} 80%, ${night(0)} 100%)`,
			`linear-gradient(0deg, ${night(0.7)} 0%, ${night(0)} 30%)`,
			`linear-gradient(180deg, ${night(0.6)} 0%, ${night(0)} 26%)`
		]);
		const dark = css(div('fill', root), { background: INK.night });

		// the header: Later on couchtv, the wordmark as its last word
		const head = css(div('abs lu-head', root), { left: `${LIST.x}px`, top: '112px' });
		const later = masked(head, 'lu-word', 'Later');
		const on = masked(head, 'lu-word', 'on');
		const wm = masked(head, 'lu-word lu-wm');
		span(null, wm, 'couch');
		span('tv', wm, 'tv');

		// the rows, each as tall as its name and lines need
		const list = css(div('abs lu-list', root), {
			left: `${LIST.x}px`,
			top: '0',
			width: `${LIST.w}px`
		});
		if (five) list.classList.add('five');
		const rows = items.map((it) => {
			const r = div('lu-row', list);
			const rule = div('lu-rule', r);
			const time = masked(r, 'lu-time tnum', it.time);
			(time.parentElement as HTMLElement).classList.add('lu-time-mask');
			if (it.state === 'live') time.classList.add('now');
			const tx = css(div('lu-text', r), { marginLeft: `${LIST.time}px` });
			let tag: HTMLElement | null = null;
			if (it.state === 'live') {
				tag = div('lu-live', div('oa-mask lu-tag live', r));
				span('lu-live-dot', tag);
				span(null, tag, 'Live');
			} else if (it.state === 'next' && it.badge) {
				tag = meta(div('oa-mask lu-tag next', r), 'lu-badge', it.badge.split(' · '));
			}
			const name = div('lu-name', tx);
			const w = words(name, it.title.name);
			// beside the tag, two lines at most
			fitText(name, {
				width: LIST.w - LIST.time - (tag ? tag.offsetWidth + 28 : 0),
				min: 30,
				lines: 2,
				leading: 1.04
			});
			const subEl = meta(div('oa-mask lu-sub-mask', tx), 'lu-sub', it.sub.split(' · '));
			fitText(subEl, { width: LIST.w - LIST.time, min: 20 });
			let fill: HTMLElement | null = null;
			let bar: HTMLElement | null = null;
			if (it.state === 'live') {
				bar = div('lu-bar', tx);
				fill = div('lu-bar-fill', bar);
			}
			return { r, rule, time, name, lines: byLines(name, w.items), sub: subEl, tag, bar, fill, it };
		});
		const last = div('lu-rule lu-last', list);
		// the list hangs from under the header, or sits on the foot when it is tall
		const listTop = Math.min(five ? LIST.top5 : LIST.top, LIST.foot - list.offsetHeight);
		list.style.top = `${listTop}px`;
		const tops = rows.map((row) => row.r.offsetTop);
		const heights = rows.map((row) => row.r.offsetHeight);

		// the spotlight's marker: a red bar left of the row it lights
		const marker = div('abs lu-marker', list);
		const at = (i: number) =>
			`translateY(${tops[i] + 18}px) scaleY(${((heights[i] - 36) / 100).toFixed(4)})`;

		// --- in ----------------------------------------------------------------------------------
		tl.add(dark, [{ opacity: 1 }, { opacity: 0 }], 0, 0.8, 'sine');
		tl.cue('header', 0.24);
		reveal(tl, [[later], [on], [wm]], 0.24, { stagger: 0.07, dur: 0.9 });
		tl.cue('rows', 0.56);
		rows.forEach((row, i) => {
			const a = 0.56 + i * 0.08;
			tl.add(
				row.rule,
				[{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }],
				a - 0.1,
				0.8,
				'glide'
			);
			reveal(tl, [[row.time], ...row.lines], a, { stagger: 0.06, dur: 0.85 });
			reveal(tl, [[row.sub]], a + 0.1, { dur: 0.85 });
			if (row.tag) reveal(tl, [[row.tag]], 1.3 + i * 0.04, { dur: 0.7 });
			if (row.bar && row.fill) {
				tl.add(
					row.bar,
					[{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }],
					a + 0.1,
					0.8,
					'glide'
				);
				const p = Math.max(0, Math.min(1, row.it.progress ?? 0));
				tl.add(
					row.fill,
					[{ transform: 'scaleX(0)' }, { transform: `scaleX(${p.toFixed(4)})` }],
					1.3,
					0.9,
					'glide'
				);
			}
		});
		tl.add(
			last,
			[{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }],
			0.46 + n * 0.08,
			0.8,
			'glide'
		);

		// --- the spotlight walks the rows --------------------------------------------------------
		tl.cue('spotlight', s0);
		marker.style.transform = at(0);
		tl.add(marker, [{ opacity: 0 }, { opacity: 1 }], s0 - 0.3, 0.3, 'sine');
		shots.forEach((s, i) => {
			if (i === 0) return;
			// a camera settling on it: a dissolve and a short slide
			tl.add(s, [{ opacity: 0 }, { opacity: 1 }], lands[i] - 0.1, 0.8, 'sine');
			tl.add(
				s,
				[{ transform: 'translateX(48px)' }, { transform: 'translateX(0px)' }],
				lands[i] - 0.1,
				1.0,
				'glide'
			);
		});
		rows.forEach((row, i) => {
			const els = [row.name, row.sub];
			const dim = [{ opacity: 1 }, { opacity: 0.42 }];
			const lit = [{ opacity: 0.42 }, { opacity: 1 }];
			// everyone but the first dims as the spotlight starts; each lights in its turn
			if (i > 0) for (const e of els) tl.add(e, dim, s0 - 0.3, 0.4, 'sine');
			if (i > 0) for (const e of els) tl.add(e, lit, lands[i] - 0.1, 0.5, 'sine');
			if (i < n - 1) for (const e of els) tl.add(e, dim, lands[i + 1] - 0.1, 0.5, 'sine');
			if (i > 0) {
				tl.cue(`spotlight: ${row.it.title.name}`, lands[i]);
				tl.add(
					marker,
					[{ transform: at(i - 1) }, { transform: at(i) }],
					lands[i] - 0.1,
					0.6,
					'move'
				);
			}
		});

		// --- out ---------------------------------------------------------------------------------
		tl.cue('out', out);
		conceal(tl, [[later], [on], [wm]], out, { stagger: 0.03 });
		rows.forEach((row, i) => {
			const a = out + 0.06 + 0.04 * i;
			conceal(tl, [[row.time], ...row.lines, [row.sub], ...(row.tag ? [[row.tag]] : [])], a, {
				stagger: 0.02
			});
			tl.add(row.rule, [{ opacity: 1 }, { opacity: 0 }], a, 0.4, 'leave');
			if (row.bar) tl.add(row.bar, [{ opacity: 1 }, { opacity: 0 }], a, 0.4, 'leave');
		});
		tl.add(last, [{ opacity: 1 }, { opacity: 0 }], out + 0.06 + 0.04 * n, 0.4, 'leave');
		tl.add(marker, [{ opacity: 1 }, { opacity: 0 }], out + 0.1, 0.35, 'leave');
		tl.add(dark, [{ opacity: 0 }, { opacity: 1 }], D - 0.6, 0.6, 'sine');

		// --- sound -------------------------------------------------------------------------------
		tl.sound(0, (w, b, skip) => score(sound, b, w - skip, D), D);
		lands.forEach((t, i) =>
			tl.sound(t, (w, b) =>
				glass(house(sound, b), w, ['A4', 'B4', 'D5', 'E5', 'F#5'][i], {
					decay: 2.2,
					tone: 0.85,
					gain: 0.7,
					pan: 0.3 - 0.12 * i,
					verb: 0.65,
					seed: 610 + i
				})
			)
		);
		return tl;
	}
};

/** The bed under the list: the pad, the piano's slow eighths, a sub, the open fifth at the end. */
function score(k: Kit, b: Bus, w: number, D: number) {
	const h = house(k, b);
	const until = w + D - END;
	// the open fifth comes after the last row's glass note (at the latest 2.6 s before the end)
	// and rings out under the rows lifting away
	const fifth = D - 2.4;
	padSweep(h, w, [
		[0, 700],
		[2.4, 2600],
		[fifth, 1800],
		[D, 900]
	]);
	pad(h, w, ['D3', 'F#3', 'A3', 'C#4', 'E4'], fifth - 1.8, {
		attack: 1.4,
		release: 1.6,
		level: 0.75,
		seed: 41
	});
	sub(h, w + 0.4, 'D2', 2.4, { gain: 0.55, attack: 0.4, release: 1.6 });
	const arp = ['D4', 'F#4', 'A4', 'E5'];
	// eighths from the first downbeat until the fifth
	for (let i = 0, t = 0.4; t < fifth - 0.2; i++, t += BEAT / 2)
		piano(h, w + t, arp[i % arp.length], {
			vel: i % 4 === 0 ? 0.42 : 0.32,
			dur: 0.9,
			pan: [-0.25, -0.05, 0.12, 0.3][i % 4],
			verb: 0.55,
			gain: 0.62,
			seed: 640 + (i % 8)
		});
	const r = ringRoom(w + fifth, until);
	pad(h, w + fifth, ['D3', 'A3', 'D4'], 0.6 * r, {
		attack: 0.5,
		release: 1.8 * r,
		level: 0.6,
		seed: 43
	});
	(['D3', 'A3'] as const).forEach((note, i) =>
		piano(h, w + fifth + 0.2 + 0.01 * i, note, {
			vel: 0.5,
			dur: 2.4 * r,
			pan: -0.1 + 0.2 * i,
			gain: 0.8,
			seed: 660 + i
		})
	);
	sonicRingOut(h, w + fifth + 0.2, until);
}
