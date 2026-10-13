// A sting: the gap filler between promos, any whole number of frames from 2 to 6 s. One design,
// the ident's ending, showing as much of the popcorn moment as the length allows: from 3.2 s the
// couch fades in alone and the popcorn drops onto it (later the longer the sting), shorter ones
// resolve the whole logo in; either way couch, t and v type in on the sonic logo and the hold
// takes up the rest.

import type { Bus } from '../audio/engine';
import type { Kit } from '../audio/kit';
import { fadeOut, house } from '../audio/voices';
import { frame } from '../core/timeline';
import type { StingSpec } from '../spec';
import { timeline, type Template } from '../templates/shared';
import { ending, FALL, type EndingTimes } from './ending';
import { BEAT, popcornBuild, popcornFall, popcornLanding, SONIC, sonicSting } from './sonic';

/** The shortest sting that shows the popcorn drop: the couch, the fall, a beat, the motif, a hold. */
const DROP_FROM = 3.2;

/** When things happen in a sting of `D` seconds. */
export function stingTimes(D: number): EndingTimes {
	const notes = (first: number) =>
		[first, first + SONIC.at[1], first + SONIC.at[2]] as [number, number, number];
	if (D >= DROP_FROM) {
		// what the length adds goes four parts to the moment before the landing, six to the hold
		const land = frame(0.8 + 0.4 * (D - DROP_FROM));
		return { land, couchIn: [0, 0.32], notes: notes(land + BEAT) };
	}
	return { land: null, resolve: [0, 0.4], notes: notes(frame(0.4 + 0.5 * (D - 2))) };
}

export const sting: Template<StingSpec> = {
	version: 4,
	build({ root, sound }, spec) {
		const D = frame(Math.min(6, Math.max(2, spec.duration)));
		const tl = timeline(root, D);
		root.classList.add('sting');
		const times = stingTimes(D);
		ending(tl, root, times);
		const first = times.notes[0];
		if (times.land !== null) {
			tl.cue('the couch', 0);
			tl.cue('the popcorn falls', times.land - FALL);
			tl.cue('it lands: the logo', times.land);
		} else tl.cue('the logo resolves in', 0);
		tl.cue('couchtv: the sonic logo', first);
		tl.sound(0, (w, b, skip) => score(sound, b, w - skip, times, D), D);
		return tl;
	}
};

/** `w` is the sting's zero on the context. */
function score(k: Kit, b: Bus, w: number, times: EndingTimes, D: number) {
	const h = house(k, b);
	const first = times.notes[0];
	if (times.land !== null) {
		popcornBuild(h, w, w + times.land, w + D, { gain: 0.85 });
		popcornFall(h, w + times.land, Math.min(1.1, times.land - 0.1), { gain: 0.85 });
		popcornLanding(h, w + times.land, { gain: 0.8, ring: 1.4 });
	}
	// a resolving logo has the swell from its first frame into the first note
	sonicSting(k, b, w + first, { lead: times.land === null ? first : 0 });
	fadeOut(h, w + D - 0.6, 0.6, 0.1);
}
