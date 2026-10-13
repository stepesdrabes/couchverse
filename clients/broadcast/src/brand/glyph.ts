// The Couchverse logo, taken apart: the couch (its lower body, and the backrest dome that the
// popcorn hides, completed along the logo's own curves) and the popcorn bucket. Drawn
// together with the popcorn's knockout they are the logo; the couch alone travels the worlds.
// Units are the logo's (1046 x 745).

import glyph from './couchverse-glyph.json';

export const LOGO_W = 1046;
export const LOGO_H = 745;
/** The couch alone starts below the popcorn: its backrest's top. */
export const COUCH_TOP = 168;

export const COUCH = glyph.lower + glyph.dome;
export const POPCORN = glyph.popcorn;
export const LOGO = glyph.logo;

/** The knockout around a glyph in its field's colour (the logo's white gaps are ~30 units). */
export const GAP = 30;

const SVG = 'http://www.w3.org/2000/svg';

export interface Glyph {
	/** The glyph's group, in logo units: place it with a transform. */
	g: SVGGElement;
	set(fill: string, field: string): void;
}

/**
 * A glyph group: a halo in the field colour (the knockout, which also fills the glyph's own
 * gaps, so it reads the same on any background), then the glyph.
 */
export function glyphGroup(parent: Element, d: string, { gap = GAP, cls = '' } = {}): Glyph {
	const g = document.createElementNS(SVG, 'g');
	if (cls) g.setAttribute('class', cls);
	const halo = document.createElementNS(SVG, 'path');
	halo.setAttribute('d', d);
	halo.setAttribute('stroke-width', String(gap * 2));
	halo.setAttribute('stroke-linejoin', 'round');
	const body = document.createElementNS(SVG, 'path');
	body.setAttribute('d', d);
	g.append(halo, body);
	parent.appendChild(g);
	return {
		g,
		set(fill, field) {
			halo.setAttribute('fill', field);
			halo.setAttribute('stroke', field);
			body.setAttribute('fill', fill);
		}
	};
}
