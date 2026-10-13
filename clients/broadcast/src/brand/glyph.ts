// The Couchverse logo (clients/web/static/logo.svg) taken apart: the couch (its lower body, and
// the backrest dome that the popcorn hides, completed along the logo's own curves) and the
// popcorn bucket. The popcorn knocked out of the couch by LOGO_GAP around its silhouette (its
// outer contour, so nothing behind the bucket shows through its holes) gives the logo exactly;
// a golden test rasterizes both. The full idents knock it out with a halo round its whole path
// (GAP), as their concepts drew it. Units are the logo's (1046 x 745).

import glyph from './couchverse-glyph.json' with { type: 'json' };

export const LOGO_W = 1046;
export const LOGO_H = 745;
/** The couch alone starts below the popcorn: its backrest's top. */
export const COUCH_TOP = 168;

export const COUCH = glyph.lower + glyph.dome;
export const POPCORN = glyph.popcorn;
/** The popcorn's outer contour: its knockout. */
export const SILHOUETTE = POPCORN.slice(0, POPCORN.indexOf('M', 1));
/** The logo in one path, as the web's logo.svg draws it. */
export const LOGO = glyph.logo;

/** The couch's box, [x0, y0, x1, y1]: the dome's top to the feet. */
export const COUCH_BOX = [0, 178.611, 1045.07, 744.062] as const;
/** The popcorn's box. */
export const POPCORN_BOX = [303.5, 0, 741.502, 614] as const;

/** The logo's own gap round the popcorn's silhouette. */
export const LOGO_GAP = 28.5;
/** The knockout around a glyph in its field's colour (the logo's white gaps are ~30 units). */
export const GAP = 30;

/** The popcorn tilts about a point low in the bucket. */
export const PIVOT = { x: 500, y: 600 } as const;
