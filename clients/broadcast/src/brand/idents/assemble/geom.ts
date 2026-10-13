// The couch of the Couchverse logo taken apart into the primitives it is drawn from: two
// circles (the rolled arms), the slab (the seat and the base), the posts that join them, the
// dome (the backrest) and two feet. Together, in one colour, they cover exactly the glyph's
// couch (its `lower` and `dome` paths), so the built couch is the logo's own. Logo units
// (1046 x 745).

import glyph from '../../couchverse-glyph.json';

export const ARM_R = 97.59;
export const ARM_Y = 480.33;
export const ARM_LX = 97.59;
export const ARM_RX = 947.48;
/** The axis the couch is symmetric about. */
export const MID_X = (ARM_LX + ARM_RX) / 2;
export const SEAT_Y = 553.646;

/** The slab: the seat and the base, its top the seat line, its lower corners rounded. */
export const BASE =
	'M102.6 553.646H942.35V632.993' +
	'C942.376 676.819 906.854 712.342 863.028 712.342H181.996' +
	'C138.176 712.342 102.648 676.819 102.648 632.993Z';

/**
 * The posts that join the slab to the arms: under each arm, from the seat line up to the arm's
 * centre; their inner sides are the lines the glyph drops from the arms to the seat. Inside the
 * circles they are hidden; what shows is the notch between a circle and the seat they fill.
 */
export const POST_L = 'M102.6 478H195.13V554.4H102.6Z';
export const POST_R = 'M849.92 478H942.35V554.4H849.92Z';

/** A foot, overlapping the slab a hair so the join never shows a seam. */
const foot = (x0: number, x1: number) =>
	`M${x0} 711.6H${x1}V735.243C${x1} 740.112 ${x1 - 3.951} 744.062 ${x1 - 8.82} 744.062` +
	`H${x0 + 8.819}C${x0 + 3.951} 744.062 ${x0} 740.112 ${x0} 735.243Z`;

export const FOOT_L = foot(199.473, 275.806);
export const FOOT_R = foot(769.264, 845.597);
export const FOOT_LX = (199.473 + 275.806) / 2;
export const FOOT_RX = (769.264 + 845.597) / 2;

export const DOME: string = glyph.dome;
export const DOME_BOTTOM = 520.1;

export const POPCORN: string = glyph.popcorn;
