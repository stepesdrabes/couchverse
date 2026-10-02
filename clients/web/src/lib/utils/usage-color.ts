// Usage percentage -> status colour: solid green up to 50%, blending to
// orange at 75% and red at 100%.
type Rgb = [number, number, number];

const GREEN: Rgb = [16, 185, 129]; // #10b981
const ORANGE: Rgb = [245, 158, 11]; // #f59e0b
const RED: Rgb = [239, 68, 68]; // #ef4444

function mix(a: Rgb, b: Rgb, t: number): Rgb {
	return [0, 1, 2].map((i) => Math.round(a[i] + (b[i] - a[i]) * t)) as Rgb;
}

export function usageColor(pct: number): string {
	const p = Math.min(100, Math.max(0, pct));
	const [r, g, b] =
		p <= 50 ? GREEN : p <= 75 ? mix(GREEN, ORANGE, (p - 50) / 25) : mix(ORANGE, RED, (p - 75) / 25);
	return `rgb(${r}, ${g}, ${b})`;
}
