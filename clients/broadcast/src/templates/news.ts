// The news (DESIGN.md 2.11, sharp): the top Czech headlines one at a time, each whole and as the
// outlet wrote it, labelled with the outlet's name in plain text. English chrome frames Czech
// content the way a quotation is framed: a red bar beside the story, a red segment per story
// along the top lighting as its story comes; the headline is the large text, set at 80 px where
// it fits on three lines and smaller only for the longest, so a short headline still holds the
// frame. Between the changes nothing moves. The music: a muted felt-piano ostinato on D and A at
// 75 bpm over a soft pulse and the pad in D minor, a glass note at each story descending to D.

import { glass, house, pad, padSweep, piano, pulse } from '../audio/voices';
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
	mark,
	SAFE_RIGHT
} from './bulletin';
import { timeline, type Template } from './shared';

/** The story's place: its bar at x 96, the text 48 px in, centred between the segments and the foot. */
const STORY = { x: 96, indent: 48, mid: 668 };
const HEADLINE = { size: 80, min: 72, lines: 3, leading: 1.14 };
const SEGMENTS = { y: 330, gap: 16 };

/**
 * When each story is on: its reading time (1.4 s and 38 ms a character, at least 3.2 s), scaled
 * so the stories fill the bulletin from `open` to `close`, each change on the half beat.
 */
export function holds(stories: NewsStory[], open: number, close: number): [number, number][] {
	const want = stories.map((s) => Math.max(3.2, 1.4 + s.headline.length * 0.038));
	const k = (close - open) / want.reduce((a, b) => a + b, 0);
	const half = BEAT / 2;
	const edges = [open];
	let t = open;
	for (const w of want) {
		t += w * k;
		edges.push(Math.round(t / half) * half);
	}
	edges[edges.length - 1] = close;
	return want.map((_, i) => [edges[i], edges[i + 1]]);
}

/** `Reported by 4 outlets`; nothing for a story only its own outlet has. */
export const reach = (s: NewsStory) => (s.outlets > 1 ? `Reported by ${s.outlets} outlets` : '');

// a line parked under its clip's edge
const BELOW = 'translateY(130%)';
const ABOVE = 'translateY(-130%)';

/** The headline fitted, then rebuilt as one clipped line per fitted line, for the rise. */
function headlineLines(parent: HTMLElement, text: string) {
	const h = div('nw-headline', parent, text);
	css(h, { fontSize: `${HEADLINE.size}px`, lineHeight: String(HEADLINE.leading) });
	const width = SAFE_RIGHT - STORY.x - STORY.indent;
	fitText(h, { width, min: HEADLINE.min, lines: HEADLINE.lines, leading: HEADLINE.leading });
	const lines: string[] = [''];
	for (const n of h.childNodes) {
		if (n instanceof HTMLBRElement) lines.push('');
		else lines[lines.length - 1] += n.textContent ?? '';
	}
	h.replaceChildren();
	return lines.map((l) => span(null, span('nw-line', h), l.trim()));
}

export const news: Template<NewsSpec> = {
	version: 2,
	build({ root, sound }, spec) {
		const D = spec.duration ?? 20;
		const tl = timeline(root, D);
		root.classList.add('bulletin', 'news');
		field(tl, root, { x: 960, y: 1260, rx: 1700, ry: 560, a: 0.7 });

		const stories = spec.stories.slice(0, 5);
		const n = stories.length;
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

		const slots = holds(stories, 0.6, D - 0.5);
		const boxes = stories.map((s, i) => {
			const [a, b] = slots[i];
			const last = i === n - 1;
			const box = css(div('nw-story', root), { left: `${STORY.x}px` });
			const bar = div('nw-bar', box);
			const text = div('nw-text', box);
			const meta = div('nw-meta', div('nw-line', text));
			const outlet = span('nw-outlet', meta, s.outlet);
			fitText(outlet, { width: 700, min: 22 });
			const r = reach(s);
			if (r) span('nw-reach', meta, r);
			const lines = headlineLines(text, s.headline);
			box.style.top = `${Math.round(STORY.mid - box.offsetHeight / 2)}px`;
			tl.show(box, a, last ? D : b + 0.001);
			tl.cue(`story ${i + 1}: ${s.outlet}`, a);

			// in: the bar and the segment wipe open, the outlet and the lines rise into place
			const height = box.offsetHeight;
			tl.add(
				bar,
				[{ transform: 'translateY(0px) scaleY(0)' }, { transform: 'translateY(0px) scaleY(1)' }],
				a,
				0.5,
				'glide'
			);
			tl.add(segs[i].on, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], a, 0.55, 'move');
			tl.add(meta, [{ transform: BELOW }, { transform: 'translateY(0)' }], a + 0.08, 0.6, 'glide');
			lines.forEach((l, j) =>
				tl.add(
					l,
					[{ transform: BELOW }, { transform: 'translateY(0)' }],
					a + 0.16 + j * 0.09,
					0.8,
					'glide'
				)
			);
			// out, but for the last: the lines lift away, the bar closes downwards, the segment
			// turns paper as its story is read
			if (!last) {
				const out = b - 0.4;
				[meta, ...lines].forEach((l, j) =>
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
				tl.add(
					segs[i].done,
					[{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }],
					out + 0.1,
					0.4,
					'move'
				);
			}
			return box;
		});
		exit(tl, [h.root, ...segs.map((s) => s.track), boxes[n - 1], credit, corner]);

		// --- the sound -------------------------------------------------------------------------
		const end = D - 1.2;
		tl.sound(
			0,
			(when, b, skip) => {
				const w = when - skip;
				const hs = bulletinHouse(sound, b, w, D);
				padSweep(hs, w, [
					[0, 600],
					[1.6, 1800],
					[end - 1, 2400],
					[end, 900]
				]);
				pad(hs, w + 0.1, ['D3', 'A3', 'E4', 'F4'], end - 1.4, {
					attack: 1.4,
					release: 1.4,
					level: 0.75,
					seed: 140
				});
			},
			D
		);
		// the ostinato in eighths from the first story to the last bar before the end, the pulse
		// on beats one and three
		const from = slots[0][0];
		for (let t = from, i = 0; t < end - 0.3; t += BEAT / 2, i++) {
			const note = i % 2 ? 'A3' : 'D3';
			const accent = i % 8 === 0;
			tl.sound(t, (w, b) =>
				piano(house(sound, b), w, note, {
					vel: accent ? 0.38 : 0.26,
					dur: 0.3,
					pan: i % 2 ? 0.12 : -0.12,
					verb: 0.35,
					gain: accent ? 0.62 : 0.5,
					seed: 700 + (i % 16)
				})
			);
			if (i % 4 === 0)
				tl.sound(t, (w, b) =>
					pulse(house(sound, b), w, 'D2', { decay: 0.55, gain: 0.6, open: 0.1, seed: 760 })
				);
		}
		// a glass note as each story comes, stepping down to D
		const bells = ['A5', 'F5', 'E5', 'C5', 'D5'].slice(5 - n);
		bells[0] = 'A5';
		slots.forEach(([a], i) =>
			tl.sound(a, (w, b) =>
				glass(house(sound, b), w, bells[i], {
					decay: i === n - 1 ? 2.4 : 1.8,
					gain: 0.75,
					pan: -0.25 + (0.5 * i) / Math.max(1, n - 1),
					verb: 0.65,
					seed: 780 + i
				})
			)
		);
		return tl;
	}
};
