// The admin's working copy of a title's trailer (the Trailer section of the title's Couch TV
// panel): which of the picker's scenes are dropped before Save, and the pick Save sends. The
// server keeps only the scenes an admin keeps, so a dropped scene is back only by handing the
// trailer back to the analysis.
import type {
	CouchTvTrailerCut,
	CouchTvTrailerPick,
	CouchTvTrailerSceneRef
} from '$lib/generated/api';

/** A scene by what it is cut from, which holds while the picker has not chosen again. */
export const sceneKey = (s: CouchTvTrailerSceneRef) => `${s.mediaFileId}:${s.startMs}`;

export interface TrailerEdit {
	/** The trailer is the admin's (Save sends the scenes kept); false: the analysis' (Save hands it back). */
	picked: boolean;
	/** The scenes dropped, by `sceneKey`. */
	dropped: readonly string[];
}

/** Nothing edited: the trailer as the server has it. */
export const untouched = (savedByAdmin: boolean): TrailerEdit => ({
	picked: savedByAdmin,
	dropped: []
});

export function drop(edit: TrailerEdit, key: string): TrailerEdit {
	return {
		picked: true,
		dropped: edit.dropped.includes(key) ? edit.dropped : [...edit.dropped, key]
	};
}

export function keep(edit: TrailerEdit, key: string, savedByAdmin: boolean): TrailerEdit {
	const dropped = edit.dropped.filter((k) => k !== key);
	// every scene back on the analysis' own pick is not an edit at all
	return { picked: savedByAdmin || dropped.length > 0, dropped };
}

/** No trailer for the title: the admin's pick of no scene at all. */
export const dropAll = (scenes: readonly CouchTvTrailerCut[]): TrailerEdit => ({
	picked: true,
	dropped: scenes.map(sceneKey)
});

/** The analysis picks the scenes again. */
export const handBack = (): TrailerEdit => ({ picked: false, dropped: [] });

export const keptScenes = (scenes: readonly CouchTvTrailerCut[], edit: TrailerEdit) =>
	scenes.filter((s) => !edit.dropped.includes(sceneKey(s)));

/** What Save sends: the scenes kept, or null to hand the trailer back. */
export function trailerPick(
	scenes: readonly CouchTvTrailerCut[],
	edit: TrailerEdit
): CouchTvTrailerPick | null {
	if (!edit.picked) return null;
	return {
		keep: keptScenes(scenes, edit).map(({ mediaFileId, startMs }) => ({ mediaFileId, startMs }))
	};
}

/** Whether scenes make a trailer the channel airs: none left, too few, or enough. */
export function verdict(kept: number, minScenes: number): 'none' | 'too_few' | 'airs' {
	if (kept === 0) return 'none';
	return kept < minScenes ? 'too_few' : 'airs';
}

/** The seconds of a scene to show a still of: just inside its start, and its middle. */
export function stillSeconds(scene: Pick<CouchTvTrailerCut, 'startMs' | 'durationMs'>) {
	return {
		first: Math.ceil(scene.startMs / 1000),
		middle: Math.round((scene.startMs + scene.durationMs / 2) / 1000)
	};
}

/** The footage the scenes add up to, in seconds. */
export const footageSeconds = (scenes: readonly Pick<CouchTvTrailerCut, 'durationMs'>[]) =>
	Math.round(scenes.reduce((sum, s) => sum + s.durationMs, 0) / 1000);
