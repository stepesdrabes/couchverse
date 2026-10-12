// A sting: the gap filler in the ident's language, any whole number of frames from 2 to 6 s.
// The ident's finale on the tag's hook: a beat of red as the lockup lands, the brackets open on
// the hook's notes and TV drops into its block on the long one; the hold absorbs the rest.
// From 3 s it opens like the short ident (the cursor, the O, the brackets around the couch).

import { SPARKLE } from '../audio/kit';
import { roll } from '../audio/orchestra';
import { div } from '../core/dom';
import { motion, windows, type Move } from '../core/moves';
import { frame } from '../core/timeline';
import type { StingSpec } from '../spec';
import { timeline, type Template } from '../templates/shared';
import { frameItem, popIn } from './ident';
import { HOOK, themeTag } from './theme';
import { animateBrackets, centre, cursorAt, geometry, wordmark, type Frame } from './wordmark';

/** The ident's lockup size: a sting is the ident's finale. */
const SIZE = 190;

export const sting: Template<StingSpec> = {
	version: 2,
	build({ root, sound }, spec) {
		const D = frame(Math.min(6, Math.max(2, spec.duration)));
		const tl = timeline(root, D);
		root.classList.add('sting');
		const lead = D >= 3;
		// the hook's first note: the lockup lands
		const H = lead ? 1.0 : 0.4;
		const R = HOOK.tagAt.map((r) => H + r);
		const beat = R[2] - R[0];

		const red = div('id-field id-red', root);
		const wm = wordmark(root, SIZE, { colors: ['red', 'black'] });
		const m = wm.m;
		const pos = centre(wm, 960, 540);
		const centreDx = 960 - (pos.x + m.cx);
		const centreDy = 540 - (pos.y + m.cy);
		const closed: Frame = { g: m.split, k: 1, dx: 0, dy: 0 };
		const moves: Move<Frame>[] = [];
		const win: Record<'red' | 'black', [number, number][]> = {
			red: [[H + beat, D + 1]],
			black: [[H, H + beat]]
		};
		const curWin: [number, number][] = [[H + beat, R[4] + 0.12]];
		let cur0 = cursorAt(m, m.tv.x);
		let v0 = closed;

		if (lead) {
			// the short ident's opening: the cursor at the centre becomes the O, then the brackets
			// take the screen around the couch and slam back into the word on the hit
			tl.cue('the cursor', 0);
			cur0 = cursorAt(m, 960 - pos.x - m.curW / 2);
			curWin.push([0, 0.25]);
			v0 = { ...closed, dx: centreDx, dy: centreDy };
			tl.cue('the cursor becomes the O', 0.25);
			const couch = frameItem(div('id-items', root), 'couch', '#fff');
			const slam = H - 0.125;
			popIn(tl, couch, 0.5, slam);
			const k = Math.min(3.1, (couch.h * 1.1) / m.capH);
			tl.cue('the brackets frame the couch', 0.5);
			moves.push(
				{
					at: 0.5,
					dur: 0.14,
					to: { g: couch.w + 0.16 * m.em * k, k, dx: centreDx, dy: centreDy },
					ease: 'pop'
				},
				{ at: slam, dur: 0.1, to: { ...closed, g: m.split - 0.03 * m.em }, ease: 'in' }
			);
			win.red.push([0.25, H]);
			tl.sound(0.25, (w, b) => sound.clack(b, w, { gain: 0.04, freq: SPARKLE[0] }));
			tl.sound(0.5, (w, b) => sound.clack(b, w, { gain: 0.05, freq: SPARKLE[2], pan: -0.25 }));
			tl.sound(0.5, (w, b) =>
				roll(sound, b, w, w + H - 0.55, {
					voice: 'drum',
					f: 78,
					gain: 0.2,
					from: 0.3,
					to: 1,
					pitch: [1, 1.3],
					grid: [0.125, 0.0625]
				})
			);
		}

		tl.cue('the lockup lands', H);
		windows(tl, red, [[H, H + beat]]);
		for (const e of [wm.c, ...wm.uch]) windows(tl, e, [[H, D + 1]]);
		moves.push({ at: H, dur: 0.14, to: closed, ease: 'pop' });
		// the brackets open a little on each note of the hook, widest on the top note
		const pulses = (
			[
				[R[1], 0.2],
				[R[2], 0.42],
				[R[3], 0.22]
			] as const
		).flatMap(([t, w]): Move<Frame>[] => [
			{ at: t, dur: 0.05, to: { ...closed, g: m.em * w + m.split }, ease: 'out' },
			{ at: t + 0.08, dur: 0.1, to: closed, ease: 'inOut' }
		]);
		moves.push(...pulses);
		animateBrackets(tl, wm, v0, moves, win);
		const letterMoves: Move<{ g: number; off: number }>[] = [
			{ at: H - 0.002, to: { g: m.split, off: 0.3 * m.em } },
			{ at: H, dur: 0.2, to: { g: m.split, off: 0 }, ease: 'snap' },
			...pulses.map((mv) => ({ ...mv, to: { g: mv.to.g, off: 0 } }))
		];
		motion(tl, wm.c, { g: m.split, off: 0 }, letterMoves, (v) => ({
			transform: `${geometry(m, v.g).c} translateX(${v.off.toFixed(1)}px)`
		}));
		motion(tl, wm.rest, { g: m.split, off: 0 }, letterMoves, (v) => ({
			transform: `${geometry(m, v.g).rest} translateX(${(-v.off).toFixed(1)}px)`
		}));

		tl.cue('the long note: TV lands', R[4]);
		motion(
			tl,
			wm.cur,
			cur0,
			[
				{ at: H, to: cursorAt(m, m.tv.x) },
				{ at: R[4], dur: 0.12, to: cursorAt(m, m.tv.x, m.tv.w), ease: 'snap' }
			],
			(v) => ({ transform: v })
		);
		windows(tl, wm.cur, curWin);
		windows(tl, wm.tvBg, [[R[4] + 0.12, D + 1]]);
		windows(tl, wm.tvT, [[R[4], D + 1]]);
		windows(tl, wm.tvV, [[R[4] + 0.06, D + 1]]);
		tl.add(wm.root, [{ transform: 'scale(1)' }, { transform: 'scale(1.025)' }], H, D - H, 'sine');

		tl.sound(H - 0.4, (w, b) => themeTag(sound, b, w + 0.4, { gain: 0.5 }));
		for (const t of [R[1], R[2], R[3]])
			tl.sound(t, (w, b) => sound.clack(b, w, { gain: 0.022, freq: SPARKLE[1], pan: -0.1 }));
		return tl;
	}
};
