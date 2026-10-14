// The news (DESIGN.md 2.11, sharp), a minute long: an open (the header, the segments and a card
// that names the stories and where they come from), the top Czech headlines one at a time, each
// whole and as the outlet wrote it, labelled with the outlet's name in plain text, then a recap
// of all of them together and the channel's sign-off. English chrome frames Czech content the
// way a quotation is framed: a red bar beside the story, a red segment per story along the top
// lighting as its story comes and a counter at the header's end; the headline is the large text,
// set at 80 px where it fits on three lines and smaller only for the longest. Between the changes
// nothing moves. The music is one score in D, a felt-piano ostinato over a soft pulse and the
// pad, its harmony stepping from story to story and resolving into D major for the recap, which
// is where the sonic logo's chord already lives.

import {
	air,
	bowed,
	glass,
	house,
	pad,
	padSweep,
	piano,
	pulse,
	sub,
	type PadOptions
} from '../audio/voices';
import { css, div, span } from '../core/dom';
import { fitText } from '../core/fit';
import type { NewsSpec, NewsStory } from '../spec';
import {
	BEAT,
	bulletinHouse,
	credits,
	exit,
	field,
	head,
	headIn,
	headOut,
	lift,
	list,
	mark,
	SAFE_RIGHT,
	share,
	signOff
} from './bulletin';
import { timeline, type Template } from './shared';

/** The story's place: its bar at x 96, the text 48 px in, centred between the segments and the foot. */
const STORY = { x: 96, indent: 48, mid: 668 };
const HEADLINE = { size: 80, min: 72, lines: 3, leading: 1.14 };
const SEGMENTS = { y: 330, gap: 16 };
/** The recap's rows run between the segments' line and the tag's top. */
const RECAP = { top: 330, bottom: 892, who: 290 };

/** How long a story takes to read, seconds. */
const reading = (s: NewsStory) => Math.max(3.2, 1.4 + s.headline.length * 0.038);

/** The bulletin's parts, in seconds: the open, each story's window, and where the recap starts. */
export interface Running {
	open: number;
	stories: [number, number][];
	recap: number;
}

/**
 * Where each part falls: the open, every story (its reading time and 3.5 beats to come and
 * go, held between 9 and 16 beats) and the recap with the sign-off, in whole beats so every
 * change lands on the music, filling the bulletin to its last frame. Fewer stories hold longer
 * and widen the open and the recap rather than leave a story standing still.
 */
export function running(stories: NewsStory[], D: number): Running {
	const n = stories.length;
	const parts = [
		{ w: 4, min: 3, max: 6 },
		...stories.map((s) => ({ w: reading(s) / BEAT + 3.5, min: 9, max: 16 })),
		{ w: 4.5 + 1.5 * n, min: 9, max: 16 }
	];
	const beats = share(parts, Math.round(D / BEAT));
	const edges = [0];
	let at = 0;
	for (const b of beats) edges.push(Math.round((at += b) * BEAT * 1000) / 1000);
	return {
		open: edges[1],
		stories: stories.map((_, i) => [edges[i + 1], edges[i + 2]]),
		recap: edges[n + 1]
	};
}

/**
 * The harmony's steps: D minor (add 9), B flat, G minor, F and A suspended, each with the
 * chord's pad, the low and high note of its ostinato (and the octave above the low), its
 * root for the pulse, the glass note its story opens on and the bowed line of the sparse figure.
 */
const STEPS = [
	{
		pad: ['D3', 'A3', 'E4', 'F4'],
		low: 'D3',
		high: 'A3',
		octave: 'D4',
		root: 'D2',
		bell: 'A5',
		line: 'F4'
	},
	{
		pad: ['Bb2', 'F3', 'A3', 'D4'],
		low: 'Bb2',
		high: 'F3',
		octave: 'Bb3',
		root: 'Bb1',
		bell: 'F5',
		line: 'D4'
	},
	{
		pad: ['G2', 'D3', 'Bb3', 'A4'],
		low: 'G2',
		high: 'D3',
		octave: 'G3',
		root: 'G1',
		bell: 'D5',
		line: 'Bb3'
	},
	{
		pad: ['F3', 'C4', 'E4', 'A4'],
		low: 'F3',
		high: 'C4',
		octave: 'F4',
		root: 'F2',
		bell: 'C5',
		line: 'A4'
	},
	{
		pad: ['A2', 'E3', 'A3', 'D4'],
		low: 'A2',
		high: 'E3',
		octave: 'A3',
		root: 'A1',
		bell: 'D5',
		line: 'E4'
	}
] as const;
/** Which steps `n` stories walk, always ending on the A that the recap resolves to D. */
const WALK = [[0], [0, 4], [0, 2, 4], [0, 1, 2, 4], [0, 1, 2, 3, 4]];
/** The ostinato's eighths over a bar, by step: low, high, the low's octave, or a rest. */
const FIGURES = ['LHLHLHLH', 'LHOHLHOH', 'L.H.L.H.', 'HLHOHLHO', 'LHLHOHOH'];

/** `Reported by 4 outlets`; nothing for a story only its own outlet has. */
export const reach = (s: NewsStory) => (s.outlets > 1 ? `Reported by ${s.outlets} outlets` : '');

/** The outlets in the order the stories name them, each once. */
export const outletsOf = (stories: NewsStory[]) => [...new Set(stories.map((s) => s.outlet))];

// a line parked under its clip's edge
const BELOW = 'translateY(130%)';
const ABOVE = 'translateY(-130%)';

interface Fit {
	size: number;
	min: number;
	lines: number;
	leading: number;
	width: number;
}

/** The headline fitted, then rebuilt as one clipped line per fitted line, for the rise. */
function headlineLines(parent: HTMLElement, text: string, fit: Fit, cls = 'nw-headline') {
	const h = div(cls, parent, text);
	css(h, { fontSize: `${fit.size}px`, lineHeight: String(fit.leading) });
	fitText(h, { width: fit.width, min: fit.min, lines: fit.lines, leading: fit.leading });
	const lines: string[] = [''];
	for (const n of h.childNodes) {
		if (n instanceof HTMLBRElement) lines.push('');
		else lines[lines.length - 1] += n.textContent ?? '';
	}
	h.replaceChildren();
	return lines.map((l) => span(null, span('nw-line', h), l.trim()));
}

const STORY_FIT: Fit = {
	size: HEADLINE.size,
	min: HEADLINE.min,
	lines: HEADLINE.lines,
	leading: HEADLINE.leading,
	width: SAFE_RIGHT - STORY.x - STORY.indent
};

export const news: Template<NewsSpec> = {
	version: 3,
	build({ root, sound }, spec) {
		const D = spec.duration ?? 54.4;
		const tl = timeline(root, D);
		root.classList.add('bulletin', 'news');
		field(tl, root, { x: 960, y: 1260, rx: 1700, ry: 560, a: 0.7 });

		const stories = spec.stories.slice(0, 5);
		const n = stories.length;
		const run = running(stories, D);
		const h = head(root, 'Headlines', ['Czech news', spec.asOf]);
		headIn(tl, h, 0.15);
		const corner = mark(tl, root, 0.3);

		// a segment per story along the top: its track wipes in, then its red as its story comes
		const segW = (SAFE_RIGHT - 96 - SEGMENTS.gap * (n - 1)) / n;
		const segs = stories.map((_, i) => {
			const track = css(div('nw-seg', root), {
				left: `${96 + i * (segW + SEGMENTS.gap)}px`,
				width: `${segW}px`
			});
			tl.add(
				track,
				[{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }],
				0.3 + i * 0.06,
				0.6,
				'glide'
			);
			css(track, { transformOrigin: '0 50%' });
			return { track, on: div('on', track), done: div('done', track) };
		});

		const credit = credits(root, [spec.credit]);
		tl.add(credit, [{ opacity: 0 }, { opacity: 1 }], 0.6, 0.6, 'sine');

		// the counter at the header's end: the number rises out of its own clip, one per story
		const count = div('nw-count tnum', root);
		const numbers = div('nw-num', count);
		span('nw-of', count, `/ ${String(n).padStart(2, '0')}`);
		tl.add(count, [{ opacity: 0 }, { opacity: 1 }], run.open - 0.3, 0.5, 'sine');
		const nums = stories.map((_, i) => span(null, numbers, String(i + 1).padStart(2, '0')));

		// a card at the story's place: its bar and the rows that rise into it, one clipped line each
		const card = (rows: (text: HTMLElement) => HTMLElement[], a: number, b: number) => {
			const box = css(div('nw-story', root), { left: `${STORY.x}px` });
			const bar = div('nw-bar', box);
			const lines = rows(div('nw-text', box));
			box.style.top = `${Math.round(STORY.mid - box.offsetHeight / 2)}px`;
			tl.show(box, a, b + 0.001);
			const height = box.offsetHeight;
			tl.add(
				bar,
				[{ transform: 'translateY(0px) scaleY(0)' }, { transform: 'translateY(0px) scaleY(1)' }],
				a,
				0.5,
				'glide'
			);
			lines.forEach((l, j) =>
				tl.add(
					l,
					[{ transform: BELOW }, { transform: 'translateY(0)' }],
					a + 0.08 + j * 0.09,
					0.8,
					'glide'
				)
			);
			// out: the lines lift away, the bar closes downwards
			const out = b - 0.4;
			lines.forEach((l, j) =>
				tl.add(
					l,
					[{ transform: 'translateY(0)' }, { transform: ABOVE }],
					out + j * 0.03,
					0.32,
					'leave'
				)
			);
			tl.add(
				bar,
				[
					{ transform: 'translateY(0px) scaleY(1)' },
					{ transform: `translateY(${height}px) scaleY(0)` }
				],
				out + 0.08,
				0.3,
				'move'
			);
			return box;
		};

		// --- the open: what the bulletin holds and where it comes from -------------------------
		const topic = n === 1 ? 'Top story' : `Top ${n} stories`;
		card(
			(text) => {
				const title = headlineLines(text, topic, STORY_FIT);
				const from = div('nw-from', text);
				const names = span(null, span('nw-line', from), `From ${list(outletsOf(stories))}`);
				return [...title, names];
			},
			1.0,
			run.open
		);
		tl.cue('open', 0);

		// --- the stories -------------------------------------------------------------------------
		stories.forEach((s, i) => {
			const [a, b] = run.stories[i];
			let reaches: HTMLElement | null = null;
			card(
				(text) => {
					const meta = div('nw-meta', div('nw-line', text));
					const outlet = span('nw-outlet', meta, s.outlet);
					fitText(outlet, { width: 700, min: 22 });
					const r = reach(s);
					if (r) reaches = span('nw-reach', meta, r);
					return [meta, ...headlineLines(text, s.headline, STORY_FIT)];
				},
				a,
				b
			);
			tl.cue(`story ${i + 1}: ${s.outlet}`, a);
			// the reach comes after the headline has been read
			if (reaches)
				tl.add(
					reaches,
					[{ transform: BELOW }, { transform: 'translateY(0)' }],
					a + 2.4,
					0.6,
					'glide'
				);

			// the segment lights as its story comes and turns paper as it is read
			tl.add(segs[i].on, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], a, 0.55, 'move');
			tl.add(
				segs[i].done,
				[{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }],
				b - 0.3,
				0.4,
				'move'
			);
			// the number rises, and lifts when its story is read
			tl.add(
				nums[i],
				[{ transform: BELOW }, { transform: 'translateY(0)' }],
				a + 0.1,
				0.6,
				'glide'
			);
			tl.add(
				nums[i],
				[{ transform: 'translateY(0)' }, { transform: ABOVE }],
				b - 0.4,
				0.32,
				'leave'
			);
		});
		const chrome = [...segs.map((s) => s.track), count];

		// --- the recap: every headline again, then the sign-off ---------------------------------
		const hr = head(root, 'Recap', ['Czech news', spec.asOf]);
		headOut(tl, h, run.recap - 0.1);
		headIn(tl, hr, run.recap + 0.35);
		lift(tl, chrome, run.recap - 0.1);

		const tagAt = D - 5.2;
		const size = [0, 60, 52, 46, 42, 38][n];
		// the rows share one size: a long headline breaks onto two lines before it shrinks, and the
		// whole list steps down in size until it clears the sign-off
		const build = (s: number) =>
			stories.map((story, i) => {
				const row = css(div(i ? 'nw-row' : 'nw-row nw-first', root), { left: `${STORY.x}px` });
				const rule = div('nw-rule', row);
				const who = div('nw-who', row);
				const name = span(null, span('nw-line', who), story.outlet);
				fitText(name, { width: RECAP.who - 32, min: 16 });
				const what = div('nw-what', row);
				const lines = headlineLines(
					what,
					story.headline,
					{ size: s, min: s, lines: 2, leading: 1.16, width: 1728 - RECAP.who - 24 },
					'nw-recap'
				);
				// the name sits level with the headline's first line
				who.style.marginTop = `${Math.round(16 + s * 0.58 - 13)}px`;
				return { row, rule, who: name, lines };
			});
		const height = (r: { row: HTMLElement }[]) => r.reduce((a, x) => a + x.row.offsetHeight, 0);
		let rows = build(size);
		for (let s = size - 4; height(rows) > RECAP.bottom - RECAP.top && s >= 20; s -= 4) {
			for (const r of rows) r.row.remove();
			rows = build(s);
		}
		const natural = height(rows);
		const slack = Math.max(0, RECAP.bottom - RECAP.top - natural);
		const extra = Math.min(28, slack / n);
		let y = RECAP.top + (slack - extra * n) / 2;
		// the rows come in one after another, filling the time before the tag
		const spacing = Math.min(1.5, (tagAt - run.recap - 2.4) / n);
		rows.forEach(({ row, rule, who, lines }, i) => {
			const grow = row.offsetHeight + extra;
			css(row, { top: `${Math.round(y)}px`, height: `${Math.round(grow)}px` });
			y += grow;
			const at = run.recap + 0.7 + i * spacing;
			tl.add(rule, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], at, 0.6, 'glide');
			tl.add(who, [{ transform: BELOW }, { transform: 'translateY(0)' }], at + 0.1, 0.6, 'glide');
			lines.forEach((l, j) =>
				tl.add(
					l,
					[{ transform: BELOW }, { transform: 'translateY(0)' }],
					at + 0.16 + j * 0.09,
					0.8,
					'glide'
				)
			);
		});
		tl.cue('recap', run.recap);

		// the sign-off: the tag lands in the foot's corner while the recap holds
		const tag = signOff(tl, corner, tagAt, sound);
		exit(tl, [hr.root, ...rows.map((r) => r.row), credit, tag]);

		// --- the sound ---------------------------------------------------------------------------
		const end = D - 1.2;
		const walk = WALK[n - 1];
		const [first] = run.stories[0];
		// the mix and its fade, and the pads' filter over the whole bulletin
		tl.sound(
			0,
			(when, b, skip) => {
				const w = when - skip;
				bulletinHouse(sound, b, w, D);
				padSweep(house(sound, b), w, [
					[0, 600],
					[1.8, 1800],
					[first + 1, 2400],
					[run.recap - 0.3, 1500],
					[run.recap + 1.4, 2800],
					[end - 0.8, 3000],
					[end, 900]
				]);
			},
			D
		);
		// a chord is built when its turn comes, not up front: every voice built costs the render
		const chord = (from: number, notes: string[], dur: number, o: PadOptions) =>
			tl.sound(
				from,
				(when, b, skip) => pad(house(sound, b), when - skip, notes, dur, o),
				o.attack! + dur + o.release! * 2.4
			);
		// the harmony steps with the stories and settles on D major for the recap
		run.stories.forEach(([a, z], k) => {
			const from = k ? a - 0.4 : 0.1;
			chord(from, [...STEPS[walk[k]].pad], z - from - 0.8, {
				attack: k ? 0.8 : 1.4,
				release: 1.2,
				level: 0.75,
				seed: 140 + k
			});
		});
		chord(run.recap - 0.4, ['D3', 'A3', 'E4', 'F#4'], end - run.recap - 0.8, {
			attack: 1.2,
			release: 1.4,
			level: 0.8,
			seed: 150
		});
		tl.sound(run.recap - 0.2, (w, b) =>
			sub(house(sound, b), w, 'D2', end - run.recap - 0.4, { gain: 0.35, attack: 1.2, release: 1 })
		);
		// the open: the air breathes in, a low D, two notes of glass falling
		tl.sound(0, (w, b) =>
			air(house(sound, b), w, 2.6, {
				peakAt: 1.8,
				band: [400, 2000],
				pans: [-0.5, 0.5],
				gain: 0.5,
				seed: 770
			})
		);
		tl.sound(0.4, (w, b) =>
			piano(house(sound, b), w, 'D2', { vel: 0.4, dur: 2.4, verb: 0.6, gain: 0.6, seed: 771 })
		);
		[
			[1.2, 'D5', 0.4],
			[1.6, 'A4', 0.3]
		].forEach(([at, note, pan], i) =>
			tl.sound(at as number, (w, b) =>
				glass(house(sound, b), w, note as string, {
					decay: 1.8,
					gain: 0.7,
					pan: pan as number,
					verb: 0.65,
					seed: 772 + i
				})
			)
		);
		// each story: its glass note, and the ostinato in eighths in its own figure
		run.stories.forEach(([a, z], k) => {
			const step = STEPS[walk[k]];
			const figure = FIGURES[walk[k]];
			const last = k === n - 1;
			tl.sound(a, (w, b) =>
				glass(house(sound, b), w, step.bell, {
					decay: last ? 2.4 : 1.8,
					gain: 0.75,
					pan: -0.25 + (0.5 * k) / Math.max(1, n - 1),
					verb: 0.65,
					seed: 780 + k
				})
			);
			const notes = { L: step.low, H: step.high, O: step.octave };
			for (let t = a, i = 0; t < z - 0.3; t += BEAT / 2, i++) {
				const note = notes[figure[i % 8] as keyof typeof notes];
				if (note) {
					const accent = i % 8 === 0;
					tl.sound(t, (w, b) =>
						piano(house(sound, b), w, note, {
							vel: (accent ? 0.38 : 0.26) + (last ? 0.03 * Math.min(4, i / 8) : 0),
							dur: figure[1] === '.' ? 0.7 : 0.3,
							pan: i % 2 ? 0.12 : -0.12,
							verb: 0.35,
							gain: accent ? 0.62 : 0.5,
							seed: 700 + ((k * 8 + i) % 24)
						})
					);
				}
				if (i % (figure[1] === '.' ? 8 : 4) === 0)
					tl.sound(t, (w, b) =>
						pulse(house(sound, b), w, step.root, { decay: 0.55, gain: 0.6, open: 0.1, seed: 760 })
					);
			}
			// the sparse figure carries a bowed line over the chord's third
			if (figure[1] === '.')
				tl.sound(a + 1.6, (w, b) =>
					bowed(house(sound, b), w, step.line, z - a - 3.4, {
						gain: 0.5,
						attack: 0.8,
						release: 1,
						verb: 0.6,
						seed: 790 + k
					})
				);
		});
		// the recap: the air swells with the header, D major settles and its notes rise one by one
		// until the tag lands on the same chord
		tl.sound(run.recap - 0.2, (w, b) =>
			air(house(sound, b), w, 1.8, {
				peakAt: 0.9,
				band: [400, 2200],
				pans: [-0.8, 0.8],
				gain: 0.55,
				seed: 795
			})
		);
		const rising = ['D3', 'A3', 'D4', 'F#4', 'E4', 'A4', 'D4', 'A3'];
		for (let t = run.recap + 0.8, i = 0; t < tagAt - 1; t += BEAT, i++)
			tl.sound(t, (w, b) =>
				piano(house(sound, b), w, rising[i % rising.length], {
					vel: 0.34,
					dur: 1.2,
					pan: -0.1 + 0.2 * ((i % 3) / 2),
					verb: 0.5,
					gain: 0.6,
					seed: 800 + i
				})
			);
		return tl;
	}
};
