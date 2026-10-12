import type { PlayTarget, Surface } from '$lib/generated/core';

/** The core's player screen. */
export const PLAYER: Surface = { type: 'player' };

export const sameTarget = (a: PlayTarget | undefined, b: PlayTarget | undefined) =>
	!!a && !!b && a.kind === b.kind && a.id === b.id;

/** The page that plays a title. Never data-preload it: the core starts streaming on arrival. */
export const watchPath = (target: PlayTarget) => `/watch/${target.kind}/${target.id}`;

/**
 * A live channel in the player (VideoPlayer's `live`): no seeking and no pausing, the progress
 * of what is on in place of the seek bar, and the channel's own audio and subtitle choices.
 */
export interface LivePlayback {
	/** How far what is on screen has come, from 0 to 1. */
	progress?: number;
	/** Where the top bar's back button goes; none for a guest, who has nowhere else to be. */
	back?: string;
	chooseAudio(id: string): void;
	chooseSubtitles(id: string | null): void;
}
