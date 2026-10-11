// The promo: a timeless body over the title's own soundtrack, then a 4 s end card with the
// airing. Sections start at nominal times snapped to the bed's onsets; fast sections cut on
// every beat. The body never shows anything about when the title airs, so it renders once per
// title: the end card's timeless layer (backdrop, poster, title) may enter during the body's
// last moments, but the airing (eyebrow, day, time) only appears from the cut on, and the
// layout keeps a fixed place for it. The mood decides how sections move and sound, not what
// they say.

import type { Bus, LoadedBed } from '../audio/engine';
import { beatsIn, snapper } from '../audio/onsets';
import { SPARKLE } from '../audio/kit';
import { CHANNEL, channelTag, springFrames } from '../brand/lockup';
import { chars, css, div, fadeOut, fadeUp, fit, glint, img, rise, span, words } from '../core/dom';
import { SPRING } from '../core/motion';
import { applyPalette } from '../core/palette';
import { rng } from '../core/rng';
import { frame } from '../core/timeline';
import type { Airing, EvergreenEndSpec, PromoContent, PromoSpec, Shot } from '../spec';
import { addShot } from './shots';
import {
	glow,
	grain,
	rgbOf,
	shake,
	timeline,
	titlePalette,
	type Ctx,
	type Template,
	type TL
} from './shared';

type Mood = PromoContent['mood'];

type Section =
	| { kind: 'line'; at: number; line: number; snap?: false }
	| { kind: 'montage'; at: number; word?: string | null }
	| { kind: 'slam'; at: number; word: string }
	| { kind: 'title'; at: number }
	| { kind: 'type'; at: number; line: number; shot?: boolean; snap?: false }
	| { kind: 'card'; at: number; word: string }
	| { kind: 'shots'; at: number }
	| { kind: 'word'; at: number; word: string }
	| { kind: 'slab'; at: number; line: number; snap?: false }
	| { kind: 'end'; at: number };

interface MoodSpec {
	dissolve: number;
	minShot: number;
	fastShot: number;
	cutMin: number;
	grain: number;
	/** Seconds before the cut the end card's timeless layer starts to enter. */
	lead: number;
	sequence: (premiere: boolean, words: string[]) => Section[];
}

const MOODS: Record<Mood, MoodSpec> = {
	cinematic: {
		dissolve: 0.45,
		minShot: 1.7,
		fastShot: 0.62,
		cutMin: 0.55,
		grain: 0.045,
		lead: 0.8,
		sequence: (premiere, w) => [
			{ kind: 'line', at: 0, line: 0, snap: false },
			{ kind: 'line', at: 4.0, line: 1 },
			{ kind: 'montage', at: 8.8, word: w[0] ?? null },
			...(premiere
				? ([
						{ kind: 'slam', at: 12.4, word: 'Premiere' },
						{ kind: 'title', at: 14.6 },
						{ kind: 'end', at: 18.8 }
					] as Section[])
				: ([
						{ kind: 'title', at: 12.4 },
						{ kind: 'end', at: 16.6 }
					] as Section[]))
		]
	},
	grit: {
		dissolve: 0,
		minShot: 0.6,
		fastShot: 0.6,
		cutMin: 0.25,
		grain: 0.13,
		lead: 0,
		sequence: (premiere, w) => [
			{ kind: 'type', at: 0, line: 0, snap: false },
			{ kind: 'shots', at: 1.6 },
			{ kind: 'card', at: 3.6, word: premiere ? 'Premiere' : (w[0] ?? 'Tonight') },
			{ kind: 'title', at: 5.5 },
			{ kind: 'word', at: 8.3, word: premiere ? (w[0] ?? '') : (w[1] ?? '') },
			{ kind: 'type', at: 10.4, line: 1, shot: true },
			{ kind: 'end', at: 12.6 }
		]
	},
	bright: {
		dissolve: 0.3,
		minShot: 1.2,
		fastShot: 0.42,
		cutMin: 0.4,
		grain: 0,
		lead: 0.6,
		sequence: (premiere, w) => [
			{ kind: 'slab', at: 0, line: 0, snap: false },
			{ kind: 'montage', at: 3.2, word: premiere ? 'Premiere' : (w[0] ?? null) },
			{ kind: 'slab', at: 6.4, line: 1 },
			{ kind: 'title', at: 9.6 },
			{ kind: 'end', at: 13.2 }
		]
	}
};

/** How long the end card runs after the cut. */
export const END_CARD = 4;

// the body is rendered with this airing whatever the spec says, so it cannot depend on one
const BODY_AIRING: Airing = { eyebrow: 'Tonight', time: '20:15' };

const CUE: Record<Section['kind'], string> = {
	line: 'tagline (long take)',
	montage: 'montage, cuts on beats',
	slam: 'premiere slam',
	title: 'title wordmark',
	type: 'typewriter line',
	card: 'chapter card',
	shots: 'beat-cut shots',
	word: 'kinetic word',
	slab: 'tagline on a slab',
	end: 'end card (airing)'
};

export const promo: Template<PromoSpec> = {
	version: 1,
	build(ctx, spec) {
		const airing = spec.part === 'body' ? BODY_AIRING : (spec.airing ?? null);
		const tl = buildPromo(ctx, spec, airing);
		if (spec.part !== 'full') {
			const [a, b] = tl.parts[spec.part];
			tl.parts = { clip: [a, b] };
		}
		return tl;
	}
};

export const evergreenEnd: Template<EvergreenEndSpec> = {
	version: 1,
	build(ctx, spec) {
		const tl = buildPromo(ctx, spec, null);
		tl.parts = { clip: tl.parts.end };
		return tl;
	}
};

function defaultWords(c: PromoContent): string[] {
	if (c.words?.length) return c.words;
	const genre = c.title.genres[0];
	const year = c.title.year ? String(c.title.year) : undefined;
	if (c.mood === 'cinematic') return [];
	return [genre, year].filter((x): x is string => !!x);
}

/** The whole promo (body and end card); `airing` null is the evergreen end card. */
function buildPromo({ root, sound, assets }: Ctx, spec: PromoContent, airing: Airing | null): TL {
	const t = spec.title;
	const mood = spec.mood;
	const P = MOODS[mood];
	const grit = mood === 'grit';
	const bright = mood === 'bright';
	const bed: LoadedBed | null = assets.bed(spec.bed);
	const onsets = bed?.onsets ?? [];
	const snap = snapper(onsets);

	const nominal = P.sequence(!!spec.premiere, defaultWords(spec));
	const seq = nominal.map((s) => ({
		...s,
		at: 'snap' in s && s.snap === false ? s.at : snap(s.at, 0.7, 0.5)
	})) as (Section & { end: number })[];
	// the cut to the end card lands on the frame grid (the body and the end are separate clips)
	const last = seq[seq.length - 1];
	last.at = frame(last.at);
	const B = last.at;
	const D = frame(B + END_CARD);
	seq.forEach((s, i) => (s.end = i + 1 < seq.length ? seq[i + 1].at : D));

	const tl = timeline(root, D);
	tl.parts = { body: [0, B], end: [B, D], full: [0, D] };
	const pal = titlePalette(t);
	applyPalette(root, pal);
	root.classList.add('promo', `mood-${mood}`);

	const world = div('fill', root);
	const shots = div('fill', world);
	// a constant grade over every shot (text always sits on a darker, accent-tinted picture),
	// plus a heavier dim where big type sits mid-frame
	css(div('fill', world), {
		background: `linear-gradient(0deg, rgb(0 0 0 / 0.62) 0%, rgb(0 0 0 / 0.08) 45%, rgb(0 0 0 / 0.25) 100%), radial-gradient(ellipse 80% 70% at 50% 45%, transparent 40%, rgb(${rgbOf(pal.dark)} / 0.55) 100%)`
	});
	const dim = css(div('fill', world), { background: 'rgb(0 0 0 / 0.42)', opacity: '0' });
	const blackA = css(div('fill', world), { background: '#000', visibility: 'hidden' });
	const glowA = glow(world, 'accent', 960, 560, 760);
	const text = div('fill', world);
	const endLayer = div('fill', root);
	const vig = div('vignette', root);
	if (grit)
		vig.style.background =
			'radial-gradient(ellipse 70% 65% at 50% 50%, transparent 45%, rgb(0 0 0 / 0.75) 100%)';
	if (P.grain) grain(tl, root, P.grain, grit ? 9 : 4);
	const flash = div('flash', root);
	let lbT: HTMLElement | null = null;
	let lbB: HTMLElement | null = null;
	if (grit) {
		lbT = div('letterbox top', root);
		lbB = div('letterbox bottom', root);
		// gate weave: the picture breathes a pixel or two, like film in a projector
		const r = rng(31);
		const frames = Array.from({ length: 10 }, () => ({
			transform: `translate(${(r() * 2.4 - 1.2).toFixed(2)}px, ${(r() * 3 - 1.5).toFixed(2)}px)`
		}));
		tl.add(shots, frames, 0, 0.42, 'steps(10, jump-none)', { iterations: Math.ceil(D / 0.42) });
	}
	const black = (a: number, b: number) => tl.show(blackA, a, b);
	tl.add(glowA, [{ opacity: 0 }, { opacity: 0 }], 0, 0.01, 'linear');

	const pool: Shot[] = spec.shots.length
		? spec.shots
		: [{ kind: 'still', image: 'backdrop', from: [0, 0, 1, 1] }];
	let shotIndex = 0;
	const nextShot = () => pool[shotIndex++ % pool.length];
	const lines = t.taglines ?? [];

	const shot = (
		a: number,
		b: number,
		s: Shot = nextShot(),
		o: { dissolve?: number; punch?: boolean; opacity?: number } = {}
	) =>
		addShot(tl, shots, t, assets, s, a, b, {
			dissolve: o.dissolve ?? P.dissolve,
			punch: o.punch ?? grit,
			opacity: o.opacity
		});

	/** Cut points in [a, b): the bed's beats, or a steady grid without a bed. */
	const cutsIn = (a: number, b: number, gap: number) => {
		if (onsets.length) return [a, ...beatsIn(onsets, a, b, gap, P.cutMin)];
		const out = [a];
		for (let c = a + gap * 1.4; c < b - gap * 0.6; c += gap * 1.4) out.push(c);
		return out;
	};

	function shotRun(a: number, b: number, fast: boolean) {
		const cuts = cutsIn(a, b, fast ? P.fastShot : P.minShot);
		cuts.forEach((c, i) => {
			const e = i + 1 < cuts.length ? cuts[i + 1] : b;
			shot(c, e, nextShot(), {
				dissolve: fast && !grit ? 0 : P.dissolve,
				punch: grit || (bright && fast)
			});
			if (i === 0) return;
			if (grit)
				tl.sound(c, (w, bus) => sound.punch(bus, w, { gain: 0.22, pan: i % 2 ? 0.3 : -0.3 }));
			else if (bright)
				tl.sound(c - 0.06, (w, bus) =>
					sound.whoosh(bus, w, {
						dur: 0.3,
						pan: i % 2 ? [0.6, -0.2] : [-0.6, 0.2],
						gain: 0.035,
						lo: 900,
						hi: 5200,
						body: 0.1
					})
				);
			else
				tl.sound(c - 0.12, (w, bus) =>
					sound.whoosh(bus, w, {
						dur: 0.5,
						pan: i % 2 ? [0.5, -0.2] : [-0.5, 0.2],
						gain: 0.03,
						body: 0.2
					})
				);
		});
		return cuts;
	}

	function flashAt(a: number, peak = 0.85, dur = grit ? 0.09 : 0.35) {
		tl.add(flash, [{ opacity: peak }, { opacity: 0 }], a, dur, grit ? 'linear' : 'out', {
			fill: 'forwards'
		});
	}

	// each section's text exists only inside its section
	const centerBox = (a: number, b: number) => {
		const box = div('center', text);
		tl.show(box, a, b + 0.001);
		return box;
	};
	const dimFor = (a: number, b: number) => {
		tl.add(dim, [{ opacity: 0 }, { opacity: 1 }], a, 0.25, 'sine', { fill: 'forwards' });
		tl.add(dim, [{ opacity: 1 }, { opacity: 0 }], b - 0.2, 0.2, 'sine', { fill: 'forwards' });
	};

	for (const s of seq) {
		const a = s.at;
		const b = s.end;
		tl.cue(CUE[s.kind], a);
		switch (s.kind) {
			case 'line': {
				shot(a, b);
				if (a === 0) tl.add(shots, [{ opacity: 0 }, { opacity: 1 }], 0, 1.4, 'sine');
				const line = lines[s.line];
				if (!line) break;
				const box = div('promo-line', text);
				tl.show(box, a, b + 0.001);
				const ws = words(box, line);
				ws.root.style.display = 'inline-block';
				rise(tl, ws.items, a + (a === 0 ? 1.0 : 0.35), { stagger: 0.12, dur: 1.1 });
				// trailer type never sits still: a slow push for the life of the line
				tl.add(
					ws.root,
					[{ transform: 'scale(1)' }, { transform: 'scale(1.045)' }],
					a,
					b - a,
					'linear'
				);
				fadeOut(tl, box, b - 0.45, { dur: 0.45, y: -10 });
				if (a > 0)
					tl.sound(a, (w, bus) =>
						sound.impact(bus, w, { gain: 0.16, size: 1.1, tone: 44, click: 0.2, verb: 0.35 })
					);
				break;
			}
			case 'montage': {
				shotRun(a, b, true);
				if (!s.word) break;
				dimFor(a, b);
				const box = centerBox(a, b);
				const wordEl = div('promo-word', box);
				const cw = chars(wordEl, s.word);
				fit(wordEl, 1640);
				cw.root.style.display = 'inline-block';
				if (bright) {
					cw.items.forEach((c, i) => {
						const sp = springFrames(
							0,
							1,
							SPRING.bouncy,
							(v) =>
								`translateY(${((1 - v) * 70).toFixed(1)}%) scale(${(0.6 + 0.4 * v).toFixed(3)})`
						);
						tl.add(c, [{ opacity: 0 }, { opacity: 1 }], a + i * 0.035, 0.12, 'linear');
						tl.add(c, sp.frames, a + i * 0.035, sp.dur, 'linear');
					});
					tl.sound(a, (w, bus) =>
						sound.impact(bus, w, { gain: 0.22, size: 0.7, tone: 54, click: 0.6 })
					);
					tl.sound(a + 0.1, (w, bus) =>
						sound.shimmer(bus, w, { dur: 0.7, gain: 0.025, count: 9, seed: 5 })
					);
				} else {
					const mid = (cw.items.length - 1) / 2;
					cw.items.forEach((c, i) =>
						tl.add(
							c,
							[
								{ opacity: 0, transform: `translateX(${(i - mid) * 46}px)` },
								{ opacity: 1, transform: 'translateX(0)' }
							],
							a + 0.05,
							1.5,
							'enter'
						)
					);
					tl.add(
						cw.root,
						[{ transform: 'scale(1)' }, { transform: 'scale(1.035)' }],
						a,
						b - a,
						'linear'
					);
					tl.sound(a, (w, bus) =>
						sound.impact(bus, w, { gain: 0.32, size: 1.2, tone: 46, click: 0.5 })
					);
					tl.sound(a - 0.1, (w, bus) =>
						sound.whoosh(bus, w, { dur: 0.9, pan: [-0.6, 0.6], gain: 0.06 })
					);
				}
				tl.add(
					wordEl,
					[
						{ opacity: 1, transform: 'scale(1)' },
						{ opacity: 0, transform: 'scale(1.06)' }
					],
					b - 0.22,
					0.22,
					'exit'
				);
				break;
			}
			case 'slam': {
				black(a, b);
				const box = centerBox(a, b);
				const eb = div('eyebrow promo-time-eyebrow', box, CHANNEL);
				const slamEl = div('promo-slam', box);
				const tc = chars(slamEl, s.word);
				slamEl.style.display = 'inline-block';
				fit(slamEl, 1640);
				tl.add(
					glowA,
					[
						{ opacity: 0, transform: 'scale(0.7)' },
						{ opacity: 0.9, transform: 'scale(1)' }
					],
					a,
					0.9,
					'out'
				);
				tl.add(glowA, [{ opacity: 0.9 }, { opacity: 0 }], b - 0.3, 0.3, 'sine');
				flashAt(a, 0.55);
				tl.add(
					slamEl,
					[
						{ opacity: 0, transform: 'scale(1.16)' },
						{ opacity: 1, transform: 'scale(1)' }
					],
					a,
					0.7,
					'enter'
				);
				rise(tl, tc.items, a, { stagger: 0.05, dur: 0.7, from: '40%' });
				glint(tl, slamEl, a + 0.55, 1.0);
				fadeUp(tl, eb, a + 0.25, { dur: 0.7, y: 10 });
				tl.add(
					box,
					[
						{ opacity: 1, transform: 'scale(1)' },
						{ opacity: 0, transform: 'scale(1.04)' }
					],
					b - 0.3,
					0.3,
					'exit'
				);
				tl.sound(a - 1.3, (w, bus) =>
					sound.riser(bus, w, { dur: 1.3, gain: 0.06, tone: 0.4, root: 98 })
				);
				tl.sound(a, (w, bus) => sound.impact(bus, w, { gain: 0.55, size: 1.3, tone: 42 }));
				tl.sound(a + 0.55, (w, bus) => sound.shimmer(bus, w, { dur: 1.0, gain: 0.03, seed: 2 }));
				break;
			}
			case 'title': {
				shot(a, b, spec.hero ?? nextShot(), { dissolve: grit ? 0 : 0.3 });
				dimFor(a, b);
				const box = centerBox(a, b);
				const tEl = div('promo-title', box);
				const tc = chars(tEl, t.name);
				tEl.style.display = 'inline-block';
				fit(tEl, 1660);
				const under = grit || bright ? div('card-under', box) : null;
				const meta = div('promo-meta', box);
				[
					t.year ? String(t.year) : '',
					t.genres.slice(0, 3).join(' / '),
					t.runtimeMinutes ? `${t.runtimeMinutes} min` : ''
				]
					.filter(Boolean)
					.forEach((x, i) => span(i ? 'dot-sep' : '', meta, x));
				if (grit && under) {
					black(a, a + 0.12);
					tl.add(
						tEl,
						[
							{ opacity: 0, transform: 'scale(1.6)' },
							{ opacity: 1, transform: 'scale(1)' }
						],
						a,
						0.2,
						'snap'
					);
					shake(tl, world, a, { amp: 22, seed: 13, dur: 0.45 });
					flashAt(a, 0.95, 0.12);
					css(under, { width: tEl.offsetWidth * 0.5 + 'px', marginTop: '14px' });
					tl.add(
						under,
						[{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }],
						a + 0.18,
						0.3,
						'snap'
					);
					fadeUp(tl, meta, a + 0.5, { y: 8, dur: 0.4 });
					tl.sound(a, (w, bus) => sound.impact(bus, w, { gain: 0.6, size: 1.1, tone: 44 }));
					tl.sound(a, (w, bus) => sound.punch(bus, w, { gain: 0.4 }));
				} else if (bright && under) {
					tc.items.forEach((c, i) => {
						const sp = springFrames(0.3, 1, SPRING.bouncy);
						tl.add(c, [{ opacity: 0 }, { opacity: 1 }], a + 0.1 + i * 0.035, 0.1, 'linear');
						tl.add(c, sp.frames, a + 0.1 + i * 0.035, sp.dur, 'linear');
					});
					css(under, { width: tEl.offsetWidth * 0.4 + 'px', marginTop: '20px' });
					tl.add(
						under,
						[{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }],
						a + 0.5,
						0.45,
						'snap'
					);
					fadeUp(tl, meta, a + 0.8, { y: 12, dur: 0.8 });
					tl.sound(a + 0.1, (w, bus) =>
						sound.impact(bus, w, { gain: 0.3, size: 0.8, tone: 50, click: 0.5 })
					);
					tl.sound(a + 0.5, (w, bus) => sound.shimmer(bus, w, { dur: 1.1, gain: 0.03, seed: 9 }));
				} else {
					rise(tl, tc.items, a + 0.15, { stagger: 0.045, dur: 1.1, from: '60%' });
					glint(tl, tEl, a + 1.0, 1.2);
					fadeUp(tl, meta, a + 1.0, { y: 12, dur: 1.0 });
					tl.sound(a - 0.15, (w, bus) =>
						sound.whoosh(bus, w, { dur: 0.8, pan: [0.5, -0.5], gain: 0.05 })
					);
					tl.sound(a + 1.0, (w, bus) => sound.shimmer(bus, w, { dur: 1.2, gain: 0.03, seed: 9 }));
				}
				tl.add(
					box,
					[
						{ opacity: 1, transform: 'scale(1)' },
						{ opacity: 0, transform: 'scale(1.03)' }
					],
					b - 0.35,
					0.35,
					'exit'
				);
				break;
			}
			case 'type': {
				const line = lines[s.line];
				if (!line) {
					shotRun(a, b, true);
					break;
				}
				if (s.shot) shot(a, b, nextShot(), { dissolve: 0, punch: false, opacity: 0.42 });
				else black(a, b);
				const box = centerBox(a, b);
				const lineEl = div('type-line', box);
				const tc = chars(lineEl, line);
				const caret = span('caret', lineEl);
				const step = 0.055;
				const startT = a + 0.25;
				const cx = (x: number) => [
					{ transform: `translateX(${x}px)` },
					{ transform: `translateX(${x}px)` }
				];
				tl.add(caret, cx(0), a, 0.01, 'linear');
				let pause = 0;
				tc.items.forEach((c, i) => {
					if (/[.,]/.test(tc.items[i - 1]?.textContent ?? '')) pause += 0.18;
					const at = startT + i * step + pause;
					if (at >= b - 0.1) return;
					tl.add(c, [{ opacity: 0 }, { opacity: 1 }], at, 0.01, 'linear');
					tl.add(caret, cx(c.offsetLeft + c.offsetWidth + 4), at, 0.01, 'linear');
					tl.sound(at, (w, bus) =>
						sound.key(bus, w, { gain: 0.07, pan: (i / tc.items.length) * 0.8 - 0.4, seed: i + 3 })
					);
				});
				tl.add(
					caret,
					[
						{ opacity: 1 },
						{ opacity: 1, offset: 0.5 },
						{ opacity: 0, offset: 0.5 },
						{ opacity: 0 }
					],
					a,
					0.6,
					'linear',
					{ iterations: Math.ceil((b - a) / 0.6) }
				);
				tl.sound(a, (w, bus, skip) => sound.projector(bus, w, b - a, skip, { gain: 0.05 }), b - a);
				tl.add(box, [{ opacity: 1 }, { opacity: 0 }], b - 0.08, 0.08, 'linear');
				break;
			}
			case 'card': {
				black(a, b);
				const box = centerBox(a, b);
				const inner = div(null, box);
				const w = div('card-word', inner, s.word);
				fit(w, 1640);
				const under = div('card-under', inner);
				tl.add(
					w,
					[
						{ opacity: 0, transform: 'scale(1.35)' },
						{ opacity: 1, transform: 'scale(1)' }
					],
					a,
					0.16,
					'snap'
				);
				tl.add(
					under,
					[{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }],
					a + 0.12,
					0.35,
					'snap'
				);
				shake(tl, world, a, { amp: 14, seed: 21 });
				flashAt(a, 0.7);
				tl.sound(a, (w2, bus) => sound.punch(bus, w2, { gain: 0.5 }));
				tl.add(box, [{ opacity: 1 }, { opacity: 0 }], b - 0.06, 0.06, 'linear');
				break;
			}
			case 'shots': {
				shotRun(a, b, true);
				flashAt(a, 0.6);
				tl.sound(a, (w, bus) => sound.punch(bus, w, { gain: 0.45 }));
				break;
			}
			case 'word': {
				shotRun(a, b, true);
				if (!s.word) break;
				dimFor(a, b);
				const box = centerBox(a, b);
				const wordEl = div('promo-word', box, s.word);
				fit(wordEl, 1640);
				tl.add(
					wordEl,
					[
						{ opacity: 0, transform: 'scale(1.3)' },
						{ opacity: 1, transform: 'scale(1)' }
					],
					a,
					0.18,
					'snap'
				);
				shake(tl, world, a, { amp: 12, seed: 4 });
				flashAt(a, 0.5);
				tl.sound(a, (w, bus) => sound.punch(bus, w, { gain: 0.42 }));
				tl.add(
					wordEl,
					[
						{ opacity: 1, transform: 'scale(1)' },
						{ opacity: 0, transform: 'scale(1.06)' }
					],
					b - 0.22,
					0.22,
					'exit'
				);
				break;
			}
			case 'slab': {
				shot(a, b);
				if (a === 0) tl.add(shots, [{ opacity: 0 }, { opacity: 1 }], 0, 0.6, 'sine');
				const line = lines[s.line];
				if (!line) break;
				const box = div('promo-slab', text);
				tl.show(box, a, b + 0.001);
				const slab = div('slab-bg', box);
				const ws = words(box, line);
				ws.root.classList.add('slab-text');
				const at0 = a + (a === 0 ? 0.5 : 0.25);
				tl.add(slab, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], at0, 0.4, 'snap');
				ws.items.forEach((it, i) => {
					const sp = springFrames(0.5, 1, SPRING.bouncy);
					tl.add(it, [{ opacity: 0 }, { opacity: 1 }], at0 + 0.15 + i * 0.08, 0.1, 'linear');
					tl.add(it, sp.frames, at0 + 0.15 + i * 0.08, sp.dur, 'linear');
					tl.sound(at0 + 0.15 + i * 0.08, (w, bus) =>
						sound.tick(bus, w, {
							freq: SPARKLE[i % SPARKLE.length],
							gain: 0.03,
							pan: -0.4 + i * 0.12
						})
					);
				});
				tl.add(
					slab,
					[{ transform: 'scaleX(1)' }, { transform: 'scaleX(0)' }],
					b - 0.3,
					0.3,
					'exit'
				);
				tl.add(ws.root, [{ opacity: 1 }, { opacity: 0 }], b - 0.3, 0.2, 'exit');
				tl.sound(at0 - 0.05, (w, bus) =>
					sound.whoosh(bus, w, {
						dur: 0.45,
						pan: [-0.6, 0.1],
						gain: 0.04,
						lo: 700,
						hi: 4200,
						body: 0.2
					})
				);
				break;
			}
			case 'end':
				endCard(a);
				break;
		}
	}

	function endCard(at: number) {
		const lead = P.lead;
		const T = at - lead;
		const layer = endLayer;
		tl.show(layer, T, D + 1);
		if (lead)
			tl.add(
				layer,
				bright
					? [
							{ opacity: 0, transform: 'translateX(6%)' },
							{ opacity: 1, transform: 'translateX(0)' }
						]
					: [{ opacity: 0 }, { opacity: 1 }],
				T,
				lead + 0.1,
				bright ? 'enter' : 'sine'
			);
		const bgWrap = div('fill', layer);
		css(img('fill', bgWrap, assets.blurred(t.backdrop)), {
			width: '100%',
			height: '100%',
			objectFit: 'cover',
			opacity: '0.5'
		});
		css(div('fill', layer), {
			background:
				'linear-gradient(90deg, rgb(7 8 13 / 0.5), rgb(7 8 13 / 0.86) 55%, rgb(7 8 13 / 0.94))'
		});
		const gA = glow(layer, 'accent', 450, 540, 640);
		const posterCard = div('card ec-poster', layer);
		const pImg = img(null, posterCard, assets.picture(t.poster).url);
		const info = div('ec-info', layer);
		const ebRow = div('un-eyebrow', info);
		const rail = css(div('rail', ebRow), { width: '56px' });
		const title = words(div('hero ec-title', info), t.name);
		fit(title.root.parentElement as HTMLElement, 980);
		// a fixed-height row: the airing never moves the timeless layout around it
		const whenRow = div('ec-when', info);
		const metaRow = div('ec-meta', info);
		const chipsEl = [
			t.year ? String(t.year) : '',
			...t.genres.slice(0, 3),
			t.runtimeMinutes ? `${t.runtimeMinutes} min` : ''
		]
			.filter(Boolean)
			.map((x) => span('chip', metaRow, x));

		// the timeless layer
		tl.add(bgWrap, [{ transform: 'scale(1.12)' }, { transform: 'scale(1)' }], T, D - T, 'out');
		tl.add(gA, [{ opacity: 0 }, { opacity: 0.8 }], T, 1.4, 'sine');
		if (grit) {
			if (lbT && lbB) {
				tl.add(
					lbT,
					[{ transform: 'translateY(0)' }, { transform: 'translateY(-100%)' }],
					at + 0.1,
					0.7,
					'inOut'
				);
				tl.add(
					lbB,
					[{ transform: 'translateY(0)' }, { transform: 'translateY(100%)' }],
					at + 0.1,
					0.7,
					'inOut'
				);
			}
			tl.add(
				posterCard,
				[
					{ opacity: 0, transform: 'translateX(-60px) rotate(-2deg) scale(1.04)' },
					{ opacity: 1, transform: 'translateX(0) rotate(0deg) scale(1)' }
				],
				at,
				0.32,
				'snap'
			);
			flashAt(at, 0.6);
			tl.sound(at, (w, bus) => sound.punch(bus, w, { gain: 0.42 }));
		} else if (bright) {
			const sp = springFrames(0.85, 1, SPRING.bouncy);
			tl.add(posterCard, [{ opacity: 0 }, { opacity: 1 }], T, 0.2, 'linear');
			tl.add(posterCard, sp.frames, T, sp.dur, 'linear');
			tl.sound(T - 0.1, (w, bus) =>
				sound.whoosh(bus, w, { dur: 0.6, pan: [0.7, 0.1], gain: 0.045, lo: 700, hi: 4000 })
			);
		} else {
			tl.add(
				posterCard,
				[
					{ opacity: 0, transform: 'translateY(50px) scale(0.96)' },
					{ opacity: 1, transform: 'translateY(0) scale(1)' }
				],
				T,
				1.1,
				'enter'
			);
			tl.sound(T - 0.2, (w, bus) =>
				sound.whoosh(bus, w, { dur: 0.9, pan: [-0.7, -0.2], gain: 0.05 })
			);
		}
		tl.add(pImg, [{ transform: 'scale(1.08)' }, { transform: 'scale(1)' }], T, D - T, 'out');
		const i0 = grit ? at + 0.2 : T + 0.15;
		tl.add(rail, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], i0, 0.6, 'enter');
		rise(tl, title.items, i0 + 0.1, {
			stagger: 0.08,
			dur: grit ? 0.5 : 0.9,
			ease: grit ? 'snap' : 'enter'
		});
		chipsEl.forEach((c, i) => fadeUp(tl, c, i0 + 0.35 + i * 0.06, { y: 10, dur: 0.6 }));

		// the airing: nothing of it exists before the cut
		const eyebrowText = airing ? airing.eyebrow : 'Coming up';
		const eb = div('eyebrow', ebRow, eyebrowText);
		tl.show(eb, at, D + 1);
		tl.add(
			eb,
			[
				{ opacity: 0, transform: 'translateX(-14px)' },
				{ opacity: 1, transform: 'translateX(0)' }
			],
			at + 0.05,
			0.6,
			'enter'
		);
		const whenAt = at + (grit ? 0.15 : 0.3);
		if (airing) {
			const day = airing.day ? span('ec-day', whenRow, airing.day) : null;
			const timeEl = span(bright ? 'ec-time ec-pill tnum' : 'ec-time tnum', whenRow, airing.time);
			timeEl.style.display = 'inline-block';
			if (day) {
				tl.show(day, at, D + 1);
				fadeUp(tl, day, whenAt, { y: 14, dur: 0.6 });
			}
			tl.show(timeEl, at, D + 1);
			if (bright) {
				const sp = springFrames(0.4, 1, SPRING.bouncy);
				tl.add(timeEl, [{ opacity: 0 }, { opacity: 1 }], whenAt + 0.1, 0.12, 'linear');
				tl.add(timeEl, sp.frames, whenAt + 0.1, sp.dur, 'linear');
			} else {
				tl.add(
					timeEl,
					[
						{ opacity: 0, transform: `translateY(${grit ? 0 : 24}px) scale(${grit ? 1.3 : 1})` },
						{ opacity: 1, transform: 'translateY(0) scale(1)' }
					],
					whenAt + 0.1,
					grit ? 0.18 : 0.7,
					grit ? 'snap' : 'enter'
				);
				if (!grit) glint(tl, timeEl, whenAt + 0.75, 0.8);
			}
			tl.sound(whenAt + 0.1, (w, bus) =>
				grit
					? sound.punch(bus, w, { gain: 0.3 })
					: sound.impact(bus, w, { gain: 0.22, size: 0.8, tone: 48, click: 0.3 })
			);
			tl.sound(whenAt + 0.12, (w, bus) =>
				sound.tick(bus, w, { freq: SPARKLE[1], gain: 0.04, pan: 0.2 })
			);
		} else {
			const on = span('ec-time ec-channel', whenRow, `on ${CHANNEL}`);
			tl.show(on, at, D + 1);
			fadeUp(tl, on, whenAt + 0.1, { y: 20, dur: 0.7 });
			tl.sound(whenAt + 0.1, (w, bus) =>
				sound.tick(bus, w, { freq: SPARKLE[1], gain: 0.04, pan: 0.2 })
			);
		}

		const tagAt = at + 1.3;
		// after the tape stop the tag plays alone, so it carries more of the weight
		channelTag(tl, layer, tagAt, sound, { gain: grit ? 0.72 : 0.5 });

		const blk = css(div('fill', root), { background: '#000' });
		tl.add(blk, [{ opacity: 0 }, { opacity: 1 }], D - 0.6, 0.6, 'sine');

		const bedOpts = grit
			? { fadeIn: 0.05, fadeOut: 0.6, end: D, gain: 1, tapeStop: { at: tagAt - 0.62, dur: 0.55 } }
			: {
					fadeIn: 0.2,
					fadeOut: 1.6,
					end: D,
					gain: 1,
					ducks: [{ at: tagAt, depth: bright ? 0.5 : 0.42, hold: 1.2, attack: 0.25, release: 0.8 }]
				};
		tl.sound(0, (w, bus: Bus, skip) => sound.bed(bus, w, bed, skip, bedOpts), D);
		if (grit) tl.cue('tape stop', tagAt - 0.62);
	}

	return tl;
}
