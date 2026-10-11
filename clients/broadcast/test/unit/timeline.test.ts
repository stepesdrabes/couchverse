import { describe, expect, it } from 'vitest';
import { FPS, frame, Timeline } from '../../src/core/timeline';

// a stand-in for an element: the timeline only needs `animate`
function fakeElement() {
	const anims: { currentTime: number | null }[] = [];
	const el = {
		anims,
		animate: () => {
			const a = { currentTime: 0 as number | null, startTime: null, pause() {}, cancel() {} };
			anims.push(a);
			return a;
		}
	};
	return el as unknown as Element & { anims: typeof anims };
}

describe('Timeline', () => {
	it('sees motion only where an animation is active', () => {
		const root = {} as HTMLElement;
		const tl = new Timeline(root, 10);
		tl.add(fakeElement(), [{ opacity: 0 }, { opacity: 1 }], 1, 0.5);
		tl.add(fakeElement(), [{ opacity: 0 }, { opacity: 1 }], 4, 1, 'linear', { iterations: 2 });
		const f = (n: number) => n / FPS;
		expect(tl.changes(f(0), f(1))).toBe(false);
		expect(tl.changes(f(24), f(25))).toBe(true);
		expect(tl.changes(f(38), f(39))).toBe(false);
		expect(tl.changes(f(110), f(111))).toBe(true);
		expect(tl.changes(5.96, 6.0)).toBe(true);
		expect(tl.changes(6.0, 6.04)).toBe(false);
	});

	it('treats a changing value as motion', () => {
		const tl = new Timeline({} as HTMLElement, 10);
		const texts: string[] = [];
		tl.dynamic(
			(t) => String(Math.ceil(10 - t)),
			(v) => texts.push(String(v))
		);
		expect(tl.changes(2.2, 2.6)).toBe(false);
		expect(tl.changes(2.92, 2.96)).toBe(false);
		// the counter turns over at 3 s, so the frame at 3.0 already differs
		expect(tl.changes(2.96, 3.0)).toBe(true);
		tl.update(0.5);
		tl.update(0.6);
		tl.update(1.5);
		expect(texts).toEqual(['10', '9']);
	});

	it('seeks every animation to the same time', async () => {
		const el = fakeElement();
		const tl = new Timeline({} as HTMLElement, 10);
		tl.add(el, [{ opacity: 0 }, { opacity: 1 }], 1, 1);
		tl.add(el, [{ opacity: 1 }, { opacity: 0 }], 5, 1);
		await tl.seek(3.2);
		expect(el.anims.map((a) => a.currentTime)).toEqual([3200, 3200]);
	});

	it('finds the cue at a time', () => {
		const tl = new Timeline({} as HTMLElement, 10);
		tl.cue('a', 0);
		tl.cue('b', 2.5);
		expect(tl.cueAt(1)?.name).toBe('a');
		expect(tl.cueAt(2.5)?.name).toBe('b');
	});

	it('schedules sounds from an offset, reseeding per cue', () => {
		const tl = new Timeline({} as HTMLElement, 10);
		const heard: [number, number][] = [];
		const seeds: number[] = [];
		tl.sound(1, (w, _b, skip) => heard.push([w, skip]));
		tl.sound(2, (w, _b, skip) => heard.push([w, skip]), 6);
		tl.sound(5, (w, _b, skip) => heard.push([w, skip]));
		tl.scheduleSounds(100, 3, {} as never, (i) => seeds.push(i));
		// the cue at 1 s is gone, the bed from 2 s joins 1 s in, the cue at 5 s is 2 s away
		expect(heard).toEqual([
			[100, 1],
			[102, 0]
		]);
		expect(seeds).toEqual([1, 2]);
	});

	it('rounds to the frame grid', () => {
		expect(frame(11.5)).toBe(11.52);
		expect(frame(13.487)).toBe(13.48);
	});
});
