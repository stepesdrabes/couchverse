// Note names, for every score: the house instruments and the old theme's orchestra alike.

const PC: Record<string, number> = {
	C: 0,
	'C#': 1,
	Db: 1,
	D: 2,
	'D#': 3,
	Eb: 3,
	E: 4,
	F: 5,
	'F#': 6,
	Gb: 6,
	G: 7,
	'G#': 8,
	Ab: 8,
	A: 9,
	'A#': 10,
	Bb: 10,
	B: 11
};

/** A note name (A4, F#3, Bb1) in Hz, equal temperament. */
export function hz(name: string): number {
	const m = /^([A-G][b#]?)(-?\d)$/.exec(name);
	if (!m) throw new Error(`bad note ${name}`);
	return 440 * 2 ** ((PC[m[1]] + 12 * (Number(m[2]) + 1) - 69) / 12);
}

export const pitchClass = (name: string): number => PC[/^([A-G][b#]?)/.exec(name)?.[1] ?? ''];
