import type { PlayTarget, Surface } from '$lib/generated/core';

/** The core's player screen. */
export const PLAYER: Surface = { type: 'player' };

export const sameTarget = (a: PlayTarget | undefined, b: PlayTarget | undefined) =>
	!!a && !!b && a.kind === b.kind && a.id === b.id;

/** The page that plays a title. Never data-preload it: the core starts streaming on arrival. */
export const watchPath = (target: PlayTarget) => `/watch/${target.kind}/${target.id}`;
