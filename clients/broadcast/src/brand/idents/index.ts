// The full idents (D35): four designs of 10.52 s, each with its own picture and score, that end
// the same way: the couch alone on black, a bucket of popcorn drops onto it and they become the
// Couchverse logo, then "couch", "t", "v" type in on the sonic logo's three notes (A4, E5, D5).
// Each picture is a canvas (assemble: stacked SVG copies) drawn from t; a render averages
// sub-frames over a half-frame shutter (motion blur, `wait !== null` in `Timeline.dynamic`),
// live playback draws one sample.

import type { IdentSpec } from '../../spec';
import type { Ctx, TL } from '../../templates/shared';
import { assemble } from './assemble/ident';
import { cinematic } from './cinematic/ident';
import { journey } from './journey/ident';
import { morph } from './morph/ident';

export type IdentDesign = NonNullable<IdentSpec['design']>;

const DESIGNS: Record<IdentDesign, (ctx: Ctx) => TL> = { journey, cinematic, morph, assemble };

/** The designs in the order the channel rotates them (the server's `identDesigns`). */
export const IDENT_DESIGNS = Object.keys(DESIGNS) as IdentDesign[];

/** The full cut in the spec's design (journey without one). */
export function buildFullIdent(ctx: Ctx, spec: IdentSpec): TL {
	return (DESIGNS[spec.design ?? 'journey'] ?? journey)(ctx);
}
