// Smooth functions of time for the camera, the couch and the light: monotone cubics through
// keys (C1, never overshooting a key) and integrated speeds, so nothing on screen ever changes
// velocity in one frame.

export type Key = [number, number];

export const clamp01 = (v: number) => Math.max(0, Math.min(1, v));

/** 0 at a, 1 at b, smooth (C2) in between. */
export function smoother(a: number, b: number, x: number): number {
	const t = clamp01((x - a) / (b - a));
	return t * t * t * (t * (t * 6 - 15) + 10);
}

/** 0 at a, 1 at b, the classic smoothstep (C1). */
export function smooth(a: number, b: number, x: number): number {
	const t = clamp01((x - a) / (b - a));
	return t * t * (3 - 2 * t);
}

/**
 * A monotone cubic through keys (Fritsch-Carlson): passes every key, never overshoots between
 * them, flat before the first key and after the last.
 */
export function curve(keys: Key[]): (t: number) => number {
	const ks = [...keys].sort((a, b) => a[0] - b[0]);
	const n = ks.length;
	if (n === 1) return () => ks[0][1];
	const d: number[] = [];
	for (let i = 0; i < n - 1; i++) d.push((ks[i + 1][1] - ks[i][1]) / (ks[i + 1][0] - ks[i][0]));
	// the ends are flat: the value settles into its first and last keys
	const m: number[] = [0];
	for (let i = 1; i < n - 1; i++) m.push(d[i - 1] * d[i] <= 0 ? 0 : (d[i - 1] + d[i]) / 2);
	m.push(0);
	for (let i = 0; i < n - 1; i++) {
		if (d[i] === 0) {
			m[i] = 0;
			m[i + 1] = 0;
			continue;
		}
		const a = m[i] / d[i];
		const b = m[i + 1] / d[i];
		const s = a * a + b * b;
		if (s > 9) {
			const k = 3 / Math.sqrt(s);
			m[i] = k * a * d[i];
			m[i + 1] = k * b * d[i];
		}
	}
	return (t: number) => {
		if (t <= ks[0][0]) return ks[0][1];
		if (t >= ks[n - 1][0]) return ks[n - 1][1];
		let i = 0;
		while (t > ks[i + 1][0]) i++;
		const h = ks[i + 1][0] - ks[i][0];
		const u = (t - ks[i][0]) / h;
		const u2 = u * u;
		const u3 = u2 * u;
		return (
			(2 * u3 - 3 * u2 + 1) * ks[i][1] +
			(u3 - 2 * u2 + u) * h * m[i] +
			(-2 * u3 + 3 * u2) * ks[i + 1][1] +
			(u3 - u2) * h * m[i + 1]
		);
	};
}

/**
 * A position from a speed: the integral of `speed` from 0, tabulated once at 1 ms (and
 * extrapolated with the end speeds), so the position at t never depends on how t was reached.
 */
export function integral(speed: (t: number) => number, from = -1, to = 12): (t: number) => number {
	const dt = 0.001;
	const n = Math.ceil((to - from) / dt);
	const table = new Float64Array(n + 1);
	for (let i = 1; i <= n; i++) table[i] = table[i - 1] + speed(from + (i - 0.5) * dt) * dt;
	const at = (x: number) => {
		const f = (x - from) / dt;
		if (f <= 0) return (x - from) * speed(from);
		if (f >= n) return table[n] + (x - to) * speed(to);
		const i = Math.floor(f);
		return table[i] + (table[i + 1] - table[i]) * (f - i);
	};
	const zero = at(0);
	return (t: number) => at(t) - zero;
}
