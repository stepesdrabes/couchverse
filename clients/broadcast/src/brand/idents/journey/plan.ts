// The shared timeline of picture and score. 75 bpm: a beat is 0.8 s, exactly 20 frames, so every
// beat and sixteenth lands on a frame. Two bars of journey, the popcorn lands on bar three's
// downbeat, the sonic logo types couchtv a beat later and the rest of the bar rings out.

import { curve, integral, smoother } from './curves';

export const BEAT = 0.8;
export const D = 10.52;
export const at = (beat: number) => Math.round(beat * BEAT * 25) / 25;

/** The couch is alone on black, in its place in the lockup. */
export const ALONE = 5.6;
/** The popcorn drops for 0.72 s and lands on bar three's downbeat. */
export const LAND = at(8);
export const DROP = LAND - 0.72;
/** couch, t, v: the sonic logo, a beat after the landing, on sixteenths. */
export const NOTES = [at(9), at(9.25), at(9.5)];

/** Where the journey is, for the viewer's cue list. */
export const CUES: [string, number][] = [
	['the dunes', 0],
	['the sea', 1.2],
	['the sun sets behind the couch', 2.2],
	['the city lights up', 2.8],
	['the rise into the stars', 4.3],
	['the lights go down', 4.9],
	['the popcorn drops', DROP],
	['it lands: the logo', LAND],
	['couchtv', NOTES[0]]
];

// --- the camera ------------------------------------------------------------------------------

/** Where the horizon sits before the camera tilts up. */
export const H0 = 640;

/**
 * The truck: px/s of the couch's own plane (depth 1). It cruises, then gives its speed to the
 * crane as the camera rises.
 */
const truckSpeed = curve([
	[0, 560],
	[1.6, 600],
	[3.0, 560],
	[4.0, 430],
	[4.8, 170],
	[5.7, 0]
]);
export const truck = integral(truckSpeed);

/**
 * The camera's height above the ground (px at depth 1): it skims low over the dunes and the sea,
 * then cranes up. A crane moves near things fast and far things not at all, so the stars hold
 * still while the city sinks away below.
 */
const height = curve([
	[0, 175],
	[1.4, 188],
	[2.6, 200],
	[3.6, 225],
	[4.3, 300],
	[4.8, 500],
	[5.2, 930],
	[5.75, 1500]
]);

/** A gentle tilt up: everything moves down together, slowly enough to keep stars points. */
const tilt = curve([
	[0, 0],
	[4.0, 0],
	[5.1, 140],
	[5.8, 220]
]);

/** The lights go down: the city, the glow over it and the stars fade out to black. */
export const dim = (t: number) => smoother(4.55, 5.5, t);

export interface Camera {
	/** Truck position, px at depth 1. */
	x: number;
	/** Height above the ground, px at depth 1. */
	h: number;
	/** The horizon's screen y. */
	hz: number;
}

export const camera = (t: number): Camera => ({ x: truck(t), h: height(t), hz: H0 + tilt(t) });

/** Dusk to night: 0 while the sun is up, 1 once the city is lit. */
export const night = (t: number) => smoother(2.4, 4.2, t);

/** The sun's height above the horizon (px, its centre): it sets into the sea behind the couch. */
export const sunAlt = curve([
	[0, 230],
	[1.2, 160],
	[2.2, 70],
	[3.0, -55],
	[3.7, -300]
]);
export const SUN_X = 880;
export const SUN_R = 238;

// --- the couch -------------------------------------------------------------------------------

/** The lockup: the logo 540 px wide, its top-left at (690, 222), couchtv under it. */
export const LOCK = { w: 540, x: 690, y: 222, word: 184, wordY: 650 };

const couchX = curve([
	[0, 700],
	[2.2, 880],
	[3.4, 884],
	[4.4, 905],
	[ALONE, 960]
]);
const couchY = curve([
	[0, 636],
	[1.2, 580],
	[2.2, 522],
	[3.2, 505],
	[4.2, 498],
	[5.0, 470],
	[ALONE, 457.7]
]);
const couchScale = curve([
	[0, 0.36],
	[4.2, 0.37],
	[ALONE, LOCK.w / 1046]
]);

export interface Pose {
	/** The couch's centre (its visible box, logo units 523, 456.5), screen px. */
	x: number;
	y: number;
	/** Screen px per logo unit. */
	k: number;
	/** Degrees. */
	rot: number;
}

/** It floats: a slow bob and a lagging tilt that fade out as it comes to rest in the lockup. */
export function couchPose(t: number): Pose {
	const calm = 1 - smoother(4.2, ALONE, t);
	const w = (2 * Math.PI) / (4 * BEAT);
	const bob = Math.sin(w * t + 0.6) * 6 * calm;
	const rot = Math.sin(w * t - 0.5) * 1.1 * calm;
	return { x: couchX(t), y: couchY(t) + bob, k: couchScale(t), rot };
}
