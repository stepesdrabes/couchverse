// The short ident (4.52 s): the popcorn ending every full ident closes on, alone. It opens on
// the couch alone on black in its place in the lockup (the frame the credits squeeze hands
// over into), the popcorn drops onto it and they are the Couchverse logo, couch, t and v type
// in on the sonic logo, and the lockup holds to the end.
//
// Score, on the sonic logo's 75 bpm: the dominant (A sus4) swells under the couch alone while
// the arrival chord rises reversed into the landing, a breath of silence, then the landing (the
// impact and D major add 9, wide and low), the sonic logo a beat later and its chord ringing out.

import type { Bus } from '../audio/engine';
import type { Kit } from '../audio/kit';
import { fadeOut, house } from '../audio/voices';
import { frame } from '../core/timeline';
import { timeline, type Ctx, type TL } from '../templates/shared';
import { ending, type EndingTimes } from './ending';
import { BEAT, popcornBuild, popcornFall, popcornLanding, SONIC, sonicLogo } from './sonic';

export const SHORT_IDENT = 4.52;

/** The landing; the sonic logo's first note comes a beat after it. */
const LAND = 1.2;

export const SHORT_IDENT_TIMES: EndingTimes = {
	land: LAND,
	notes: [LAND + BEAT, LAND + BEAT + SONIC.at[1], LAND + BEAT + SONIC.at[2]]
};

/** The short ident: nothing in its spec changes it. */
export function buildShortIdent({ root, sound }: Ctx): TL {
	const D = frame(SHORT_IDENT);
	const tl = timeline(root, D);
	root.classList.add('ident');
	ending(tl, root, SHORT_IDENT_TIMES);
	const [n0] = SHORT_IDENT_TIMES.notes;
	tl.cue('the couch (handed over by the squeeze)', 0);
	tl.cue('the popcorn falls', LAND - 0.72);
	tl.cue('it lands: the logo', LAND);
	tl.cue('couchtv: the sonic logo', n0);
	tl.sound(0, (w, b, skip) => score(sound, b, w - skip, D), D);
	return tl;
}

function score(k: Kit, b: Bus, w: number, D: number) {
	const h = house(k, b);
	popcornBuild(h, w, w + LAND, w + D);
	popcornFall(h, w + LAND, LAND - 0.1);
	popcornLanding(h, w + LAND);
	sonicLogo(h, w + SHORT_IDENT_TIMES.notes[0]);
	// the hall carries the end down, so the cut to what follows lands on near silence
	fadeOut(h, w + D - 1.0, 1.0, 0.08);
}
