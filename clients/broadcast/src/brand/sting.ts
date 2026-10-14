// A sting: the gap filler between promos, any whole number of frames from 5 to 10 s. One design,
// the ident's ending: the couch fades in alone, the popcorn drops onto it, couch, t and v type
// in on the sonic logo and the hold takes up the rest, while the chord rings out and the sound is
// over before the last frame. The shortest sting is as long as the logo needs to be stated whole
// and to die away (the last note has about two seconds), the longest stretches the build before
// the landing and the hold after it.

import type { Bus } from '../audio/engine';
import type { Kit } from '../audio/kit';
import { house } from '../audio/voices';
import { frame } from '../core/timeline';
import type { StingSpec } from '../spec';
import { timeline, type Template } from '../templates/shared';
import { ending, FALL, type EndingTimes } from './ending';
import {
	BEAT,
	END,
	popcornBuild,
	popcornFall,
	popcornLanding,
	SONIC,
	sonicRingOut,
	sonicSting
} from './sonic';

/** The shortest and the longest sting, seconds (the spec's bounds, and the composer's). */
export const STING = { min: 5, max: 10 };

/** The length at which the landing comes soonest: the couch, the popcorn's fall of 0.72 s. */
const SOONEST = 3.2;

/** When things happen in a sting of `D` seconds. */
export function stingTimes(D: number): EndingTimes {
	// what the length adds goes four parts to the moment before the landing, six to the hold
	const land = frame(0.8 + 0.4 * (D - SOONEST));
	const first = land + BEAT;
	return { land, couchIn: [0, 0.32], notes: [first, first + SONIC.at[1], first + SONIC.at[2]] };
}

export const sting: Template<StingSpec> = {
	version: 5,
	build({ root, sound }, spec) {
		const D = frame(Math.min(STING.max, Math.max(STING.min, spec.duration)));
		const tl = timeline(root, D);
		root.classList.add('sting');
		const times = stingTimes(D);
		ending(tl, root, times);
		tl.cue('the couch', 0);
		tl.cue('the popcorn falls', times.land - FALL);
		tl.cue('it lands: the logo', times.land);
		tl.cue('couchtv: the sonic logo', times.notes[0]);
		tl.sound(0, (w, b, skip) => score(sound, b, w - skip, times, D), D);
		return tl;
	}
};

/** `w` is the sting's zero on the context. */
function score(k: Kit, b: Bus, w: number, times: EndingTimes, D: number) {
	const h = house(k, b);
	const until = w + D - END;
	popcornBuild(h, w, w + times.land, w + D, { gain: 0.85 });
	popcornFall(h, w + times.land, Math.min(1.1, times.land - 0.1), { gain: 0.85 });
	popcornLanding(h, w + times.land, { gain: 0.8, ring: 1.4, until });
	sonicSting(k, b, w + times.notes[0], { until });
	sonicRingOut(h, w + times.notes[2], until);
}
