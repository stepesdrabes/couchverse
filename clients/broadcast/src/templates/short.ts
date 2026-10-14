// A short promo ("Tomorrow 21:00", the title): the camera rushes into the title's scenes from the
// night, accelerating, and on the bed's first hit a red bar swipes across the frame and wipes in
// the title card behind it: the scenes under the night, the title's logo (else its name) whole
// in the middle, then the airing in a red block above it. The channel tag closes.

import type { Kit } from '../audio/kit';
import {
	air,
	glass,
	house,
	impact,
	inhale,
	pad,
	padSweep,
	pulse,
	riser,
	shimmer,
	sub
} from '../audio/voices';
import { channelTag } from '../brand/lockup';
import { BRAND } from '../brand/wordmark';
import { css, div } from '../core/dom';
import { fitText } from '../core/fit';
import { inset } from '../core/framing';
import { edge } from '../core/moves';
import { FPS } from '../core/timeline';
import type { ShortPromoSpec } from '../spec';
import { logoImage, planCuts, scenes, shotLengths, type SceneCut } from './reel';
import { timeline, type Template, type TL } from './shared';

/** The house tempo: 75 bpm. */
const BEAT = 0.8;
/** The swipe: a red bar 240 px wide crossing the frame in 0.36 s, its middle on the hit. */
const SWIPE = { w: 240, dur: 0.36 };

export const shortPromo: Template<ShortPromoSpec> = {
	version: 6,
	build({ root, sound, assets }, spec) {
		const t = spec.title;
		const D = spec.duration ?? 9;
		const tl = timeline(root, D);
		root.classList.add('short');
		css(root, { background: BRAND.night });
		const bed = assets.bed(spec.bed);
		const onsets = bed?.onsets ?? [];

		// the hit: the strongest onset in the bed's first two seconds, on a frame edge
		const hit = edge(
			onsets.filter((o) => o.t > 0.5 && o.t < 2.2).sort((a, b) => b.s - a.s)[0]?.t ?? 1.2
		);
		// the tag 3.6 s after the hit, or later in a long promo, so it never holds for long
		const tagAt = Math.min(D - 3.3, Math.max(hit + 3.6, D - 4.2));
		const lengths = shotLengths(t);
		const n = lengths.length;

		// --- the rush: from the night into the scenes, accelerating to the hit ---
		const rush = div('sh-rush', root);
		const wipeIn = hit - SWIPE.dur / 2;
		const wipeOut = hit + SWIPE.dur / 2;
		// two quick cuts when there is time for both, the second on the bed's strongest beat
		// before the hit
		const rushCuts: SceneCut[] = [];
		if (n) {
			const beat = onsets
				.filter((o) => o.t > hit * 0.4 && o.t < hit - 0.35)
				.sort((a, b) => b.s - a.s)[0]?.t;
			// never past the first shot's last frame
			const mid = edge(Math.min(beat ?? Math.min(hit / 2 + 0.1, hit - 0.45), lengths[0] / FPS));
			const two = hit >= 1.1 && n >= 2;
			rushCuts.push({ a: 0, b: two ? mid : wipeOut, shot: 0, from: 0 });
			if (two) rushCuts.push({ a: mid, b: wipeOut, shot: 1, from: 0 });
		}
		scenes(tl, rush, t, assets, rushCuts, [0, wipeOut], {
			push: [1, 1],
			still: [
				[0, 0, 1, 1],
				[0, 0, 1, 1]
			]
		});
		tl.add(
			rush,
			[{ transform: 'scale(1)' }, { transform: 'scale(1.16)' }],
			0,
			wipeOut,
			'cubic-bezier(0.55, 0, 1, 0.45)'
		);
		tl.show(rush, 0, wipeOut);
		const veil = div('sh-veil', root);
		tl.add(veil, [{ opacity: 1 }, { opacity: 0 }], 0, 0.3, 'sine');

		// --- the card, wiped in behind the swipe: the scenes under the night, the title ---
		const card = div('sh-card', root);
		const cardIn = div('sh-card-in', card);
		const grid = bed
			? onsets.filter((o) => o.s >= 0.45).map((o) => o.t)
			: Array.from({ length: Math.ceil(D / BEAT) }, (_, i) => i * BEAT);
		// the card's scenes carry on from the rush's last shot, so the swipe passes over one scene
		const last = rushCuts[rushCuts.length - 1];
		const cursor = lengths.map(() => 0);
		if (last) cursor[last.shot] = Math.round((wipeIn - last.a) * FPS);
		const cardCuts = planCuts(lengths, wipeIn, D, grid, {
			min: 1.4,
			want: 2.6,
			first: last?.shot ?? 0,
			cursor
		});
		const focus = t.crops?.short?.[1] ?? inset([0, 0, 1, 1], 0.8);
		scenes(tl, cardIn, t, assets, cardCuts, [wipeIn, D], {
			push: [1.08, 1.16],
			still: [inset(focus, 0.92), inset(focus, 0.86)]
		});
		div('sh-grade', cardIn);

		const group = div('sh-group', cardIn);
		const when = div('sh-when', group);
		const eb = div('sh-eyebrow', when);
		const block = div('sh-block', eb);
		const ebMask = div('sh-mask', eb);
		const ebText = div(null, ebMask, spec.airing.eyebrow);
		const timeMask = div('sh-mask', when);
		const timeText = div(
			'tnum',
			timeMask,
			spec.airing.day ? `${spec.airing.day} ${spec.airing.time}` : spec.airing.time
		);
		fitText(when, { width: 1600, min: 36 });

		const titleBox = div('sh-title', group);
		const logo = logoImage(titleBox, assets, t.logo, BRAND.night, {
			width: 1300,
			height: 420,
			area: 330000
		});
		const name = logo ? null : div('sh-name', titleBox, t.name);
		// under the airing, clear of the channel tag
		if (name) fitText(name, { width: 1600, min: 120, lines: 3, height: 600 });
		const mark = logo?.el ?? name;

		tl.cue('the hit: the swipe wipes the title in', hit);
		const swipe = div('sh-swipe', root);
		tl.add(
			card,
			[{ transform: `translateX(${-1920 - SWIPE.w}px)` }, { transform: 'translateX(0px)' }],
			wipeIn,
			SWIPE.dur,
			'move'
		);
		tl.add(
			cardIn,
			[{ transform: `translateX(${1920 + SWIPE.w}px)` }, { transform: 'translateX(0px)' }],
			wipeIn,
			SWIPE.dur,
			'move'
		);
		tl.add(
			swipe,
			[{ transform: `translateX(${-SWIPE.w}px)` }, { transform: 'translateX(1920px)' }],
			wipeIn,
			SWIPE.dur,
			'move'
		);
		tl.show(swipe, wipeIn, wipeOut);
		// the title settles as the swipe passes, then drifts closer through the card
		if (mark) {
			tl.add(mark, [{ transform: 'scale(1.06)' }, { transform: 'scale(1)' }], wipeIn, 0.7, 'glide');
			tl.add(
				mark,
				[{ transform: 'scale(1)' }, { transform: 'scale(1.03)' }],
				wipeIn + 0.7,
				D - wipeIn - 0.7,
				'linear'
			);
		}

		// the airing: its block opens, the words rise into it, the time after
		const wAt = hit + 0.5;
		tl.cue('the airing', wAt);
		tl.add(block, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], wAt, 0.45, 'glide');
		tl.add(
			ebText,
			[{ transform: 'translateY(105%)' }, { transform: 'translateY(0)' }],
			wAt + 0.1,
			0.6,
			'glide'
		);
		tl.add(
			timeText,
			[{ transform: 'translateY(105%)' }, { transform: 'translateY(0)' }],
			wAt + 0.22,
			0.6,
			'glide'
		);

		channelTag(tl, root, tagAt, sound, { gain: 0.5 });
		const out = div('sh-veil', root);
		tl.add(out, [{ opacity: 0 }, { opacity: 1 }], D - 0.4, 0.4, 'sine');

		score(tl, sound, { hit, wAt, tagAt, D, bed: !!bed });
		tl.sound(
			0,
			(w, b, skip) =>
				sound.bed(b, w, bed, skip, {
					fadeIn: 0.02,
					fadeOut: 1.6,
					end: D,
					gain: 1,
					ducks: [{ at: tagAt - 0.1, depth: 0.45, hold: 1.4, attack: 0.25 }]
				}),
			D
		);
		return tl;
	}
};

interface Moments {
	hit: number;
	wAt: number;
	tagAt: number;
	D: number;
	bed: boolean;
}

/**
 * The house's part. Under a bed: a reversed swell into the hit (the hit is the bed's own), air
 * under the swipe, a glass note as the airing lands, all 10 dB and more under it. Without one,
 * the house carries the promo: a chord rising reversed into the hit, the hit itself (a soft
 * impact, D major add 9 blooming with a sub and the light above it), a felted pulse on the beat
 * under the card, the pad holding until the tag's sonic logo.
 */
function score(tl: TL, sound: Kit, o: Moments) {
	const { hit } = o;
	const lead = Math.min(hit - 0.05, 1.1);
	tl.sound(hit - lead, (w, b) => {
		const h = house(sound, b);
		const at = w + lead;
		inhale(h, at, lead, { gain: o.bed ? 0.7 : 0.6, breath: 0.04, seed: 601 });
		air(h, at - lead, lead + 0.3, {
			peakAt: lead - 0.05,
			band: [300, 2600],
			gain: o.bed ? 0.45 : 0.35,
			seed: 608
		});
		if (!o.bed) {
			riser(h, at, ['A2', 'E3', 'A3', 'D4', 'E4'], lead, { gain: 1.25, breath: 0.035, seed: 602 });
			sub(h, at - lead, 'A1', Math.max(0.1, lead - 0.5), {
				gain: 0.6,
				attack: Math.min(0.5, lead - 0.1),
				release: 0.15
			});
		}
	});
	tl.sound(hit - 0.15, (w, b) => {
		const h = house(sound, b);
		air(h, w, 0.6, {
			peakAt: 0.18,
			band: [900, 5200],
			pans: [-0.8, 0.8],
			gain: o.bed ? 0.3 : 0.45,
			seed: 603
		});
	});
	tl.sound(o.wAt + 0.1, (w, b) => {
		const h = house(sound, b);
		glass(h, w, 'A5', { decay: 1.4, gain: o.bed ? 0.4 : 0.7, pan: 0.15, verb: 0.6, seed: 604 });
	});
	if (o.bed) return;
	tl.sound(hit, (w, b) => {
		const h = house(sound, b);
		impact(h, w, { gain: 0.42, root: 'D1', seed: 605 });
		const ring = Math.max(1.2, o.tagAt - hit - 0.9);
		pad(h, w, ['D3', 'A3', 'E4', 'F#4', 'A4'], ring, {
			attack: 0.08,
			release: 1.6,
			level: 0.7,
			spread: 24,
			seed: 606
		});
		padSweep(h, w, [
			[0, 5200],
			[1.2, 2400],
			[ring, 1600],
			[ring + 1.6, 900]
		]);
		sub(h, w, 'D2', ring, { gain: 0.45, attack: 0.05, release: 1.2 });
		shimmer(h, w + 0.02, ['A5', 'D6', 'F#6'], 1.4, {
			gain: 0.55,
			attack: 0.4,
			release: 1.6,
			seed: 607
		});
		// a felted heartbeat under the card, until the tag
		for (let i = 1, at = BEAT; hit + at < o.tagAt - 0.6; i++, at += BEAT)
			pulse(h, w + at, i % 4 === 2 ? 'A1' : 'D2', { gain: 1.15, decay: 0.32, seed: 610 + i });
	});
}
