// "Up next": the next program's scenes under a slow push, its logo (or its name) lower left
// under a red "Up next" block, and the horizon line along the foot of the frame filling red
// until it reaches the start time, where the program begins. Ends on night (the program starts
// from it); the bug stays to the last frame, where the player's own bug takes over at the same
// place. After an expanding squeeze it continues that squeeze's last frame (`enter: 'continue'`).
//
// Score: the title's own bed leads, dipping under the sonic logo on glass on the three beats
// before the last (A4, E5, D5), the D ringing out under the type lifting away, so the sound is
// over before the program starts; without a bed the house plays a D add 9 pad opening over the
// clip, the felt piano touching its notes, then the same glass logo.

import type { Bus } from '../audio/engine';
import type { Kit } from '../audio/kit';
import { glass, house, pad, padSweep, piano, sub } from '../audio/voices';
import { bug } from '../brand/lockup';
import { END, ringRoom, SONIC, sonicRingOut } from '../brand/sonic';
import { css, div, span } from '../core/dom';
import { fitText } from '../core/fit';
import type { UpNextSpec } from '../spec';
import {
	conceal,
	GRADE,
	grade,
	INK,
	label,
	labelIn,
	labelOut,
	masked,
	meta,
	push,
	reveal,
	scenes,
	titleMark
} from './onair';
import { timeline, type Template } from './shared';

/** The picture's push (zoom at the first and the last frame) and its crop without scenes. */
export const UPNEXT = {
	zoom: [1.04, 1.12] as [number, number],
	crop: [0.03, 0.03, 0.94, 0.94] as [number, number, number, number],
	/**
	 * Seconds of the next title's scenes an expanding squeeze has shown when up next continues
	 * its last frame: the squeeze's card plays them from 3.0 to its end at 11.52.
	 */
	continued: 8.52
};

/** The type block's left edge, its widest line and its foot (DESIGN.md 1.6). */
const BLOCK = { x: 96, width: 1104, foot: 920 };
/** The horizon line and the start time over its end. */
const HORIZON = { y: 984, x0: 96, x1: 1824 };
/** A logo's place in the block: the same visual weight whatever its shape. */
const LOGO = { area: 168000, width: 920, height: 300 };

export const upNext: Template<UpNextSpec> = {
	version: 9,
	build({ root, sound, assets }, spec) {
		const t = spec.title;
		const D = spec.duration ?? 8;
		const tl = timeline(root, D);
		root.classList.add('upnext');
		const continuing = spec.enter === 'continue';
		const bed = assets.bed(spec.bed);

		// the picture, graded into the night on the left and at the foot
		const view = div('fill oa-view', root);
		const cam = scenes(tl, view, t, assets, {
			offset: continuing ? UPNEXT.continued : 0,
			crop: t.crops?.upNext?.[0] ?? UPNEXT.crop
		});
		push(tl, cam, 0, D, UPNEXT.zoom);
		grade(root, [GRADE.bug, GRADE.left(66), GRADE.foot(58), GRADE.dim(0.14)]);

		// the horizon: a hairline that fills red until it reaches the start time
		const track = css(div('abs un-track', root), {
			left: `${HORIZON.x0}px`,
			top: `${HORIZON.y}px`,
			width: `${HORIZON.x1 - HORIZON.x0}px`
		});
		const fill = css(div('abs un-fill', root), {
			left: `${HORIZON.x0}px`,
			top: `${HORIZON.y - 1}px`,
			width: `${HORIZON.x1 - HORIZON.x0}px`
		});
		const head = css(div('abs un-head', root), {
			left: `${HORIZON.x0}px`,
			top: `${HORIZON.y - 7}px`
		});
		const time = masked(root, 'un-time tnum', spec.time);
		css(time.parentElement as HTMLElement, {
			position: 'absolute',
			right: `${1920 - HORIZON.x1}px`,
			top: `${HORIZON.y - 58}px`
		});

		// night over the picture and the line at both ends: one layer instead of fading each
		const dark = css(div('fill', root), { background: INK.night });

		// the type, lower left
		const block = css(div('abs un-block', root), {
			left: `${BLOCK.x}px`,
			bottom: `${1080 - BLOCK.foot}px`,
			width: `${BLOCK.width}px`
		});
		const lbl = label(block, 'Up next', 'un-label');
		const mark = titleMark(
			block,
			t,
			assets,
			'un-title',
			{ width: BLOCK.width, min: 76, lines: 3 },
			LOGO
		);
		let ep: HTMLElement | null = null;
		if (spec.episode) {
			ep = div('un-ep', div('oa-mask un-ep-mask', block));
			span('un-code tnum', ep, spec.episode.code);
			const name = span('un-ep-name', ep, spec.episode.name);
			// beside the code, within the block
			fitText(name, {
				width: BLOCK.width - name.offsetLeft,
				min: 30,
				lines: 2,
				leading: 1.2
			});
		}
		const runtime = spec.episode?.runtimeMinutes ?? t.runtimeMinutes;
		const metaEl = meta(div(`oa-mask un-meta-mask${ep ? ' after-ep' : ''}`, block), 'un-meta', [
			...(spec.episode ? [] : t.year ? [String(t.year)] : []),
			...t.genres.slice(0, 2),
			...(runtime ? [`${runtime} min`] : [])
		]);

		if (spec.clock) bug(tl, root, spec.clock);

		// --- in ----------------------------------------------------------------------------------
		const T0 = continuing ? 0.16 : 0.32;
		if (!continuing) tl.add(dark, [{ opacity: 1 }, { opacity: 0 }], 0, 0.8, 'sine');
		else dark.style.opacity = '0';
		tl.add(track, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], 0, 0.9, 'glide');
		tl.add(fill, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], 0, D, 'linear');
		tl.add(
			head,
			[
				{ transform: 'translateX(0px)' },
				{ transform: `translateX(${HORIZON.x1 - HORIZON.x0 - 4}px)` }
			],
			0,
			D,
			'linear'
		);
		// it comes with the line (a continuing frame 0 is the squeeze's last, which has none)
		tl.add(head, [{ opacity: 0 }, { opacity: 1 }], 0.1, 0.4, 'sine');
		tl.cue('up next', T0);
		labelIn(tl, lbl, T0);
		tl.cue(mark.logo ? 'the logo' : 'the title', T0 + 0.12);
		reveal(tl, mark.lines, T0 + 0.12, { stagger: 0.08, dur: mark.logo ? 1.0 : 0.9 });
		const after = T0 + 0.12 + (mark.lines.length - 1) * 0.08;
		if (ep) reveal(tl, [[ep]], after + 0.3, { dur: 0.8 });
		reveal(tl, [[metaEl]], after + (ep ? 0.4 : 0.3), { dur: 0.8 });
		reveal(tl, [[time]], T0 + 0.6, { dur: 0.8 });

		// --- out: the type lifts away, the line reaches the time, the picture goes to night ------
		const out = D - 0.72;
		tl.cue('type out', out);
		labelOut(tl, lbl, out);
		conceal(tl, [...mark.lines, ...(ep ? [[ep]] : []), [metaEl]], out + 0.03, { stagger: 0.03 });
		tl.add(dark, [{ opacity: 0 }, { opacity: 1 }], D - 0.32, 0.32, 'sine');
		tl.cue('the program', D);

		// --- sound -------------------------------------------------------------------------------
		const notes = SONIC.notes.map((_, i) => D - 3.2 + 0.8 * i);
		tl.sound(
			0,
			(w, b, skip) =>
				sound.bed(b, w, bed, skip, {
					fadeIn: 0.35,
					fadeOut: 1.2,
					end: D,
					// a 3 dB dip under each note of the logo
					ducks: notes.map((at) => ({ at, depth: 0.71, attack: 0.12, hold: 0.36, release: 0.3 }))
				}),
			D
		);
		if (!bed) tl.sound(0, (w, b, skip) => houseBed(sound, b, w - skip, D), D);
		notes.forEach((at, i) =>
			tl.sound(at, (w, b) => logoNote(sound, b, w, i, !!bed, w - at + D - END))
		);
		// the D dies away under the type lifting off, the program starting from silence
		tl.sound(notes[2], (w, b) => sonicRingOut(house(sound, b), w, w - notes[2] + D - END));
		return tl;
	}
};

/** The house's bed without the title's own: D add 9 opening over the clip, the piano touching it. */
function houseBed(k: Kit, b: Bus, w: number, D: number) {
	const h = house(k, b);
	padSweep(h, w, [
		[0, 400],
		[D - 2.6, 2400],
		[D, 900]
	]);
	// it opens until the logo and lets go there: the notes and the hall carry the rest
	pad(h, w, ['D3', 'A3', 'E4', 'F#4'], D - 4.4, {
		attack: 1.2,
		release: 1.4,
		level: 0.9,
		seed: 61
	});
	sub(h, w + 0.05, 'D2', D - 4.2, { gain: 0.45, attack: 1.0, release: 1.2 });
	// beats 1 and 3 of the first bar, then the next downbeat
	(
		[
			[0.32, 'A3'],
			[1.92, 'E4'],
			[3.52, 'D4']
		] as const
	).forEach(([at, note], i) =>
		piano(h, w + at, note, { vel: 0.5, dur: 2.6, pan: -0.12 + 0.12 * i, gain: 0.9, seed: 510 + i })
	);
}

/** One note of the sonic logo on glass, the last one ringing out to `until`. */
function logoNote(k: Kit, b: Bus, w: number, i: number, overBed: boolean, until: number) {
	const h = house(k, b);
	const last = i === SONIC.notes.length - 1;
	glass(h, w, SONIC.notes[i], {
		decay: (last ? 3.2 : 1.8) * ringRoom(w, until),
		tone: 0.9,
		gain: (overBed ? 0.55 : 0.9) * (last ? 1.15 : 1),
		pan: [-0.18, 0.14, 0][i],
		verb: last ? 0.8 : 0.6,
		seed: 530 + i
	});
}
