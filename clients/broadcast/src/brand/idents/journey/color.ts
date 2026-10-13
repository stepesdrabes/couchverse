// Colours mixed in linear light, so blends between the palette's reds, near-black and warm white
// stay clean (sRGB mixing turns red into mud on the way to black).

export type RGB = [number, number, number];

const toLin = (c: number) => {
	const v = c / 255;
	return v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4;
};

export const toSrgb = (v: number) => {
	const x = Math.max(0, Math.min(1, v));
	return 255 * (x <= 0.0031308 ? x * 12.92 : 1.055 * x ** (1 / 2.4) - 0.055);
};

/** `#rrggbb` as linear RGB. */
export function hex(h: string): RGB {
	const n = parseInt(h.slice(1), 16);
	return [toLin((n >> 16) & 255), toLin((n >> 8) & 255), toLin(n & 255)];
}

export function mix(a: RGB, b: RGB, k: number): RGB {
	return [a[0] + (b[0] - a[0]) * k, a[1] + (b[1] - a[1]) * k, a[2] + (b[2] - a[2]) * k];
}

/** A colour for canvas fill and stroke styles. */
export function css(c: RGB, alpha = 1): string {
	const r = toSrgb(c[0]).toFixed(1);
	const g = toSrgb(c[1]).toFixed(1);
	const b = toSrgb(c[2]).toFixed(1);
	return alpha >= 1
		? `rgb(${r} ${g} ${b})`
		: `rgb(${r} ${g} ${b} / ${Math.max(0, alpha).toFixed(4)})`;
}

/** A colour ramp through [position, colour] stops, interpolated in linear light. */
export function ramp(stops: [number, RGB][]): (x: number) => RGB {
	return (x) => {
		if (x <= stops[0][0]) return stops[0][1];
		for (let i = 1; i < stops.length; i++) {
			if (x <= stops[i][0]) {
				const [x0, c0] = stops[i - 1];
				const [x1, c1] = stops[i];
				return mix(c0, c1, (x - x0) / (x1 - x0));
			}
		}
		return stops[stops.length - 1][1];
	};
}

/**
 * A canvas gradient that follows `ramp` in linear light: the canvas interpolates in sRGB
 * between stops, so the ramp is sampled densely enough that the difference never shows.
 */
export function gradientStops(
	g: CanvasGradient,
	at: (x: number) => RGB,
	n = 24,
	alpha?: (x: number) => number
) {
	for (let i = 0; i <= n; i++) {
		const x = i / n;
		g.addColorStop(x, css(at(x), alpha ? alpha(x) : 1));
	}
	return g;
}

/** The palette's anchors: Couchverse red, near-black and a warm white (the deeper reds live in the world's ramps). */
export const PAL = {
	red: hex('#e50914'),
	black: hex('#050505'),
	warm: hex('#fff2e4')
};
