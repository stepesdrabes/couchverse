// Runtime accent theming. The admin picks a single accent colour; the strong
// and soft variants are derived from it so the whole palette shifts together.

function clampByte(n: number): number {
	return Math.max(0, Math.min(255, Math.round(n)));
}

function parseHex(hex: string): [number, number, number] | null {
	const m = /^#?([0-9a-f]{6})$/i.exec(hex.trim());
	if (!m) return null;
	const int = parseInt(m[1], 16);
	return [(int >> 16) & 255, (int >> 8) & 255, int & 255];
}

function toHex(r: number, g: number, b: number): string {
	return '#' + [r, g, b].map((v) => clampByte(v).toString(16).padStart(2, '0')).join('');
}

/** mix toward black (amount < 0) or white (amount > 0) */
function shade(rgb: [number, number, number], amount: number): string {
	const target = amount < 0 ? 0 : 255;
	const t = Math.abs(amount);
	return toHex(...(rgb.map((c) => c + (target - c) * t) as [number, number, number]));
}

/** Apply an accent colour by setting the palette CSS variables on :root. */
export function applyAccent(accent: string) {
	const rgb = parseHex(accent);
	if (!rgb) return;
	const root = document.documentElement.style;
	root.setProperty('--color-accent', toHex(...rgb));
	root.setProperty('--color-accent-strong', shade(rgb, -0.22));
	// soft tint over the dark background - low-alpha accent
	root.setProperty('--color-accent-soft', `rgba(${rgb[0]}, ${rgb[1]}, ${rgb[2]}, 0.16)`);
}
