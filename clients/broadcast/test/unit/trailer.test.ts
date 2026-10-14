import { describe, expect, it } from 'vitest';
import {
	BED_LUFS,
	DIALOGUE_LUFS,
	dbAt,
	duckKeys,
	gainOf,
	levelling,
	loudnessOf,
	ride,
	SPEECH
} from '../../src/audio/mix';
import { bedKeys, STEADY } from '../../src/templates/trailer-score';
import {
	assignTakes,
	BODY,
	cutWindow,
	linesOf,
	montageCuts,
	planTrailer,
	reelsOf,
	scenesOf,
	type Mood,
	type PlanInput,
	type SceneFacts,
	type Take
} from '../../src/templates/trailer-plan';

const onFrame = (t: number) => Math.abs(t * 25 - Math.round(t * 25)) < 1e-6;

/** A scene of `len` seconds with lines of `speech` seconds at `starts`. */
const scene = (len: number, ...lines: [number, number][]): SceneFacts => ({ len, lines });

// eight scenes of about 45 s: a quiet opening, talking scenes, one without a line, a short last one
const SCENES: SceneFacts[] = [
	scene(4.8, [0.3, 4.4]),
	scene(5.3, [0.3, 3.0], [3.2, 4.7]),
	scene(6.4, [0.2, 2.0], [3.4, 6.0]),
	scene(7),
	scene(6.2, [0.4, 1.8], [2.3, 4.0], [4.4, 5.5]),
	scene(5.5, [0.3, 2.2], [2.9, 5.0]),
	scene(5.8, [0.5, 5.2]),
	scene(3.4, [0.3, 1.3])
];

// eight footage shots of 2.4 s, as a movie has them (a series two, whatever its length)
const SHOTS = Array<number>(8).fill(2.4);

const plan = (over: Partial<PlanInput> = {}) =>
	planTrailer({
		mood: 'cinematic',
		premiere: false,
		scenes: SCENES,
		cards: [17, 16],
		bed: null,
		shots: SHOTS,
		...over
	});

const MOODS: Mood[] = ['cinematic', 'grit', 'bright'];

describe('the trailer plan', () => {
	it('runs 60 to 90 s in whole frames, one section after the other', () => {
		for (const mood of MOODS) {
			for (const premiere of [false, true]) {
				const p = plan({ mood, premiere });
				expect(p.end, `${mood} ${premiere}`).toBeGreaterThanOrEqual(60);
				expect(p.end).toBeLessThanOrEqual(BODY[1]);
				expect(onFrame(p.end)).toBe(true);
				let at = 0;
				for (const s of p.sections) {
					expect(s.at).toBeCloseTo(at, 6);
					expect(onFrame(s.at) && onFrame(s.end)).toBe(true);
					expect(s.end).toBeGreaterThan(s.at);
					at = s.end;
				}
				expect(at).toBeCloseTo(p.end, 6);
			}
		}
	});

	it('is cut the way trailers are: the cold open, the channel, scenes and cards, the montage, the title, the button', () => {
		const kinds = plan().sections.map((s) => s.kind);
		expect(kinds[0]).toBe('scene');
		expect(kinds[1]).toBe('presents');
		const m = kinds.indexOf('montage');
		expect(kinds.slice(m)).toEqual(['montage', 'title', 'scene']);
		expect(kinds.slice(2, m)).toContain('card');
		expect(scenesOf(plan()).map((s) => s.role)).toEqual([
			'open',
			...Array(6).fill('act'),
			'button'
		]);
		// the card before the montage holds the last tagline; a premiere card follows it
		expect(kinds[m - 1]).toBe('card');
		expect(plan({ premiere: true }).sections.map((s) => s.kind)).toContain('premiere');
	});

	it('never cuts into a line, and keeps the fades clear of every word', () => {
		for (const mood of MOODS) {
			const p = plan({ mood });
			for (const s of scenesOf(p)) {
				const src = SCENES[s.scene];
				// the sound's stretch holds every line it touches whole
				for (const [a, b] of src.lines) {
					const inside = a >= s.soundFrom - 1e-6 && b <= s.soundTo + 1e-6;
					const outside = b <= s.soundFrom + 1e-6 || a >= s.soundTo - 1e-6;
					expect(inside || outside, `${mood} scene ${s.scene} line ${a}-${b}`).toBe(true);
				}
				// the picture lies inside the sound, by at most four frames at either end
				expect(s.from).toBeGreaterThanOrEqual(s.soundFrom - 1e-6);
				expect(s.to).toBeLessThanOrEqual(s.soundTo + 1e-6);
				expect(s.from - s.soundFrom).toBeLessThanOrEqual(4 / 25 + 1e-6);
				expect(s.soundTo - s.to).toBeLessThanOrEqual(4 / 25 + 1e-6);
				// no word sits in the overlap the other scene's sound shares
				for (const [a, b] of src.lines.filter(
					(l) => l[0] >= s.soundFrom - 1e-6 && l[1] <= s.soundTo + 1e-6
				)) {
					if (s.from > s.soundFrom) expect(a).toBeGreaterThan(s.from);
					if (s.to < s.soundTo) expect(b).toBeLessThan(s.to);
				}
				// the lines on the trailer's clock sit inside the scene's own picture and sound
				for (const [a, b] of s.lines) {
					expect(a).toBeGreaterThanOrEqual(s.at - 4 / 25 - 1e-6);
					expect(b).toBeLessThanOrEqual(s.end + 4 / 25 + 1e-6);
				}
			}
			expect(linesOf(p).length).toBeGreaterThan(0);
		}
	});

	it('keeps the button on screen after its last word, for the end card to close over', () => {
		for (const mood of MOODS) {
			const button = scenesOf(plan({ mood })).find((s) => s.role === 'button');
			expect(button, mood).toBeDefined();
			const last = button?.lines.at(-1)?.[1] ?? Infinity;
			expect((button?.end ?? 0) - last, mood).toBeGreaterThanOrEqual(0.7 - 1e-6);
		}
	});

	it('leaves out the scenes with the least speech when there are too many, keeping story order', () => {
		const many: SceneFacts[] = Array.from({ length: 14 }, (_, i) =>
			// every third scene has no line at all
			i % 3 === 2 ? scene(8) : scene(8, [0.4, 2 + (i % 4)], [5, 7])
		);
		const p = plan({ scenes: many });
		expect(p.end).toBeLessThanOrEqual(BODY[1]);
		const kept = scenesOf(p).map((s) => s.scene);
		expect(kept).toEqual([...kept].sort((a, b) => a - b));
		expect(kept[0]).toBe(0);
		expect(kept.at(-1)).toBe(13);
		expect(kept.length).toBeLessThan(14);
		// the ones with no line to speak of went first
		const dropped = many.map((_, i) => i).filter((i) => !kept.includes(i));
		expect(dropped.some((i) => many[i].lines.length === 0)).toBe(true);
	});

	it('gives the montage the time when the scenes fall short, as far as there is picture', () => {
		const few = plan({ scenes: SCENES.slice(0, 3) });
		const montage = few.sections.find((s) => s.kind === 'montage');
		// eight shots of 2.4 s less the two the title takes: 14.4 s of picture, the montage's most is 16
		expect(montage && montage.end - montage.at).toBeGreaterThan(plan().sections.length ? 8 : 0);
		expect(montage && montage.end - montage.at).toBeLessThanOrEqual(14.4 + 1e-6);
		expect(few.end).toBeLessThan(plan().end);
	});

	it('cuts the montage short, or leaves it out, rather than show a picture twice', () => {
		for (const shots of [[], [2.4], [2.4, 3], [2.4, 3, 2], Array(6).fill(2)]) {
			for (const mood of MOODS) {
				const p = plan({ mood, shots, scenes: SCENES.slice(0, 4) });
				const m = p.sections.find((s) => s.kind === 'montage');
				const t = p.sections.find((s) => s.kind === 'title');
				if (t?.kind !== 'title') throw new Error('no title');
				// the picture the montage and the title use is never the same twice (but for the
				// title replaying a scene when there is nothing unseen at all)
				const seen = new Map<string, [number, number][]>();
				for (const k of [...(m?.kind === 'montage' ? m.takes : []), ...t.takes]) {
					const key = `${k.src.kind}${k.src.index}`;
					const span: [number, number] = [k.from, k.from + (k.end - k.at)];
					for (const [a, b] of seen.get(key) ?? [])
						if (k.src.kind === 'shot' || m?.kind === 'montage')
							expect(span[0] >= b - 1e-6 || span[1] <= a + 1e-6, `${mood} ${shots}`).toBe(true);
					seen.set(key, [...(seen.get(key) ?? []), span]);
				}
				// a montage is a montage: at least two seconds of cuts, none without picture
				if (m?.kind === 'montage') expect(m.end - m.at).toBeGreaterThanOrEqual(2);
				// the title starts where the montage ends, and nothing is longer than 90 s
				expect(p.hit).toBeCloseTo(t.at, 6);
				expect(p.end).toBeLessThanOrEqual(BODY[1]);
			}
		}
		// the shorter the pool the shorter the montage, and none without a pool
		const lengths = [[], [2.4, 3], Array(5).fill(2.4), SHOTS].map((shots) => {
			const m = plan({ shots, scenes: SCENES.slice(0, 4) }).sections.find(
				(s) => s.kind === 'montage'
			);
			return m ? m.end - m.at : 0;
		});
		expect(lengths[0]).toBe(0);
		expect(lengths).toEqual([...lengths].sort((a, b) => a - b));
	});

	it('lets the other parts breathe when there is no montage, never padding with picture', () => {
		const thin = plan({ shots: [], scenes: SCENES.slice(0, 4) });
		expect(thin.sections.some((s) => s.kind === 'montage')).toBe(false);
		// the cards are read in 2.4 s (cinematic) and held up to a second more
		const held = thin.sections.filter((s) => s.kind === 'card').map((c) => c.end - c.at);
		expect(held.length).toBe(2);
		for (const h of held) {
			expect(h).toBeGreaterThan(2.4 + 0.1);
			expect(h).toBeLessThanOrEqual(2.4 + 1 + 1e-6);
		}
		// the channel's moment and the title are held longer than their own time too
		const presents = thin.sections.find((s) => s.kind === 'presents');
		expect((presents?.end ?? 0) - (presents?.at ?? 0)).toBeGreaterThan(2.8);
	});

	it('trims a long scene to whole lines in the gaps between them', () => {
		const long = scene(12, [0.5, 2], [2.4, 4], [5, 9.5], [10.2, 11.5]);
		const [from, to] = cutWindow(long, 8);
		expect(to - from).toBeLessThanOrEqual(8 + 1e-6);
		for (const [a, b] of long.lines)
			expect(b <= from + 1e-6 || a >= to - 1e-6 || (a >= from && b <= to)).toBe(true);
		// one line longer than the most plays whole
		expect(cutWindow(scene(12, [1, 10]), 6)).toEqual([0.68, 10.4]);
		expect(cutWindow(scene(5), 8)).toEqual([0, 5]);
	});

	it('keeps the opener on its first line and the button on its last, whatever speaks more', () => {
		// four lines in twelve seconds, the most speech in the middle two (a free cut drops both ends)
		const long = scene(12, [0.5, 1], [3, 5], [5.6, 7.6], [10.4, 11.4]);
		const free = cutWindow(long, 5.6);
		expect(free[0]).toBeGreaterThan(1);
		expect(free[1]).toBeLessThan(10.4);
		// the opener starts on the first line (a cut a little before it), the button ends on the last
		const first = cutWindow(long, 5.6, 'first');
		expect(first[0]).toBeLessThanOrEqual(0.5);
		expect(first[1] - first[0]).toBeLessThanOrEqual(5.6 + 1e-6);
		const last = cutWindow(long, 5.6, 'last');
		expect(last[1]).toBeGreaterThanOrEqual(11.4);
		expect(last[1] - last[0]).toBeLessThanOrEqual(5.6 + 1e-6);
		// a last line longer than the most plays whole
		const [from, to] = cutWindow(scene(12, [0.5, 2], [3, 10.5]), 5.6, 'last');
		expect(from).toBeLessThanOrEqual(3);
		expect(to).toBeGreaterThanOrEqual(10.5);
		// and the plan uses it: the button's picture ends on its scene's last line
		const p = plan({
			scenes: [...SCENES.slice(0, 7), scene(11, [0.4, 1.4], [2.2, 4.6], [5.2, 7.6], [9.6, 10.6])]
		});
		const button = scenesOf(p).find((s) => s.role === 'button');
		if (button) expect(button.soundTo).toBeGreaterThanOrEqual(10.6);
	});

	it('puts the montage on the soundtrack and the title on its peak', () => {
		// beats every 0.5 s of the excerpt, a strong hit at 10.4 s
		const onsets = Array.from({ length: 80 }, (_, i) => ({ t: i * 0.5, s: i % 4 ? 0.6 : 1 }));
		const p = plan({ bed: { onsets } });
		expect(p.bedAt).not.toBeNull();
		const m = p.sections.find((s) => s.kind === 'montage');
		const title = p.sections.find((s) => s.kind === 'title');
		if (m?.kind !== 'montage' || !title) throw new Error('no montage');
		const bedAt = p.bedAt ?? 0;
		expect(m.at - bedAt).toBeCloseTo(2.4, 6);
		// the hit falls on a strong onset of the excerpt
		const onset = (title.at - bedAt) / 0.5;
		expect(Math.abs(onset - Math.round(onset)) < 0.03 && Math.round(onset) % 4 === 0).toBe(true);
		for (const c of m.cuts) expect(onFrame(c)).toBe(true);
		expect(m.cuts[0]).toBeCloseTo(m.at, 6);
		expect(m.cuts.at(-1) ?? 0).toBeLessThanOrEqual(m.end - 0.4 + 1e-6);
	});

	it('quickens the cuts as the montage goes, on a steady grid without a soundtrack', () => {
		const cuts = montageCuts(10, 18, [], { gap: [1, 0.5], cutMin: 0.5 });
		const gaps = cuts.slice(1).map((c, i) => c - cuts[i]);
		expect(gaps.length).toBeGreaterThan(6);
		expect(gaps[0]).toBeGreaterThan(gaps.at(-1) ?? 0);
		expect(gaps.every((g) => g >= 0.5 - 1e-6)).toBe(true);
	});
});

describe('the picture of the montage and the title', () => {
	const at = (src: Take['src']) => `${src.kind}${src.index}`;

	it('adds the stretches beside what plays, and the scenes left out, to the shots', () => {
		const scenes: SceneFacts[] = [scene(8), scene(5), scene(2.5), scene(0.5)];
		const reels = reelsOf([2.4, 3], scenes, [
			{ scene: 0, from: 1, to: 6 },
			{ scene: 2, from: 0, to: 2.5 }
		]);
		const of = (kind: string, index: number) =>
			reels.filter((r) => r.src.kind === kind && r.src.index === index);
		expect(of('shot', 0)).toEqual([{ src: { kind: 'shot', index: 0 }, from: 0, to: 2.4 }]);
		// scene 0 plays 1-6 s: 0.8 s before it and 1.8 s after it are free, a margin kept
		expect(of('scene', 0).map((r) => [r.from, r.to])).toEqual([
			[0, 0.8],
			[6.2, 8]
		]);
		// scene 1 was left out: all of it; scene 2 plays whole: nothing; scene 3 is too short to cut
		expect(of('scene', 1).map((r) => [r.from, r.to])).toEqual([[0, 5]]);
		expect(of('scene', 2)).toEqual([]);
		expect(of('scene', 3)).toEqual([]);
	});

	it('plays every stretch forwards from where it was left, never back and never twice', () => {
		const reels = reelsOf(Array(6).fill(2.4), [], []);
		const cuts = montageCuts(10, 20, [], { gap: [1, 0.5], cutMin: 0.5 });
		const { takes, end } = assignTakes(cuts, 20, reels);
		expect(end).toBe(20);
		expect(takes.length).toBe(cuts.length);
		const cursor = new Map<string, number>();
		for (const k of takes) {
			const from = cursor.get(at(k.src)) ?? 0;
			expect(k.from).toBeCloseTo(from, 6);
			cursor.set(at(k.src), from + (k.end - k.at));
			expect(cursor.get(at(k.src)) ?? 0).toBeLessThanOrEqual(2.4 + 1e-6);
		}
		// takes follow one another without a gap
		takes.forEach((k, i) => i && expect(k.at).toBeCloseTo(takes[i - 1].end, 6));
	});

	it('never lets two shots take turns, and goes on in one longer take instead', () => {
		const reels = reelsOf([4, 4], [], []);
		const cuts = montageCuts(10, 18, [], { gap: [1, 0.5], cutMin: 0.5 });
		const { takes, end } = assignTakes(cuts, 18, reels);
		const order = takes.map((k) => k.src.index);
		// a b then the second shot goes on (no a b a)
		for (let i = 2; i < order.length; i++) expect(order[i] === order[i - 2]).toBe(false);
		// and the picture runs out (8 s of it): the montage ends where the next cut would have been
		expect(end).toBeLessThanOrEqual(18);
		const shown = takes.reduce((n, k) => n + (k.end - k.at), 0);
		expect(shown).toBeLessThanOrEqual(8 + 1e-6);
	});
});

describe('the trailer mix: how loud the music sits', () => {
	it('takes a quieter line deeper, ramping between the depths, and never lets the music up between close ones', () => {
		const keys = duckKeys([
			[10, 12, 0],
			[12.5, 14, 6]
		]);
		const at = (t: number) => dbAt(keys, t);
		expect(at(11)).toBeCloseTo(SPEECH.depth, 6);
		expect(at(13.5)).toBeCloseTo(SPEECH.depth - 6, 6);
		// between the two it is between the two depths, never back at 0
		for (const t of [12.2, 12.3, 12.4]) {
			expect(at(t)).toBeLessThan(SPEECH.depth + 0.01);
			expect(at(t)).toBeGreaterThan(SPEECH.depth - 6 - 0.01);
		}
		// alone, a quieter line has its own duck
		expect(dbAt(duckKeys([[10, 12, 6]]), 11)).toBeCloseTo(SPEECH.depth - 6, 6);
	});
});

describe('the soundtrack of a trailer enters with the story when it can', () => {
	const onsets = Array.from({ length: 200 }, (_, i) => ({ t: i * 0.5, s: i % 4 ? 0.6 : 1 }));
	const at = (length?: number) => plan({ bed: { onsets, length } }).bedAt;
	const presents = plan().sections.find((s) => s.kind === 'presents')?.at ?? NaN;

	it("comes in with the channel's moment when it lasts to the end, else as late as it must", () => {
		const riser = plan({ bed: { onsets } }).riseAt;
		// no length known: with the riser, as before
		expect(at()).toBeCloseTo(riser, 6);
		// long enough to last the whole trailer: with the channel's moment
		expect(at(120)).toBeCloseTo(presents, 6);
		// a short one: at the riser, never later
		expect(at(10)).toBeCloseTo(riser, 6);
		// in between it starts so that it ends with the trailer, between the two
		const mid = at(45) ?? NaN;
		expect(mid).toBeGreaterThan(presents);
		expect(mid).toBeLessThan(riser);
		const p = plan({ bed: { onsets, length: 45 } });
		expect(mid + 45).toBeGreaterThanOrEqual(p.end + 3.5);
	});

	it('sits 12 dB under the dialogue from its entry to the riser, and comes up between the lines', () => {
		const p = plan({ bed: { onsets, length: 120 } });
		const keys = bedKeys(p, 'cinematic', p.end + 1.3, (p.bedAt ?? 0) + 120);
		const lines = linesOf(p);
		const curve = ride(keys, lines, p.bedAt ?? 0, p.riseAt);
		const db = (t: number) =>
			20 * Math.log10(curve.find(([x]) => Math.abs(x - t) < 1e-3)?.[1] ?? 0);
		// under a line: 12 dB under the dialogue (the soundtrack is levelled to BED_LUFS)
		const [a, b] = lines.find(([x, y]) => x > (p.bedAt ?? 0) + 4 && y < p.riseAt - 1) ?? [NaN, NaN];
		const mid = Math.round(((a + b) / 2) * 50) / 50;
		expect(BED_LUFS + db(mid) - DIALOGUE_LUFS).toBeLessThan(-11);
		expect(BED_LUFS + db(mid) - DIALOGUE_LUFS).toBeGreaterThan(-13.5);
		// and between the lines, never gone: at its steady level
		const quiet = curve.filter(
			([t]) => t > (p.bedAt ?? 0) + 3 && !lines.some(([x, y]) => t > x - 1 && t < y + 1)
		);
		expect(quiet.length).toBeGreaterThan(0);
		for (const [t, g] of quiet) expect(20 * Math.log10(g), `at ${t}`).toBeCloseTo(STEADY, 1);
	});
});

describe('the trailer mix', () => {
	it('ducks the music 13 dB under every line and closes up close ones', () => {
		const lines: [number, number][] = [
			[10, 12],
			[12.5, 14],
			[30, 31]
		];
		const base: [number, number][] = [[0, 0]];
		const points = ride(base, lines, 0, 40);
		const at = (t: number) => points.find(([x]) => Math.abs(x - t) < 1e-3)?.[1] ?? NaN;
		for (const t of [10.2, 11, 12, 12.2, 13, 13.9, 30.1, 30.9])
			expect(20 * Math.log10(at(t)), `at ${t}`).toBeLessThanOrEqual(SPEECH.depth + 0.05);
		// back at full level well before and after
		expect(at(5)).toBe(1);
		expect(at(20)).toBe(1);
		expect(at(39)).toBe(1);
		// a gap of half a second between two lines never lets the music back up
		expect(
			Math.max(...points.filter(([t]) => t > 10.5 && t < 13.5).map(([, g]) => g))
		).toBeLessThan(gainOf(SPEECH.depth) * 1.01);
		// ramps take the time they say
		const keys = duckKeys(lines);
		// (a held stretch keeps a key at each line's edges, so a quieter line can sit deeper)
		expect(keys.length).toBe(10);
		expect(keys.map(([t]) => t)).toEqual([...keys.map(([t]) => t)].sort((a, b) => a - b));
	});

	it('rides a base curve in dB and goes silent under -80', () => {
		const base: [number, number][] = [
			[0, -80],
			[10, -6],
			[20, 0]
		];
		expect(dbAt(base, 5)).toBeCloseTo(-43, 6);
		expect(dbAt(base, 25)).toBe(0);
		const pts = ride(base, [], 0, 20);
		expect(pts[0][1]).toBe(0);
		expect(pts.at(-1)?.[1]).toBeCloseTo(1, 6);
		for (let i = 1; i < pts.length; i++) expect(pts[i][0]).toBeGreaterThan(pts[i - 1][0]);
	});

	it('levels toward a target within reach, and leaves silence alone', () => {
		expect(levelling(-30, -22)).toBe(8);
		expect(levelling(-50, -22)).toBe(14);
		expect(levelling(-5, -22)).toBe(-14);
		expect(levelling(-Infinity, -22)).toBe(0);
		expect(gainOf(-6)).toBeCloseTo(0.501, 3);
		expect(gainOf(-80)).toBe(0);
	});

	it('measures the ranges of a buffer it is given, as one stretch', () => {
		const rate = 48000;
		const data = new Float32Array(rate * 6);
		// a loud second at 1 s and a quiet one at 4 s, silence between
		for (let i = 0; i < rate; i++) data[rate + i] = 0.1 * Math.sin((2 * Math.PI * 997 * i) / rate);
		for (let i = 0; i < rate; i++)
			data[4 * rate + i] = 0.1 * Math.sin((2 * Math.PI * 997 * i) / rate);
		const buffer = {
			sampleRate: rate,
			numberOfChannels: 1,
			getChannelData: () => data
		} as unknown as AudioBuffer;
		// one mono channel read as both: -20 LUFS for a 0.1 sine in two channels (see audio.test.ts)
		expect(loudnessOf(buffer, [[1, 2]]).lufs).toBeCloseTo(-20, 0);
		expect(
			loudnessOf(buffer, [
				[1, 2],
				[4, 5]
			]).lufs
		).toBeCloseTo(-20, 0);
		expect(loudnessOf(buffer, [[2.2, 3.8]]).lufs).toBe(-Infinity);
	});
});

describe('the soundtrack of a trailer', () => {
	// beats every half second from the excerpt's start, a strong one every two seconds
	const onsets = Array.from({ length: 80 }, (_, i) => ({ t: i * 0.5, s: i % 4 ? 0.6 : 1 }));

	it('stays at least 10 dB under the dialogue through every line, and rises for the montage and the title', () => {
		for (const mood of MOODS) {
			const p = plan({ mood, bed: { onsets } });
			const at = p.bedAt ?? 0;
			const stop = at + 30;
			const tagAt = p.end + 1.3;
			const curve = ride(bedKeys(p, mood, tagAt, stop), linesOf(p), at, stop);
			// the soundtrack's reference sits BED_LUFS; a line sits DIALOGUE_LUFS
			const room = DIALOGUE_LUFS - 10 - BED_LUFS;
			let heard = 0;
			for (const [from, to] of linesOf(p).filter(([a, b]) => a >= at && b <= stop)) {
				heard++;
				for (const [t, g] of curve.filter(([t]) => t >= from && t <= to))
					expect(20 * Math.log10(g), `${mood} at ${t}`).toBeLessThanOrEqual(room + 1e-6);
			}
			expect(heard, mood).toBeGreaterThan(0);
			const at0 = (t: number) => dbAt(bedKeys(p, mood, tagAt, stop), t);
			const montage = p.sections.find((s) => s.kind === 'montage');
			const title = p.sections.find((s) => s.kind === 'title');
			if (!montage || !title) throw new Error('no montage');
			// quiet under the first scenes, climbing through the montage, loudest on the hit
			expect(at0(at)).toBeLessThan(-30);
			expect(at0(montage.at)).toBeLessThan(at0(montage.end - 0.3));
			expect(at0(title.at + 0.05)).toBeGreaterThan(at0(montage.end - 0.3));
			// silent at its end, and in grit before the tag
			expect(curve.at(-1)?.[1]).toBe(0);
		}
	});
});
