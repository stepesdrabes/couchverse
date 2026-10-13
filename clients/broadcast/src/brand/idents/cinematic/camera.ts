// The camera's path. A slow push on the dusk hill, a whip east to the town, then one long dolly
// into the lit window and through the room until the couch sits exactly where the lockup has it,
// arriving as the popcorn lands.
//
// The dolly is laid out as the couch's apparent size, eased in log space (a zoom reads as even
// when its ratio per frame is even); its speed is two smootherstep halves meeting at the peak,
// which falls as the camera passes the window frame, on beat 7.

import { type Cam, F, type Ref, smooth } from './art';
import { LOGO_H, LOGO_W } from './couch';
import { T } from './time';

/** How far the panorama turns from the hill to the town, in screen pixels. */
export const P = 2600;

const PUSH = 250;
const DRIFT = 40;
/** The camera sinks a little on the hill, so the couch rises into the sun as it grows. */
const SINK = 14;

export const REF_A: Ref = { pan: 0, tilt: 0, x: 0, y: 0, z: 0 };
/** Where the town is drawn from: the camera as the whip settles. */
export const REF_B: Ref = { pan: P, tilt: 0, x: DRIFT, y: SINK, z: PUSH };

/** The lockup: the logo 540 px wide, its top at y = 222 (the approved ending's layout). */
export const LOCK = { w: 540, x: 960 - 540 / 2, y: 222, k: 540 / LOGO_W };

const COUCH_Z = 1000;
const COUCH_PX = 150;
/** The couch's size at the end relative to the reference view (the lockup's 540 px). */
const ZOOM = LOCK.w / COUCH_PX;
const LZ = Math.log(ZOOM);
const D1 = T.dollyPeak - T.dollyFrom;
const D2 = T.dollyTo - T.dollyPeak;
const VMAX = (2 * LZ) / (D1 + D2);
/** The integral of smootherstep from 0 to x. */
const I = (x: number) => x ** 6 - 3 * x ** 5 + 2.5 * x ** 4;

/** The dolly's progress in log zoom, 0..LZ. */
function zoomLog(t: number) {
	if (t <= T.dollyFrom) return 0;
	if (t <= T.dollyPeak) return VMAX * D1 * I((t - T.dollyFrom) / D1);
	if (t <= T.dollyTo) {
		const x = (t - T.dollyPeak) / D2;
		return VMAX * D1 * 0.5 + VMAX * D2 * (x - I(x));
	}
	return LZ;
}

/** The façade stands where the camera is at the dolly's fastest. */
const WALL_Z = COUCH_Z - COUCH_Z / Math.exp(VMAX * D1 * 0.5);
/** The lit window as seen from B's reference camera (screen pixels). */
export const WIN_PX = { x0: 700, x1: 1220, y0: 330, y1: 670 };

/** The room, in B's world (its reference camera at the origin, looking down +Z, y down). */
export const ROOM = {
	/** The façade's outer and inner faces, and the window opening in it. */
	wallZ: WALL_Z,
	wallIn: WALL_Z + 15,
	win: {
		x0: ((WIN_PX.x0 - 960) * WALL_Z) / F,
		x1: ((WIN_PX.x1 - 960) * WALL_Z) / F,
		y0: ((WIN_PX.y0 - 540) * WALL_Z) / F,
		y1: ((WIN_PX.y1 - 540) * WALL_Z) / F
	},
	x0: -300,
	x1: 300,
	ceil: -215,
	floor: 110,
	back: 1110,
	/** The couch: its feet on the floor, centred, 150 px wide from the reference camera. */
	couchZ: COUCH_Z,
	couchPx: COUCH_PX
};

/** The camera's height at the end: level with the couch's middle. */
const DROP = 70;
const D_END = COUCH_Z / ZOOM;
/** The lens shift that puts the couch's feet where the lockup has them. */
const SHIFT = (() => {
	const s = COUCH_PX / LOGO_W;
	const feet = 540 + (F * ROOM.floor) / COUCH_Z;
	// the couch drawn from the reference: its logo origin at y = feet - LOGO_H * s
	const top = feet - LOGO_H * s;
	return 540 + (top - 540) * ZOOM - (DROP * F) / D_END - LOCK.y;
})();

export function camera(t: number): Cam {
	const pan = P * smooth(T.panFrom, T.panTo, t);
	// the push on the hill is already moving on the first frame and settles as the camera turns
	const u = Math.min(1, Math.max(0, t / T.dollyFrom));
	const push = 1 - (1 - u) * (1 - u);
	const v = Math.min(1, Math.max(0, t / T.panTo));
	const drift = 1 - (1 - v) * (1 - v);
	const p = zoomLog(t) / LZ;
	const zoom = Math.exp(zoomLog(t));
	const dz = COUCH_Z - COUCH_Z / zoom;
	return {
		pan,
		tilt: SHIFT * p,
		x: DRIFT * drift,
		// the camera stays high through the window and comes down to the couch inside
		y: SINK * push + DROP * p * p,
		z: PUSH * push + dz
	};
}

/** Whether the camera has reached the lockup framing. */
export const settled = (t: number) => t >= T.dollyTo;
