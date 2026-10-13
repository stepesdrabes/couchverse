// The Couch TV lockup: the Couchverse logo (glyph.ts) and the wordmark `couchtv`, lowercase in
// Plus Jakarta Sans ExtraBold at -0.035 em, `couch` in white and `tv` in the brand's red (as the
// app's own `couchverse` wordmark pairs white and red). Two layouts: `stack`, the logo above the
// word, as every ident ends; `row`, the logo left of the word for small sizes (the corner bug,
// the promo tag, the web's overlays), as the app's header pairs its logo with `couchverse`.
//
// The letters are outlines, not text: the bundled face's glyphs at weight 800 (HarfBuzz's
// hb-vector on src/assets/fonts' latin face), placed with its advances and the tracking as the
// browser lays the word out, so the lockup draws the same everywhere without a font load; a
// golden test draws them over the face's own text. The web imports this module directly
// (`@couchverse/broadcast/brand/wordmark`), so it stays light: no templates, no audio.

import { colors } from '../generated/tokens';
import { LOGO, LOGO_H, LOGO_W } from './glyph';

export const CHANNEL = 'Couch TV';

/**
 * The brand's colours: the red (the token accent), night (the field, the idents' black), paper
 * (the warm white of all type on air) and the white of the lockup's `couch`, which all four
 * approved idents end on.
 */
export const BRAND = {
	red: colors.accent,
	night: '#050505',
	paper: '#f4ede4',
	white: '#ffffff'
} as const;

/**
 * couchtv's three parts as outlines in font units (1000 per em), y down from the baseline, each
 * from its own origin; `x` is where it starts and `advance` how far the next one does (the
 * glyphs' advances with the -0.035 em tracking after every letter, as CSS and canvas apply it).
 */
export const PARTS = [
	{
		text: 'couch',
		x: 0,
		advance: 2896,
		d: 'M323 12Q241 12 175.5 -25.5Q110 -63 71.5 -128Q33 -193 33 -274Q33 -355 71 -419.5Q109 -484 175 -521Q241 -558 323 -558Q384 -558 436 -537Q488 -516 525 -478.5Q562 -441 578 -390L448 -334Q434 -375 400.5 -399Q367 -423 323 -423Q284 -423 253.5 -404Q223 -385 205.5 -351Q188 -317 188 -273Q188 -229 205.5 -195Q223 -161 253.5 -142Q284 -123 323 -123Q368 -123 401 -147Q434 -171 448 -212L578 -155Q563 -107 526 -69Q489 -31 437 -9.5Q385 12 323 12ZM902 12Q821 12 754.5 -25Q688 -62 648.5 -126.5Q609 -191 609 -273Q609 -356 648.5 -420Q688 -484 754.5 -521Q821 -558 902 -558Q983 -558 1049 -521Q1115 -484 1154.5 -420Q1194 -356 1194 -273Q1194 -191 1154.5 -126.5Q1115 -62 1049 -25Q983 12 902 12ZM902 -123Q943 -123 973.5 -142Q1004 -161 1021.5 -195Q1039 -229 1039 -273Q1039 -317 1021.5 -350.5Q1004 -384 973.5 -403.5Q943 -423 902 -423Q861 -423 830 -403.5Q799 -384 781.5 -350.5Q764 -317 764 -273Q764 -229 781.5 -195Q799 -161 830 -142Q861 -123 902 -123ZM1443 12Q1377 12 1330.5 -16.5Q1284 -45 1260 -96Q1236 -147 1236 -215L1236 -546L1386 -546L1386 -227Q1386 -196 1398.5 -172.5Q1411 -149 1434 -136Q1457 -123 1486 -123Q1516 -123 1538.5 -136Q1561 -149 1573.5 -172.5Q1586 -196 1586 -227L1586 -546L1736 -546L1736 0L1596 0L1596 -108L1604 -84Q1585 -35 1542.5 -11.5Q1500 12 1443 12ZM2079 12Q1997 12 1931.5 -25.5Q1866 -63 1827.5 -128Q1789 -193 1789 -274Q1789 -355 1827 -419.5Q1865 -484 1931 -521Q1997 -558 2079 -558Q2140 -558 2192 -537Q2244 -516 2281 -478.5Q2318 -441 2334 -390L2204 -334Q2190 -375 2156.5 -399Q2123 -423 2079 -423Q2040 -423 2009.5 -404Q1979 -385 1961.5 -351Q1944 -317 1944 -273Q1944 -229 1961.5 -195Q1979 -161 2009.5 -142Q2040 -123 2079 -123Q2124 -123 2157 -147Q2190 -171 2204 -212L2334 -155Q2319 -107 2282 -69Q2245 -31 2193 -9.5Q2141 12 2079 12ZM2387 0L2387 -757L2537 -757L2537 -438L2519 -462Q2538 -511 2580.5 -534.5Q2623 -558 2680 -558Q2742 -558 2788.5 -532Q2835 -506 2861 -459.5Q2887 -413 2887 -351L2887 0L2737 0L2737 -319Q2737 -351 2724.5 -374Q2712 -397 2689.5 -410Q2667 -423 2637 -423Q2608 -423 2585 -410Q2562 -397 2549.5 -374Q2537 -351 2537 -319L2537 0L2387 0Z'
	},
	{
		text: 't',
		x: 2896,
		advance: 386,
		d: 'M318 6Q219 6 164.5 -47.5Q110 -101 110 -197L110 -416L18 -416L18 -546L23 -546Q65 -546 87.5 -567Q110 -588 110 -630L110 -670L260 -670L260 -546L388 -546L388 -416L260 -416L260 -207Q260 -179 270 -160.5Q280 -142 301 -133Q322 -124 353 -124Q360 -124 369 -125Q378 -126 388 -127L388 0Q373 2 354 4Q335 6 318 6Z'
	},
	{
		text: 'v',
		x: 3282,
		advance: 547,
		d: 'M226 0L11 -546L173 -546L322 -133L260 -133L409 -546L571 -546L356 0L226 0Z'
	}
] as const;

/**
 * The word's metrics in em: its advance (where a centred line box puts its middle), the ink's
 * left and right (the c's side bearing, the v past the last tracking), the baseline in a
 * line-height 1 box (the face's ascent 1.038 and descent 0.222 centred in it) and the ink's
 * depth under the baseline (the round letters' overshoot).
 */
export const WORD = {
	advance: 3.829,
	inkLeft: 0.033,
	inkRight: 3.853,
	baseline: 0.908,
	depth: 0.012
} as const;

export type LockupLayout = 'row' | 'stack';

/** Where things go, in px from the lockup's top left (its ink box). */
export interface Geometry {
	width: number;
	height: number;
	/** The logo's top left and px per logo unit. */
	logo: { x: number; y: number; k: number };
	/** The word's origin on its baseline and px per font unit. */
	word: { x: number; y: number; k: number };
}

/** The stacked lockup as the idents end on it: the logo 540 px wide above a 184 px word. */
const STACK = { logo: 540 / 184, wordTop: (650 - 222) / 184 } as const;
/**
 * The row lockup: the logo 36/34 em tall (the corner bug's 36 px logo beside a 34 px word), the
 * word's line box (line-height 1) sharing its top, so the word sits on the logo's middle and
 * on one line with the text beside it, its c 0.32 em from the logo.
 */
export const ROW = { logo: 36 / 34, gap: 0.32 } as const;

/** The lockup's geometry with the word at `size` px. */
export function geometry(size: number, layout: LockupLayout = 'row'): Geometry {
	const s = size;
	const wk = s / 1000;
	if (layout === 'stack') {
		const lk = (STACK.logo * s) / LOGO_W;
		// the logo centred over the word's advance, as the idents centre both on the stage
		const logoX = (WORD.advance / 2) * s - (LOGO_W * lk) / 2;
		const left = Math.min(WORD.inkLeft * s, logoX);
		const baseline = (STACK.wordTop + WORD.baseline) * s;
		return {
			width: Math.max(WORD.inkRight * s, logoX + LOGO_W * lk) - left,
			height: baseline + WORD.depth * s,
			logo: { x: logoX - left, y: 0, k: lk },
			word: { x: -left, y: baseline, k: wk }
		};
	}
	const lk = (ROW.logo * s) / LOGO_H;
	const wordX = LOGO_W * lk + (ROW.gap - WORD.inkLeft) * s;
	return {
		width: wordX + WORD.inkRight * s,
		height: LOGO_H * lk,
		logo: { x: 0, y: 0, k: lk },
		word: { x: wordX, y: WORD.baseline * s, k: wk }
	};
}

export interface LockupOptions {
	/** The word's size, px: a row is 1.06 of it tall and 5.63 wide, a stack 3.25 tall and 3.82 wide. */
	size: number;
	layout?: LockupLayout;
	/** `couch`. */
	ink?: string;
	/** The logo and `tv`. */
	brand?: string;
	/** An accessible name (`Couch TV`); without it the lockup is decorative. */
	label?: string;
}

const SVG_NS = 'http://www.w3.org/2000/svg';

const num = (v: number) => String(Math.round(v * 100) / 100);
const attr = (v: string) => v.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;');

/** The SVG transform that draws `units` at a placement. */
export const placed = (p: { x: number; y: number; k: number }) =>
	`translate(${num(p.x)} ${num(p.y)}) scale(${p.k.toFixed(6)})`;

/**
 * The static lockup as a standalone SVG string, a pure function of its options: paths only, so
 * it needs no font. The broadcast's corner bug is drawn by it, so a client overlay built from it
 * matches the picture.
 */
export function lockupSvg({
	size,
	layout = 'row',
	ink = BRAND.white,
	brand = BRAND.red,
	label
}: LockupOptions): string {
	const g = geometry(size, layout);
	const w = num(g.width);
	const h = num(g.height);
	const a11y = label ? `role="img" aria-label="${attr(label)}"` : 'aria-hidden="true"';
	const [couch, t, v] = PARTS;
	return (
		`<svg xmlns="${SVG_NS}" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}" ${a11y}>` +
		`<path fill="${attr(brand)}" transform="${placed(g.logo)}" d="${LOGO}"/>` +
		`<g transform="${placed(g.word)}">` +
		`<path fill="${attr(ink)}" d="${couch.d}"/>` +
		`<g fill="${attr(brand)}"><path transform="translate(${t.x} 0)" d="${t.d}"/>` +
		`<path transform="translate(${v.x} 0)" d="${v.d}"/></g></g></svg>`
	);
}
