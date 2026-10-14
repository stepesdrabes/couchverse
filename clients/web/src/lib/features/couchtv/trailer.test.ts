import { describe, expect, it } from 'vitest';
import type { CouchTvTrailerCut } from '$lib/generated/api';
import {
	drop,
	dropAll,
	footageSeconds,
	handBack,
	keep,
	keptScenes,
	sceneKey,
	stillSeconds,
	trailerPick,
	untouched,
	verdict
} from './trailer';

const cut = (mediaFileId: string, startMs: number, durationMs = 5000): CouchTvTrailerCut => ({
	mediaFileId,
	startMs,
	durationMs,
	lines: [],
	source: 'subtitles'
});
const scenes = [cut('a', 60_000), cut('a', 300_000), cut('b', 60_000), cut('b', 900_000)];
const keys = scenes.map(sceneKey);

describe('the trailer edit', () => {
	it('names a scene by its file and start, so two files may share a start', () => {
		expect(keys).toEqual(['a:60000', 'a:300000', 'b:60000', 'b:900000']);
	});

	it('sends nothing for the analysis own pick until a scene is dropped', () => {
		expect(trailerPick(scenes, untouched(false))).toBeNull();
		const edit = drop(untouched(false), keys[1]);
		expect(trailerPick(scenes, edit)).toEqual({
			keep: [
				{ mediaFileId: 'a', startMs: 60_000 },
				{ mediaFileId: 'b', startMs: 60_000 },
				{ mediaFileId: 'b', startMs: 900_000 }
			]
		});
		expect(keptScenes(scenes, edit).map(sceneKey)).toEqual([keys[0], keys[2], keys[3]]);
	});

	it('keeps the scenes in the order the trailer has them', () => {
		const edit = drop(drop(untouched(false), keys[3]), keys[0]);
		expect(keptScenes(scenes, edit).map(sceneKey)).toEqual([keys[1], keys[2]]);
	});

	it('is not an edit when every dropped scene is kept again on the analysis own pick', () => {
		let edit = drop(drop(untouched(false), keys[0]), keys[2]);
		edit = keep(keep(edit, keys[0], false), keys[2], false);
		expect(edit).toEqual(untouched(false));
		expect(trailerPick(scenes, edit)).toBeNull();
	});

	it('stays the admin pick when its scenes are kept again', () => {
		const edit = keep(drop(untouched(true), keys[0]), keys[0], true);
		expect(edit.picked).toBe(true);
		expect(trailerPick(scenes, edit)?.keep).toHaveLength(4);
	});

	it('drops a scene once', () => {
		const edit = drop(drop(untouched(false), keys[0]), keys[0]);
		expect(edit.dropped).toEqual([keys[0]]);
	});

	it('picks no trailer by dropping every scene, even when there are none', () => {
		expect(trailerPick(scenes, dropAll(scenes))).toEqual({ keep: [] });
		expect(trailerPick([], dropAll([]))).toEqual({ keep: [] });
	});

	it('hands the trailer back', () => {
		expect(trailerPick(scenes, handBack())).toBeNull();
		expect(handBack()).toEqual(untouched(false));
	});

	it('does not mutate the edit it was given', () => {
		const before = untouched(true);
		drop(before, keys[0]);
		expect(before.dropped).toEqual([]);
	});
});

describe('the trailer verdict', () => {
	it('airs from the fewest scenes the server names', () => {
		expect(verdict(0, 3)).toBe('none');
		expect(verdict(1, 3)).toBe('too_few');
		expect(verdict(2, 3)).toBe('too_few');
		expect(verdict(3, 3)).toBe('airs');
		expect(verdict(9, 3)).toBe('airs');
	});
});

describe('the stills of a scene', () => {
	it('are taken just inside its start and at its middle, to the second', () => {
		expect(stillSeconds({ startMs: 12_000, durationMs: 5000 })).toEqual({ first: 12, middle: 15 });
		// a start between seconds shows the next one, never the shot before the scene
		expect(stillSeconds({ startMs: 12_300, durationMs: 5000 })).toEqual({ first: 13, middle: 15 });
		expect(stillSeconds({ startMs: 12_900, durationMs: 3000 })).toEqual({ first: 13, middle: 14 });
	});

	it('are two different seconds even for the shortest scene', () => {
		for (let start = 0; start < 5000; start += 40) {
			const { first, middle } = stillSeconds({ startMs: start, durationMs: 3000 });
			expect(middle).toBeGreaterThan(first);
			expect(middle * 1000).toBeLessThan(start + 3000);
		}
	});
});

describe('the footage', () => {
	it('adds up to whole seconds', () => {
		expect(footageSeconds(scenes)).toBe(20);
		expect(footageSeconds([])).toBe(0);
	});
});
