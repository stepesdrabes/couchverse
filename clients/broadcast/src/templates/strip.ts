// A strip promo ("Every weekday at 18:00"): the series' scenes for a beat, then a flat panel
// slides in over the left half with a red bar on its edge, the scenes moving aside, and the
// column types in on it: the eyebrow in a red block, the title's logo (else its name), the time
// as the hero numeral, the week, whose aired days light red on the beat, and the foot line.
// Before the channel tag the panel sweeps on across the scenes and the tag lands on the clean
// field. Moods: bright is day (a paper panel, night type, the scenes as they are, hard cuts on
// the beat), dark is night (a night panel with the dusk at its foot, the scenes dimmed,
// dissolves, slower moves).

import type { Kit } from '../audio/kit';
import { beatsIn, snapper } from '../audio/onsets';
import { hz } from '../audio/pitch';
import { air, glass, house, impact, pad, padSweep, piano, sub } from '../audio/voices';
import { channelTag } from '../brand/lockup';
import { BRAND } from '../brand/wordmark';
import { chars, css, div, el, span } from '../core/dom';
import { fitText } from '../core/fit';
import { edge } from '../core/moves';
import type { StripPromoSpec } from '../spec';
import { logoImage, planCuts, scenes, shotLengths } from './reel';
import { timeline, type Template, type TL } from './shared';

const DAYS = ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'];
const DAY_NAMES = ['Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday'];

/** The house tempo: 75 bpm, a beat is 20 frames. */
const BEAT = 0.8;
/** The panel's edge: the frame's middle; its red bar rides on the scenes' side of it. */
const SPLIT = 960;
const BAR = 12;
/** The column's width: from the title-safe line to 72 px before the panel's edge. */
const COL_W = SPLIT - 96 - 72;

/** Each mood's field and type tiers (on paper the tiers are night over paper, solid). */
const MOODS = {
	bright: {
		field: BRAND.paper,
		ink: BRAND.night,
		soft: '#5f5a55',
		dim: '#a39d96',
		track: '#ddd5ca',
		open: 0.9,
		ease: 'glide',
		fill: 0.36,
		dissolve: 0,
		want: 2.4
	},
	dark: {
		field: BRAND.night,
		ink: BRAND.paper,
		soft: '#b1aca6',
		dim: '#65625e',
		track: '#3e3d3b',
		open: 1.2,
		ease: 'dolly',
		fill: 0.5,
		dissolve: 0.6,
		want: 3.2
	}
} as const;

/** The days' notes, climbing D major's pentatonic, one per lit day. */
const DAY_NOTES = ['D5', 'E5', 'F#5', 'A5', 'B5', 'D6', 'E6'];

/** A box that clips what rises into it from below (a masked reveal). */
const mask = (parent: Element, cls = 'sp-mask') => div(cls, parent);

function riseIn(tl: TL, e: Element, at: number, dur = 0.8, from = '105%') {
	tl.add(
		e,
		[{ transform: `translateY(${from})` }, { transform: 'translateY(0)' }],
		at,
		dur,
		'glide'
	);
}

export const stripPromo: Template<StripPromoSpec> = {
	version: 5,
	build({ root, sound, assets }, spec) {
		const t = spec.title;
		const D = spec.duration ?? 16;
		const dark = spec.mood === 'dark';
		const m = MOODS[spec.mood];
		const tl = timeline(root, D);
		root.classList.add('strip', dark ? 'mood-dark' : 'mood-bright');
		css(root, { background: BRAND.night });
		const bed = assets.bed(spec.bed);
		const onsets = bed?.onsets ?? [];
		const snap = snapper(onsets);
		// the beat grid the cuts and the days follow: the bed's beats, else the house tempo
		const grid = bed
			? onsets.filter((o) => o.s >= 0.45).map((o) => o.t)
			: Array.from({ length: Math.ceil(D / BEAT) }, (_, i) => i * BEAT);

		const tagAt = D - 3.6;
		// the panel's sweep on the bed's strongest onset near its place
		const close = edge(snap(tagAt - 1.3, 0.3, 0.5));
		const closeDur = 0.62;

		// --- the scenes, behind everything: full frame, then aside for the panel ---
		const world = div('sp-world', root);
		const cuts = planCuts(shotLengths(t), 0, close + closeDur, grid, {
			min: 1.2,
			want: m.want
		});
		scenes(tl, world, t, assets, cuts, [0, close + closeDur], {
			dissolve: m.dissolve,
			push: dark ? [1.02, 1.07] : [1, 1.05],
			still: t.crops?.strip
		});

		// --- the panel: its right edge on the frame's middle at rest, its red bar beside it ---
		const panel = css(div('sp-panel', root), { background: m.field });
		div('sp-bar', panel);
		// the night's dusk rising at the panel's foot (the scenes are dimmed by their own opacity,
		// so this is the one layer over them)
		if (dark) div('sp-dusk', root);

		// --- the column ---
		const col = css(div('sp-col', root), { color: m.ink });
		const ebBox = div('sp-eyebrow', col);
		const ebBlock = div('sp-eyebrow-block', ebBox);
		const ebMask = mask(ebBox);
		const ebText = div('sp-eyebrow-text', ebMask, spec.airing.eyebrow);
		fitText(ebText, { width: COL_W - 40, min: 30 });

		const titleBox = div('sp-title', col);
		// by day a logo made for dark backgrounds sits on a night plate rather than being redrawn
		const logo = logoImage(
			titleBox,
			assets,
			t.logo,
			m.field,
			{ width: COL_W, height: 300, area: 207000 },
			dark ? {} : { plate: { field: BRAND.night, pad: [26, 30] } }
		);
		const titleParts: HTMLElement[] = [];
		let plate: HTMLElement | null = null;
		if (logo) {
			const holder = logo.plate ? div('sp-plate', titleBox) : titleBox;
			if (logo.plate) plate = css(div('sp-plate-bg', holder), { background: BRAND.night });
			const lm = mask(holder, 'sp-mask sp-logo-mask');
			lm.appendChild(logo.el);
			titleParts.push(logo.el);
		} else {
			const name = div('sp-name', titleBox, t.name);
			fitText(name, { width: COL_W, min: 72, lines: 3 });
			// one mask per fitted line, so each line rises into its own
			const lines: Node[][] = [[]];
			for (const n of [...name.childNodes]) {
				if (n instanceof HTMLBRElement) lines.push([]);
				else lines[lines.length - 1].push(n);
			}
			name.replaceChildren();
			for (const line of lines) {
				const lm = mask(name, 'sp-mask sp-line');
				const inner = div('sp-line-in', lm);
				inner.append(...line);
				titleParts.push(inner);
			}
		}

		const timeMask = mask(col, 'sp-mask sp-time');
		const time = chars(timeMask, spec.airing.time, 'tnum');

		const week = div('sp-week', col);
		const lit = new Set(spec.airing.days);
		const slots = DAYS.map((name, d) => {
			const s = div('sp-slot', week);
			const lm = mask(s, 'sp-mask sp-day');
			const lab = div('sp-day-in', lm);
			const off = css(span('sp-day-off', lab, name), { color: m.dim });
			const on = css(span('sp-day-on', lab, name), { color: m.ink });
			const track = css(div('sp-track', s), { background: m.track });
			const fill = div('sp-fill', track);
			return { s, lab, off, on, track, fill, lit: lit.has(d) };
		});

		const f = spec.foot;
		let foot: HTMLElement | null = null;
		let marker: HTMLElement | null = null;
		if (f) {
			const fm = mask(col, 'sp-mask sp-foot');
			foot = css(div('sp-foot-in', fm), { color: m.soft });
			if (f.kind === 'count') {
				css(el('b', 'tnum', foot, String(f.count)), { color: m.ink });
				foot.append(f.count === 1 ? ' episode, in order' : ' episodes, in order');
			} else {
				foot.append(`Season ${f.season} from `);
				css(el('b', null, foot, DAY_NAMES[f.day]), { color: m.ink });
				marker = css(div('sp-marker', slots[f.day].s), { background: m.ink });
			}
			fitText(foot, { width: COL_W, min: 26 });
		}
		// the column sits on the frame's middle, a little high
		css(col, { top: `${Math.round((1080 - col.offsetHeight) / 2 - 20)}px` });

		// --- the open: the scenes come up, the panel slides in and pushes them aside ---
		const veil = div('sp-veil', root);
		tl.add(veil, [{ opacity: 1 }, { opacity: 0 }], 0, 0.4, 'sine');
		// the scenes alone for a beat, then the panel on the bed's strongest onset near it
		const pIn = edge(snap(bed ? 1.0 : BEAT, 0.35, 0.5));
		tl.cue('the panel slides in', pIn);
		tl.add(
			panel,
			[{ transform: `translateX(${-SPLIT - BAR}px)` }, { transform: 'translateX(0px)' }],
			pIn,
			m.open,
			m.ease
		);
		tl.add(
			world,
			[{ transform: 'translateX(0px)' }, { transform: `translateX(${SPLIT / 2}px)` }],
			pIn,
			m.open,
			m.ease
		);

		// --- the column types in on the panel ---
		const t0 = pIn + m.open * (dark ? 0.55 : 0.5);
		tl.cue('eyebrow', t0);
		tl.add(ebBlock, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], t0, 0.5, 'glide');
		riseIn(tl, ebText, t0 + 0.12, 0.6);
		const tTitle = t0 + 0.24;
		tl.cue('title', tTitle);
		if (plate)
			tl.add(
				plate,
				[{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }],
				tTitle - 0.08,
				0.6,
				'glide'
			);
		titleParts.forEach((p, i) => riseIn(tl, p, tTitle + (plate ? 0.1 : 0) + i * 0.08, 0.9));
		const tTime = snap(tTitle + 0.75 + (dark ? 0.15 : 0), 0.35, 0.5);
		tl.cue('time', tTime);
		time.items.forEach((c, i) => riseIn(tl, c, tTime + i * 0.05, 0.85));
		const tWeek = tTime + 0.6;
		slots.forEach((s, i) => {
			riseIn(tl, s.lab, tWeek + i * 0.035, 0.6);
			tl.add(
				s.track,
				[{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }],
				tWeek + i * 0.035,
				0.6,
				'glide'
			);
		});

		// the aired days light up, on the beats where the bed has them, else half a beat apart
		const litSlots = slots.filter((s) => s.lit);
		const first = tWeek + 0.65;
		const beats = bed ? beatsIn(onsets, first - 0.2, close, 0.28, 0.3, 0) : [];
		let prev = first - 0.4;
		const litAt = litSlots.map(() => {
			const next = beats.find((x) => x > prev + 0.26 && x < prev + 0.7) ?? prev + BEAT / 2;
			prev = edge(next);
			return prev;
		});
		if (litAt.length) tl.cue('days light up', litAt[0]);
		litSlots.forEach((s, i) => {
			const at = litAt[i];
			tl.add(s.fill, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], at, m.fill, 'glide');
			tl.add(s.on, [{ opacity: 0 }, { opacity: 1 }], at, 0.2, 'linear');
			tl.add(s.off, [{ opacity: 1 }, { opacity: 0 }], at, 0.2, 'linear');
		});
		const afterDays = (litAt[litAt.length - 1] ?? first) + 0.45;
		if (marker) {
			tl.add(
				marker,
				[
					{ opacity: 0, transform: 'translateY(-10px)' },
					{ opacity: 1, transform: 'translateY(0px)' }
				],
				afterDays,
				0.5,
				'glide'
			);
		}
		if (foot) {
			tl.cue(f?.kind === 'count' ? 'episode count' : 'season line', afterDays + 0.1);
			riseIn(tl, foot, afterDays + 0.1, 0.8);
		}

		// --- the close: the panel sweeps on across the scenes, the tag lands on the field ---
		tl.cue('the panel covers the scenes', close);
		tl.add(
			panel,
			[{ transform: 'translateX(0px)' }, { transform: `translateX(${SPLIT + BAR}px)` }],
			close,
			closeDur,
			'move'
		);
		tl.add(
			world,
			[
				{ transform: `translateX(${SPLIT / 2}px)` },
				{ transform: `translateX(${SPLIT / 2 + 240}px)` }
			],
			close,
			closeDur,
			'move'
		);
		tl.show(world, 0, close + closeDur);
		const tag = channelTag(tl, root, tagAt, sound, { gain: 0.5 });
		// on paper the lockup's white `couch` is inked in night, as `lockupSvg` draws it on light
		if (!dark)
			tag
				.querySelectorAll(`path[fill="${BRAND.white}"]`)
				.forEach((p) => p.setAttribute('fill', BRAND.night));

		const out = div('sp-veil', root);
		tl.add(out, [{ opacity: 0 }, { opacity: 1 }], D - 0.32, 0.32, 'sine');

		// --- sound ---
		score(tl, sound, { pIn, tTime, litAt, close, tagAt, D, dark, bed: !!bed });
		tl.sound(
			0,
			(w, b, skip) =>
				sound.bed(b, w, bed, skip, {
					fadeIn: 0.3,
					fadeOut: 1.6,
					end: D,
					gain: 1,
					ducks: [{ at: tagAt - 0.1, depth: 0.4, hold: 1.6, attack: 0.25 }]
				}),
			D
		);
		return tl;
	}
};

interface Moments {
	pIn: number;
	tTime: number;
	litAt: number[];
	close: number;
	tagAt: number;
	D: number;
	dark: boolean;
	bed: boolean;
}

/**
 * The house's part: under a bed only punctuation, 10 dB and more under it (air under the
 * panel's moves, a soft landing for the time, a note per lit day: glass climbing D major's
 * pentatonic, or in the dark the felt piano an octave lower); without one, the house music
 * itself (a pad on D add9 opening up, a sub, the piano walking the chord on the beat) under the
 * same notes. The tag's sonic logo comes with the tag.
 */
function score(tl: TL, sound: Kit, o: Moments) {
	const k = o.bed ? 1 : 1.6;
	tl.sound(Math.max(0.05, o.pIn - 0.55), (w, b) => {
		const h = house(sound, b);
		air(h, w, 1.5, { peakAt: 0.7, band: [380, 2400], pans: [-0.7, 0], gain: 0.35 * k, seed: 501 });
	});
	tl.sound(o.tTime, (w, b) => {
		const h = house(sound, b);
		impact(h, w, { gain: (o.bed ? 0.16 : 0.3) * (o.dark ? 1.2 : 1), root: 'D1', seed: 502 });
	});
	o.litAt.forEach((at, i) =>
		tl.sound(at, (w, b) => {
			const h = house(sound, b);
			const note = DAY_NOTES[i % DAY_NOTES.length];
			if (o.dark)
				piano(h, w, hz(note) / 2, {
					vel: 0.5,
					dur: 1.1,
					pan: -0.4 + i * 0.12,
					verb: 0.6,
					gain: (o.bed ? 0.55 : 0.65) * k,
					seed: 510 + i
				});
			else
				glass(h, w, note, {
					decay: 1.6,
					gain: (o.bed ? 0.5 : 0.55) * k,
					pan: -0.45 + i * 0.15,
					verb: 0.6,
					seed: 520 + i
				});
		})
	);
	tl.sound(o.close - 0.1, (w, b) => {
		const h = house(sound, b);
		air(h, w, 1.0, { peakAt: 0.5, band: [500, 3200], pans: [-0.4, 0.8], gain: 0.3 * k, seed: 503 });
	});
	if (o.bed) return;
	const D = o.D;
	tl.sound(0, (w, b) => {
		const h = house(sound, b);
		const chord = o.dark ? ['D3', 'A3', 'B3', 'F#4'] : ['D3', 'A3', 'E4', 'F#4'];
		pad(h, w, chord, Math.max(1, o.tagAt - 1.6), {
			attack: 1.4,
			release: 1.8,
			level: 1,
			seed: 530
		});
		padSweep(h, w, [
			[0, 420],
			[o.tTime, 1500],
			[o.close, 2600],
			[o.tagAt - 0.4, 1200],
			[D, 800]
		]);
		sub(h, w + 0.3, 'D2', Math.max(1, o.tagAt - 1.8), { gain: 0.45, attack: 1.2, release: 1.4 });
		// the piano walks the chord on the beat, from the time to the tag
		const walk = o.dark ? ['D3', 'A3', 'F#3', 'E3'] : ['D4', 'A4', 'F#4', 'E4'];
		for (let i = 0, at = o.tTime; at < o.tagAt - 0.9; i++, at += 2 * BEAT)
			piano(h, w + at, walk[i % walk.length], {
				vel: 0.4,
				dur: 1.4,
				pan: i % 2 ? 0.2 : -0.2,
				verb: 0.55,
				gain: 0.85,
				seed: 540 + i
			});
	});
}
