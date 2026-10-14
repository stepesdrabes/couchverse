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
import { bedKeys } from '../../src/templates/trailer-score';
import {
	BODY,
	cutWindow,
	linesOf,
	montageCuts,
	planTrailer,
	scenesOf,
	type Mood,
	type PlanInput,
	type SceneFacts
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

const plan = (over: Partial<PlanInput> = {}) =>
	planTrailer({
		mood: 'cinematic',
		premiere: false,
		scenes: SCENES,
		cards: [17, 16],
		bed: null,
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

	it('gives the montage the time when the scenes fall short', () => {
		const few = plan({ scenes: SCENES.slice(0, 3) });
		const montage = few.sections.find((s) => s.kind === 'montage');
		expect(montage && montage.end - montage.at).toBeCloseTo(16, 6);
		expect(few.end).toBeLessThan(plan().end);
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
		expect(keys.length).toBe(8);
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
