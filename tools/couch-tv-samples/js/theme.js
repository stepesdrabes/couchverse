// The Couch TV theme in three arrangements over one grid and one hook.
//
// Grid: 120 bpm (a beat is 0.5 s). The full cut builds for three bars and hits at 6.0; the
// short cut builds for one bar and hits at 2.0. Picture (ident.js) and score both read GRID,
// so the brackets snap shut on the hit and the TV block lands on the hook's last note.
//
// Hook (the sonic logo, five notes): A4 D5 A5 G5 F#5, "da-da DAA da DAAA". Up a fourth and a
// fifth (the brackets opening), down two steps to the major third (closing). Rhythm: two
// eighths, a quarter, an eighth, then the long note pushed onto the "and" of beat three.

import { hz, pitchClass, seedOf } from './synth.js';

export const BEAT = 0.5;
export const GRID = {
	full: { cut: 'full', D: 10.5, hit: 6.0, final: 8.0 },
	short: { cut: 'short', D: 4.5, hit: 2.0, final: 3.25 }
};
export const HOOK = {
	notes: ['A4', 'D5', 'A5', 'G5', 'F#5'],
	at: [0, 0.25, 0.5, 1.0, 1.25],
	// the promo tag states it a touch quicker
	tagAt: [0, 0.2, 0.4, 0.8, 1.0]
};

const SCALES = {
	'D minor': ['D', 'E', 'F', 'G', 'A', 'Bb', 'C'],
	'D major': ['D', 'E', 'F#', 'G', 'A', 'B', 'C#'],
	'D mixolydian': ['D', 'E', 'F#', 'G', 'A', 'B', 'C']
};

const F = (names) => names.map(hz);
const ev = () => {
	const list = [];
	// `dur` lets a long event start part-way through when playback starts inside it; the
	// callback always gets the event's own (possibly past) start time
	const at = (t, fn, dur = 0) => list.push({ at: t, dur, fn: (w, b, skip = 0) => fn(w - skip, b) });
	return { list, at };
};

// --- charts: every chord of every arrangement, with the key it must belong to --------------

export const CHARTS = {
	1: {
		Dm: { notes: ['D2', 'A2', 'D3', 'F3'], key: 'D minor' },
		Bb: { notes: ['Bb1', 'F2', 'Bb2', 'D3'], key: 'D minor' },
		C: { notes: ['C2', 'G2', 'C3', 'E3'], key: 'D minor' },
		Cstr: { notes: ['C3', 'G3', 'C4', 'E4'], key: 'D minor' },
		Chorn: { notes: ['G3', 'C4', 'E4'], key: 'D minor' },
		Dhit: { notes: ['D2', 'A2', 'D3', 'F#3'], key: 'D major' },
		Dstr: { notes: ['D3', 'A3', 'D4', 'F#4'], key: 'D major' },
		CDstr: { notes: ['D3', 'G3', 'C4', 'E4'], key: 'D mixolydian' },
		Dchoir: { notes: ['D4', 'F#4', 'A4'], key: 'D major' },
		Dfinal: { notes: ['D2', 'A2', 'D3', 'A3', 'E4', 'F#4', 'A4'], key: 'D major' },
		Dbrass: { notes: ['D3', 'A3', 'D4', 'F#4'], key: 'D major' },
		Dchoir2: { notes: ['A3', 'D4', 'F#4', 'A4'], key: 'D major' }
	},
	2: {
		Bm: { notes: ['B3', 'D4', 'F#4'], key: 'D major' },
		G: { notes: ['B3', 'D4', 'G4'], key: 'D major' },
		D: { notes: ['A3', 'D4', 'F#4'], key: 'D major' },
		A: { notes: ['A3', 'C#4', 'E4'], key: 'D major' },
		Aup: { notes: ['A3', 'C#4', 'E4', 'A4'], key: 'D major' },
		Dfinal: { notes: ['D3', 'A3', 'D4', 'E4', 'F#4', 'A4'], key: 'D major' },
		bass: { notes: ['B1', 'G1', 'D2', 'A1', 'D1'], key: 'D major' }
	},
	3: {
		Dadd9: { notes: ['D3', 'A3', 'F#4'], key: 'D major' },
		arpD: { notes: ['D4', 'A4', 'E5', 'F#5'], key: 'D major' },
		Bm7: { notes: ['B2', 'F#3', 'A3', 'D4'], key: 'D major' },
		G: { notes: ['G2', 'D3', 'B3', 'D4'], key: 'D major' },
		Em7: { notes: ['E3', 'B3', 'D4', 'G4'], key: 'D major' },
		Asus: { notes: ['A2', 'E3', 'A3', 'D4'], key: 'D major' },
		A: { notes: ['A2', 'E3', 'A3', 'C#4'], key: 'D major' },
		Dhit: { notes: ['D3', 'A3', 'D4', 'F#4', 'A4'], key: 'D major' },
		GD: { notes: ['D3', 'G3', 'B3', 'D4', 'G4'], key: 'D major' },
		Dfinal: { notes: ['D2', 'A2', 'D3', 'A3', 'E4', 'F#4'], key: 'D major' },
		Dchoir: { notes: ['D4', 'F#4', 'A4'], key: 'D major' },
		horns: { notes: ['A3', 'D4', 'F#4'], key: 'D major' },
		gliss: { notes: ['D4', 'E4', 'F#4', 'A4', 'B4', 'D5', 'E5', 'F#5', 'A5', 'B5', 'D6'], key: 'D major' }
	}
};

/** Every chord against its key, and the hook against D major: returns the offenders. */
export function keyReport() {
	const bad = [];
	for (const [theme, chart] of Object.entries(CHARTS)) {
		for (const [name, { notes, key }] of Object.entries(chart)) {
			const scale = SCALES[key].map(pitchClass);
			for (const n of notes) if (!scale.includes(pitchClass(n))) bad.push(`${theme}/${name}: ${n} not in ${key}`);
		}
	}
	for (const n of HOOK.notes) if (!SCALES['D major'].map(pitchClass).includes(pitchClass(n))) bad.push(`hook: ${n}`);
	return { checked: Object.values(CHARTS).reduce((s, c) => s + Object.keys(c).length, 0) + 1, bad };
}

const C1 = (k) => F(CHARTS[1][k].notes);
const C2 = (k) => F(CHARTS[2][k].notes);
const C3 = (k) => F(CHARTS[3][k].notes);
const HOOKF = F(HOOK.notes);
const oct = (fs, k) => fs.map((f) => f * k);

/** The hook at `t0` with rhythm `R` on a voice: fn(when, freq, len, index). */
function hookNotes(at, t0, R, fn, { last = 1.6 } = {}) {
	R.forEach((r, i) => {
		const len = i < 4 ? R[i + 1] - r - 0.02 : last;
		at(t0 + r, (w, b) => fn(w, b, HOOKF[i], len, i), len);
	});
}

// =========================================================================================
// Theme 1: hybrid epic. D minor (i, bVI, bVII) builds to a D major lift on the hit.
// =========================================================================================

function theme1(e, g) {
	const { list, at } = ev();
	const H = g.hit;
	const full = g.cut === 'full';
	const end = g.final;

	// --- the build -------------------------------------------------------------------------
	if (full) {
		at(0, (w, b) => {
			e.taikos(b, w, { gain: 0.3, f: 50, decay: 1.4 });
			e.braam(b, w, C1('Dm'), { gain: 0.24, dur: 2.2, drive: 1.3, open: [160, 1500, 520] });
			e.subNote(b, w, hz('D1'), 1.8, { gain: 0.1, a: 0.05, r: 0.4 });
		}, 2);
		// low strings hold the tonic under the typing, then lean into the build
		at(0.05, (w, b) => e.strings(b, w, F(['D2', 'A2', 'D3']), 5.7, { gain: 0.112, a: 1.2, r: 0.2, cutoff: 1400, hp: 50, swell: [[0, 0.6], [4, 1], [5.8, 1.4]] }), 5.9);
		for (const [t, f, gn] of [[1.0, 62, 0.16], [1.5, 70, 0.22], [1.75, 70, 0.16]]) at(t, (w, b) => e.drum(b, w, { gain: gn, f, decay: 0.7, pan: -0.2, seed: seedOf(t) }));
		at(2.0, (w, b) => {
			e.taikos(b, w, { gain: 0.46, f: 47, decay: 1.4 });
			e.braam(b, w, C1('Bb'), { gain: 0.26, dur: 2.1, drive: 1.3, open: [170, 1800, 560] });
			e.subNote(b, w, hz('Bb1'), 1.8, { gain: 0.09, a: 0.02, r: 0.3 });
		}, 2);
		for (const [t, f, gn, p] of [[2.75, 88, 0.24, 0.3], [3.0, 66, 0.3, -0.2], [3.5, 88, 0.26, 0.35], [3.75, 100, 0.22, -0.3]]) at(t, (w, b) => e.drum(b, w, { gain: gn, f, decay: 0.6, pan: p, seed: seedOf(t, 2) }));
		at(4.0, (w, b) => {
			e.taikos(b, w, { gain: 0.5, f: 52, decay: 1.2 });
			e.braam(b, w, C1('C'), { gain: 0.27, dur: 1.9, drive: 1.35, open: [180, 2000, 600] });
			e.subNote(b, w, hz('C2'), 1.8, { gain: 0.07, a: 0.02, r: 0.1 });
			e.brass(b, w, C1('Chorn'), 0.32, { gain: 0.242, bright: 0.7, r: 0.25, seed: 4 });
		}, 2);
		at(5.0, (w, b) => e.brass(b, w, C1('Chorn'), 0.32, { gain: 0.286, bright: 0.9, r: 0.25, seed: 5 }));
		at(4.0, (w, b) => e.strings(b, w, C1('Cstr'), 1.86, { gain: 0.112, a: 0.08, r: 0.04, trem: 0.7, cutoff: 3600, swell: [[0, 0.5], [1.86, 1.25]] }), 1.9);
		at(4.0, (w, b) => e.roll(b, w, w + 1.9, { voice: 'drum', f: 78, gain: 0.3, from: 0.35, to: 1, pitch: [1, 1.3], verb: 0.25 }));
		at(5.0, (w, b) => e.roll(b, w, w + 0.9, { voice: 'snare', gain: 0.17, from: 0.25, to: 1, grid: [0.125, 0.0625, 0.03125] }));
		at(4.4, (w, b) => e.swell(b, w + 1.5, 1.5, { gain: 0.1 }));
		at(4.0, (w, b) => e.shepard(b, w, 1.9, { gain: 0.045 }));
		// the pulse: eighths on D, then sixteenths through B-flat and C, out a beat before the hit
		at(0.5, (w, b) => {
			const steps = [];
			for (let t = 0.5; t < 5.5 - 1e-6; t += t < 2 ? 0.25 : 0.125) {
				const i = Math.round(t / 0.125);
				const pat = t < 2 ? ['D2', 'D3'] : t < 4 ? ['Bb1', 'Bb2', 'F2', 'Bb2'] : t < 5 ? ['C2', 'C3', 'G2', 'C3'] : ['C3', 'G3', 'C4', 'G3'];
				steps.push({ when: w + t - 0.5, f: hz(pat[i % pat.length]), gain: (0.05 + 0.07 * (t / 5.5)) * (i % 4 === 0 ? 1.3 : 1), pan: i % 2 ? 0.25 : -0.25 });
			}
			e.spiccato(b, steps, { cutoff: [500, 3200], verb: 0.2 });
		});
	} else {
		at(0, (w, b) => {
			e.taikos(b, w, { gain: 0.42, f: 47, decay: 1.2 });
			e.braam(b, w, C1('Bb'), { gain: 0.25, dur: 1.1, drive: 1.3, open: [170, 1800, 560] });
			e.subNote(b, w, hz('Bb1'), 0.9, { gain: 0.08, a: 0.02, r: 0.1 });
		}, 1);
		at(1.0, (w, b) => {
			e.taikos(b, w, { gain: 0.46, f: 52, decay: 1.0 });
			e.braam(b, w, C1('C'), { gain: 0.26, dur: 0.95, drive: 1.35, open: [180, 2000, 600] });
			e.subNote(b, w, hz('C2'), 0.85, { gain: 0.065, a: 0.02, r: 0.05 });
			e.brass(b, w, C1('Chorn'), 0.3, { gain: 0.264, bright: 0.8, r: 0.2, seed: 6 });
			e.strings(b, w, C1('Cstr'), 0.88, { gain: 0.112, a: 0.06, r: 0.03, trem: 0.7, cutoff: 3600, swell: [[0, 0.6], [0.88, 1.3]] });
		}, 1);
		at(0.25, (w, b) => e.roll(b, w, w + 1.65, { voice: 'drum', f: 78, gain: 0.28, from: 0.3, to: 1, pitch: [1, 1.3], grid: [0.25, 0.125, 0.0625] }));
		at(1.25, (w, b) => e.roll(b, w, w + 0.65, { voice: 'snare', gain: 0.16, from: 0.3, to: 1, grid: [0.0625, 0.03125] }));
		at(0.6, (w, b) => e.swell(b, w + 1.3, 1.3, { gain: 0.1 }));
		at(0, (w, b) => e.shepard(b, w, 1.9, { gain: 0.045 }));
		at(0, (w, b) => {
			const steps = [];
			for (let t = 0; t < 1.5 - 1e-6; t += 0.125) {
				const i = Math.round(t / 0.125);
				const pat = t < 1 ? ['Bb1', 'Bb2', 'F2', 'Bb2'] : ['C3', 'G3', 'C4', 'G3'];
				steps.push({ when: w + t, f: hz(pat[i % 4]), gain: (0.06 + 0.07 * (t / 1.5)) * (i % 4 === 0 ? 1.3 : 1), pan: i % 2 ? 0.25 : -0.25 });
			}
			e.spiccato(b, steps, { cutoff: [700, 3200], verb: 0.2 });
		});
	}

	// --- the hit: D major ------------------------------------------------------------------
	at(H, (w, b) => {
		e.taikos(b, w, { gain: 0.42, f: 48, decay: 1.6 });
		e.impact(b, w, { gain: 0.15, size: 1.2, tone: 44, click: 0.4, verb: 0.2, dest: e.themeBus(b).in });
		e.crash(b, w, { gain: 0.15, decay: 2.4 });
		e.braam(b, w, C1('Dhit'), { gain: 0.3, dur: 2.0, drive: 1.4, open: [220, 2600, 700] });
		e.subNote(b, w, hz('D1'), end - H - 0.05, { gain: 0.11, a: 0.01, r: 0.2 });
		e.choir(b, w, C1('Dchoir'), end - H - 0.1, { gain: 0.091, a: 0.3, r: 0.5 });
	}, end - H);
	// strings move with the hook: D, C over D, D
	[[0, 'Dstr', 0.5], [0.5, 'CDstr', 0.75], [1.25, 'Dstr', end - H - 1.25]].forEach(([dt, k, len]) =>
		at(H + dt, (w, b) => e.strings(b, w, C1(k), len, { gain: 0.136, a: 0.04, r: 0.25, cutoff: 4200, seed: seedOf(dt, 7) }), len)
	);
	// the hook: trumpets at pitch, horns an octave below, a glockenspiel an octave above
	hookNotes(at, H, HOOK.at, (w, b, f, len, i) => {
		e.brass(b, w, [f], len, { gain: 0.33, bright: 1.0, a: 0.03, r: i === 4 ? 1.2 : 0.12, pan: 0.1, verb: 0.4, seed: 70 + i });
		e.brass(b, w, [f / 2], len, { gain: 0.264, bright: 0.45, a: 0.04, r: i === 4 ? 1.2 : 0.14, verb: 0.45, seed: 80 + i });
		e.glock(b, w, f * 2, { gain: 0.056, decay: i === 4 ? 2.2 : 0.9, pan: 0.2 });
	}, { last: end - H - 1.25 + (full ? 0.9 : 0.6) });
	// drums under the hook: big drums on 1 and 3, a fat snare and clap on 2 and 4, a fill
	const groove = [[0, 'T'], [0.25, 't'], [0.5, 'S'], [0.75, 't'], [1.0, 'T'], [1.25, 't'], [1.5, 'S'], [1.625, 't'], [1.75, 't'], [1.875, 't']];
	for (const [dt, k] of groove) {
		if (H + dt >= end - 1e-6 || (dt === 0 && k === 'T')) continue;
		at(H + dt, (w, b) => {
			if (k === 'T') e.taikos(b, w, { gain: 0.4, f: 50, decay: 1.0 });
			else if (k === 'S') {
				e.snare(b, w, { gain: 0.528, tone: 175, verb: 0.35, seed: seedOf(dt, 3) });
				e.clap(b, w, { gain: 0.396, verb: 0.3, seed: seedOf(dt, 4) });
			} else e.drum(b, w, { gain: 0.2, f: 90 + dt * 20, decay: 0.45, pan: dt % 0.5 ? 0.3 : -0.3, seed: seedOf(dt, 5) });
		});
	}

	// --- the final chord: D add9 -------------------------------------------------------------
	const tail = g.D - end;
	at(end, (w, b) => {
		e.taikos(b, w, { gain: full ? 0.45 : 0.4, f: 48, decay: 1.8 });
		e.crash(b, w, { gain: 0.13, decay: 3 });
		e.braam(b, w, C1('Dhit'), { gain: 0.22, dur: tail + 0.6, drive: 1.25, open: [200, 2000, 500] });
		e.strings(b, w, C1('Dfinal'), tail * 0.55, { gain: 0.16, a: 0.05, r: tail * 0.7, cutoff: 4400, seed: 91 });
		e.brass(b, w, C1('Dbrass'), tail * 0.35, { gain: 0.264, bright: 0.6, a: 0.05, r: tail * 0.6, seed: 92 });
		e.choir(b, w, C1('Dchoir2'), tail * 0.5, { gain: 0.0845, a: 0.15, r: tail * 0.6 });
		e.glock(b, w, hz('F#6'), { gain: 0.064, decay: 2.6, pan: 0.25 });
		e.glock(b, w + 0.12, hz('A6'), { gain: 0.048, decay: 2.4, pan: -0.25 });
		e.subNote(b, w, hz('D1'), tail * 0.5, { gain: 0.12, a: 0.01, r: tail * 0.6 });
	}, tail);
	return list;
}

// =========================================================================================
// Theme 2: electronic anthem. vi IV I V under a pumping supersaw, a snare-roll build, and the
// hook dropping on I V vi IV.
// =========================================================================================

function theme2(e, g) {
	const { list, at } = ev();
	const H = g.hit;
	const full = g.cut === 'full';
	const end = g.final;
	const saw = (t, k, len, cut, gain, opts = {}) =>
		at(t, (w, b) => e.supersaw(b, w, C2(k), len, { gain, cut, a: 0.01, r: opts.r ?? 0.12, d: 0.4, s: 0.9, dest: e.themeBus(b).pump, seed: seedOf(t, 2), ...opts }), len);
	const bassLine = (t, f, len, gain = 0.22) => at(t, (w, b) => e.bass(b, w, f, len, { gain, dest: e.themeBus(b).pump, seed: seedOf(t, 9) }), len);

	// the pump follows the kick; in the build it breathes on quarters with no kick at all
	at(0, (w, b) => {
		const ks = [];
		const span = full ? [[0, 5.0], [H, end + 0.01]] : [[H, end + 0.01]];
		for (const [a, z] of span) for (let t = a; t < z; t += 0.5) ks.push(w + t);
		if (!full) ks.push(w + end);
		e.pumpAt(b, ks.sort((x, y) => x - y), { depth: 0.6, recover: 0.2 });
	}, g.D);

	if (full) {
		// bar 1-2: vi IV I V, filter opening; bar 3: IV V and the build
		const prog = [[0, 'Bm', 1], [1, 'G', 1], [2, 'D', 1], [3, 'A', 1], [4, 'G', 1], [5, 'Aup', 0.88]];
		prog.forEach(([t, k, len], i) => {
			const c0 = 420 + i * 380;
			saw(t, k, len, [[0, c0], [len, c0 * 1.45]], 0.18 + i * 0.012);
		});
		// an offbeat bass between the kicks from bar 2
		[[2, 'D2'], [3, 'A1'], [4, 'G1']].forEach(([t, n]) => {
			for (const dt of [0.25, 0.75]) bassLine(t + dt, hz(n), 0.2, 0.19);
		});
		// a sixteenth-note arp through the whole build
		at(0, (w, b) => {
			const chords = { 0: 'Bm', 1: 'G', 2: 'D', 3: 'A', 4: 'G', 5: 'Aup' };
			for (let t = 0; t < 5.875 - 1e-6; t += 0.125) {
				const ch = C2(chords[Math.floor(t)]);
				const i = Math.round(t / 0.125);
				const f = ch[[0, 1, 2, 1][i % 4]] * (i % 8 >= 4 ? 2 : 1);
				e.pluck(b, w + t, f, { gain: 0.0875 + 0.025 * (t / 6), bright: 0.5 + t / 8, pan: i % 2 ? 0.35 : -0.35, decay: 0.22, delay: 0.12, dest: e.themeBus(b).pump, seed: i });
			}
		});
		for (let t = 2; t < 5 - 1e-6; t += 0.5) at(t, (w, b) => e.kick(b, w, { gain: 0.496, seed: seedOf(t) }));
		for (const t of [2.5, 3.5, 4.5]) at(t, (w, b) => e.clap(b, w, { gain: 0.484, seed: seedOf(t, 1) }));
		for (let t = 2.25; t < 5 - 1e-6; t += 0.5) at(t, (w, b) => e.hat(b, w, { gain: 0.05, open: true, seed: seedOf(t, 4) }));
		at(5.0, (w, b) => e.roll(b, w, w + 0.875, { voice: 'snare', gain: 0.3, from: 0.3, to: 1, pitch: [1, 1.6], grid: [0.125, 0.0625, 0.03125] }));
		at(4.0, (w, b) => e.roll(b, w, w + 1.0, { voice: 'snare', gain: 0.18, from: 0.2, to: 0.45, pitch: [0.9, 1], grid: [0.25, 0.125] }));
		at(3.9, (w, b) => e.riser(b, w, { dur: 1.97, gain: 0.07, tone: 0.35, lo: 300, hi: 9000, root: hz('A2'), verb: 0.3 }));
		at(4.9, (w, b) => e.swell(b, w + 0.97, 0.97, { gain: 0.09 }));
		at(2.0, (w, b) => e.crash(b, w, { gain: 0.08, decay: 1.6 }));
	} else {
		saw(0, 'G', 1, [[0, 900], [1, 1800]], 0.19);
		saw(1, 'Aup', 0.88, [[0, 1800], [0.88, 4200]], 0.2);
		at(0, (w, b) => {
			for (let t = 0; t < 1.875 - 1e-6; t += 0.125) {
				const ch = C2(t < 1 ? 'G' : 'Aup');
				const i = Math.round(t / 0.125);
				e.pluck(b, w + t, ch[[0, 1, 2, 1][i % 4]] * (i % 8 >= 4 ? 2 : 1), { gain: 0.1125 + 0.02 * t, bright: 0.7 + t / 3, pan: i % 2 ? 0.35 : -0.35, decay: 0.2, delay: 0.1, seed: i });
			}
		});
		at(0, (w, b) => e.roll(b, w, w + 1.875, { voice: 'snare', gain: 0.3, from: 0.2, to: 1, pitch: [1, 1.6], grid: [0.25, 0.125, 0.0625, 0.03125] }));
		at(0, (w, b) => e.riser(b, w, { dur: 1.97, gain: 0.07, tone: 0.35, lo: 300, hi: 9000, root: hz('A2'), verb: 0.3 }));
		at(1.0, (w, b) => e.swell(b, w + 0.97, 0.97, { gain: 0.09 }));
	}

	// --- the drop: I V vi IV, the hook on the lead ----------------------------------------------
	const drop = full
		? [[0, 'D', 0.5, 'D2'], [0.5, 'A', 0.5, 'A1'], [1.0, 'Bm', 0.5, 'B1'], [1.5, 'G', 0.5, 'G1']]
		: [[0, 'D', 0.5, 'D2'], [0.5, 'A', 0.75, 'A1']];
	for (const [dt, k, len, bn] of drop) {
		saw(H + dt, k, len, [[0, 7000], [len, 4800]], 0.23, { r: 0.06 });
		saw(H + dt, k, len, [[0, 2600], [len, 1800]], 0.1, { detune: 12, hp: 120 });
		for (let s = 0; s < len / 0.25 - 1e-6; s++) bassLine(H + dt + s * 0.25, hz(bn), 0.21, s % 2 ? 0.19 : 0.23);
	}
	for (let t = H; t < end - 1e-6; t += 0.5) at(t, (w, b) => e.kick(b, w, { gain: 0.576, seed: seedOf(t) }));
	for (let t = H + 0.5; t < end - 1e-6; t += 1) at(t, (w, b) => e.clap(b, w, { gain: 0.572, seed: seedOf(t, 1) }));
	for (let t = H; t < end - 1e-6; t += 0.125) at(t, (w, b) => e.hat(b, w, { gain: (Math.round(t * 8) % 2 ? 0.045 : 0.03), open: Math.round(t * 8) % 4 === 2, seed: seedOf(t, 4), pan: 0.25 }));
	at(H, (w, b) => {
		e.crash(b, w, { gain: 0.14, decay: 2.2 });
		e.subDrop(b, w, { gain: 0.16, from: 90, to: 36, dur: 0.9, dest: e.themeBus(b).in });
	});
	hookNotes(at, H, HOOK.at, (w, b, f, len, i) => {
		e.lead(b, w, f, len, { gain: 0.33, glide: i ? HOOKF[i - 1] : null, r: i === 4 ? 0.8 : 0.08, seed: 30 + i });
		e.pluck(b, w, f * 2, { gain: 0.1, decay: 0.5, bright: 0.8, pan: 0.3, seed: 40 + i });
	}, { last: end - H - 1.25 + (full ? 0.9 : 0.5) });

	// --- the final chord ------------------------------------------------------------------------
	const tail = g.D - end;
	at(end, (w, b) => {
		e.kick(b, w, { gain: 0.576, decay: 0.5, seed: 77 });
		e.crash(b, w, { gain: 0.14, decay: 3 });
		e.supersaw(b, w, C2('Dfinal'), tail * 0.45, { gain: 0.24, cut: [[0, 7500], [tail, 1200]], a: 0.005, d: 0.6, s: 0.75, r: tail * 0.6, dest: e.themeBus(b).pump, seed: 99 });
		e.bass(b, w, hz('D2'), 0.9, { gain: 0.18, dest: e.themeBus(b).pump, seed: 5 });
		e.subNote(b, w, hz('D1'), tail * 0.4, { gain: 0.132, a: 0.005, r: tail * 0.5 });
		e.glock(b, w, hz('F#6'), { gain: 0.06, decay: 2.2 });
	}, tail);
	return list;
}

// =========================================================================================
// Theme 3: warm orchestral-electronic. Felt keys and a soft pulse; I, vi, IV, ii, V; the hook
// on horns and celesta over a plagal turn (IV over the tonic), strings and choir to finish.
// =========================================================================================

function theme3(e, g) {
	const { list, at } = ev();
	const H = g.hit;
	const full = g.cut === 'full';
	const end = g.final;
	const keysArp = (t0, t1, chordAt, step, gain, bright) =>
		at(t0, (w, b) => {
			for (let t = t0; t < t1 - 1e-6; t += step) {
				const ch = chordAt(t);
				const i = Math.round(t / step);
				e.keys(b, w + t - t0, ch[i % ch.length], { gain: gain(t), bright, decay: 1.3, pan: (i % 4) / 3 - 0.5, seed: i + 300 });
			}
		});
	const strings = (t, k, len, o = {}) => at(t, (w, b) => e.strings(b, w, C3(k), len, { gain: 0.12, a: 0.35, r: 0.5, cutoff: 3600, seed: seedOf(t, 3), ...o }), len);

	if (full) {
		at(0, (w, b) => {
			e.warmPad(b, w, C3('Dadd9'), 1.9, { gain: 0.05, a: 0.6, r: 0.6, cutoff: [700, 1500] });
			e.subNote(b, w, hz('D2'), 1.9, { gain: 0.078, a: 0.4, r: 0.3 });
			e.timpani(b, w, hz('D2'), { gain: 0.22, decay: 2.2 });
		}, 2);
		const chordAt = (t) => C3(t < 2 ? 'arpD' : t < 3 ? 'Bm7' : t < 4 ? 'G' : t < 5 ? 'Em7' : t < 5.5 ? 'Asus' : 'A').map((f) => (f < 200 ? f * 2 : f)).sort((x, y) => x - y);
		keysArp(0.5, 4, chordAt, 0.25, (t) => 0.078 + 0.013 * t, 0.55);
		keysArp(4, 5.875, chordAt, 0.125, (t) => 0.065 + 0.026 * (t - 4), 0.65);
		strings(2, 'Bm7', 1.0, { a: 0.5 });
		strings(3, 'G', 1.0, { a: 0.25 });
		strings(4, 'Em7', 1.0, { a: 0.2, gain: 0.135 });
		strings(5, 'Asus', 0.5, { a: 0.1, gain: 0.15 });
		strings(5.5, 'A', 0.38, { a: 0.05, gain: 0.165, r: 0.05, swell: [[0, 1], [0.38, 1.3]] });
		[[2, 'B1'], [3, 'G1'], [4, 'E2'], [5, 'A1']].forEach(([t, n]) => at(t, (w, b) => e.subNote(b, w, hz(n), 0.9, { gain: 0.09, a: 0.03, r: 0.1 }), 1));
		// a heartbeat that becomes a pulse
		for (const t of [2, 3, 4, 4.5, 5, 5.5]) at(t, (w, b) => e.kick(b, w, { gain: 0.336, tone: 48, decay: 0.45, click: 0.1, seed: seedOf(t) }));
		for (let t = 2; t < 5.875 - 1e-6; t += 0.25) at(t + 0.125, (w, b) => e.hat(b, w, { gain: 0.022 + 0.01 * (t - 2) / 4, seed: seedOf(t, 6), pan: -0.3 }));
		at(5.0, (w, b) => e.roll(b, w, w + 0.9, { voice: 'timpani', f: hz('A1'), gain: 0.16, from: 0.25, to: 1, pitch: [1, 1], grid: [0.0625, 0.0625, 0.03125] }));
		at(4.6, (w, b) => e.swell(b, w + 1.3, 1.3, { gain: 0.07 }));
		at(5.5, (w, b) => e.gliss(b, w, w + 0.45, C3('gliss'), { gain: 0.0585, decay: 1.2 }));
	} else {
		at(0, (w, b) => {
			e.warmPad(b, w, C3('G'), 0.95, { gain: 0.05, a: 0.2, r: 0.3, cutoff: [900, 1600] });
			e.subNote(b, w, hz('G1'), 0.9, { gain: 0.084, a: 0.05, r: 0.1 });
			e.timpani(b, w, hz('D2'), { gain: 0.22, decay: 1.5 });
		}, 1);
		strings(0, 'G', 1.0, { a: 0.15 });
		strings(1.0, 'Asus', 0.5, { a: 0.08, gain: 0.15 });
		strings(1.5, 'A', 0.38, { a: 0.05, gain: 0.165, r: 0.05, swell: [[0, 1], [0.38, 1.3]] });
		keysArp(0, 1.875, (t) => C3(t < 1 ? 'G' : t < 1.5 ? 'Asus' : 'A').map((f) => (f < 200 ? f * 2 : f)).sort((x, y) => x - y), 0.125, (t) => 0.065 + 0.026 * t, 0.65);
		at(1.0, (w, b) => e.subNote(b, w, hz('A1'), 0.88, { gain: 0.084, a: 0.03, r: 0.05 }), 1);
		for (const t of [0, 0.5, 1, 1.5]) at(t, (w, b) => e.kick(b, w, { gain: 0.336, tone: 48, decay: 0.45, click: 0.1, seed: seedOf(t) }));
		at(1.0, (w, b) => e.roll(b, w, w + 0.9, { voice: 'timpani', f: hz('A1'), gain: 0.16, from: 0.25, to: 1, pitch: [1, 1], grid: [0.0625, 0.03125] }));
		at(0.6, (w, b) => e.swell(b, w + 1.3, 1.3, { gain: 0.07 }));
		at(1.5, (w, b) => e.gliss(b, w, w + 0.45, C3('gliss'), { gain: 0.0585, decay: 1.2 }));
	}

	// --- the hit ---------------------------------------------------------------------------------
	at(H, (w, b) => {
		e.timpani(b, w, hz('D2'), { gain: 0.34, decay: 2.4 });
		e.drum(b, w, { gain: 0.2, f: 44, decay: 1.6, slap: 0.15, verb: 0.4, seed: 5 });
		e.crash(b, w, { gain: 0.1, decay: 2.6 });
		e.subNote(b, w, hz('D1'), end - H - 0.05, { gain: 0.12, a: 0.01, r: 0.2 });
		e.choir(b, w, C3('Dchoir'), end - H - 0.1, { gain: 0.06, a: 0.4, r: 0.5 });
		e.warmPad(b, w, C3('Dadd9'), end - H, { gain: 0.04, a: 0.3, r: 0.6, cutoff: [1200, 1800] });
	}, end - H);
	[[0, 'Dhit', 0.5], [0.5, 'GD', 0.75], [1.25, 'Dhit', end - H - 1.25]].forEach(([dt, k, len]) => strings(H + dt, k, len, { a: 0.06, r: 0.3, gain: 0.13, cutoff: 4000 }));
	hookNotes(at, H, HOOK.at, (w, b, f, len, i) => {
		e.brass(b, w, [f / 2, f], len, { gain: 0.3, bright: 0.55, a: 0.05, r: i === 4 ? 1.3 : 0.16, pan: 0.15, verb: 0.5, seed: 110 + i });
		e.glock(b, w, f * 2, { gain: 0.072, decay: i === 4 ? 2.4 : 1.1, pan: -0.15 });
		e.keys(b, w, f, { gain: 0.065, decay: 1.2, bright: 0.7, seed: 120 + i });
	}, { last: end - H - 1.25 + (full ? 0.9 : 0.6) });
	// the pulse under the hook: soft kick on the beat, sixteenth keys pumping gently
	for (let t = H + 0.5; t < end - 1e-6; t += 0.5) at(t, (w, b) => e.kick(b, w, { gain: 0.336, tone: 48, decay: 0.45, click: 0.1, seed: seedOf(t) }));
	at(H, (w, b) => {
		const ch = { 0: C3('Dhit'), 1: C3('GD') };
		for (let t = 0; t < end - H - 1e-6; t += 0.125) {
			const i = Math.round(t / 0.125);
			const c = t >= 0.5 && t < 1.25 ? ch[1] : ch[0];
			e.pluck(b, w + t, c[2 + (i % 3)], { gain: 0.075, decay: 0.18, bright: 0.45, pan: i % 2 ? 0.4 : -0.4, delay: 0.1, seed: 200 + i });
		}
	});
	if (full) at(end - 0.42, (w, b) => e.gliss(b, w, w + 0.4, C3('gliss'), { gain: 0.052, decay: 1.6 }));

	// --- the final chord ----------------------------------------------------------------------------
	const tail = g.D - end;
	at(end, (w, b) => {
		e.timpani(b, w, hz('D2'), { gain: 0.36, decay: 2.6 });
		e.crash(b, w, { gain: 0.09, decay: 3 });
		e.strings(b, w, C3('Dfinal'), tail * 0.5, { gain: 0.15, a: 0.08, r: tail * 0.7, cutoff: 4000, seed: 131 });
		e.brass(b, w, C3('horns'), tail * 0.35, { gain: 0.2, bright: 0.45, a: 0.08, r: tail * 0.6, seed: 132 });
		e.choir(b, w, C3('Dchoir'), tail * 0.5, { gain: 0.055, a: 0.2, r: tail * 0.6 });
		e.warmPad(b, w, C3('Dadd9'), tail * 0.4, { gain: 0.04, a: 0.1, r: tail * 0.7, cutoff: [1600, 900] });
		e.glock(b, w, hz('F#6'), { gain: 0.072, decay: 2.8, pan: 0.2 });
		e.subNote(b, w, hz('D1'), tail * 0.4, { gain: 0.12, a: 0.01, r: tail * 0.6 });
		[0, 0.25, 0.5, 0.75, 1.0].forEach((dt, i) => e.keys(b, w + dt * 1.1, C3('arpD').concat(hz('A5'))[i], { gain: 0.065 - i * 0.006, decay: 2, bright: 0.5, pan: (i % 3) * 0.3 - 0.3, seed: 140 + i }));
	}, tail);
	return list;
}

const SCORES = { 1: theme1, 2: theme2, 3: theme3 };

export const THEME_NAMES = {
	1: { en: 'Hybrid epic', cs: 'Epický hybrid' },
	2: { en: 'Electronic anthem', cs: 'Elektronická hymna' },
	3: { en: 'Warm orchestral', cs: 'Hřejivý orchestr' }
};

// per-arrangement mix: output trim (linear), the air shelf and the 300 Hz dip (dB)
const MIX = {
	1: { trim: 0.7, air: 2.5, mud: -2.5 },
	2: { trim: 0.72, air: 1.5, mud: -2 },
	3: { trim: 0.8, air: 2.5, mud: -3 }
};
function mixBus(e, theme, b) {
	const tb = e.themeBus(b);
	const m = MIX[theme] || MIX[1];
	tb.out.gain.value = m.trim;
	tb.air.value = m.air;
	tb.mud.value = m.mud;
	return tb;
}

/** The theme as timeline events ({at, dur, fn(when, bus, skip)}) for one cut of the grid. */
export function themeScore(e, theme, grid) {
	const list = (SCORES[theme] || theme1)(e, grid);
	// the bus settings come first (and apply whenever playback starts inside the piece)
	list.unshift({ at: 0, dur: grid.D, fn: (w, b) => mixBus(e, theme, b) });
	return list;
}

/**
 * The promo tag: the hook condensed over the theme's own cadence, starting at `when` (the
 * first note). Its lead-in (a swell) starts 0.35 s earlier.
 */
export function themeTag(e, theme, b, when, { gain = 0.5 } = {}) {
	const k = gain / 0.5;
	mixBus(e, theme, b);
	const R = HOOK.tagAt;
	const n5 = when + R[4];
	const tail = 1.6;
	e.swell(b, when, 0.35, { gain: 0.05 * k });
	if (theme === 2) {
		const ks = [when, when + R[2], when + R[3], n5];
		e.pumpAt(b, ks, { depth: 0.55, recover: 0.18 });
		ks.forEach((t, i) => e.kick(b, t, { gain: 0.6 * k, seed: 300 + i }));
		e.supersaw(b, when, C2('D'), R[2] - 0.02, { gain: 0.15 * k, cut: [[0, 6000]], r: 0.06, dest: e.themeBus(b).pump, seed: 301 });
		e.supersaw(b, when + R[2], C2('A'), R[4] - R[2] - 0.02, { gain: 0.15 * k, cut: [[0, 6000]], r: 0.06, dest: e.themeBus(b).pump, seed: 302 });
		e.supersaw(b, n5, C2('Dfinal'), 0.5, { gain: 0.17 * k, cut: [[0, 7000], [tail, 1200]], r: tail, dest: e.themeBus(b).pump, seed: 303 });
		e.bass(b, n5, hz('D2'), 0.6, { gain: 0.18 * k, seed: 304 });
		e.subNote(b, n5, hz('D1'), 0.5, { gain: 0.2 * k, r: 1.0 });
		e.crash(b, n5, { gain: 0.1 * k, decay: 2 });
		R.forEach((r, i) => e.lead(b, when + r, HOOKF[i], i < 4 ? R[i + 1] - r - 0.02 : 1.0, { gain: 0.14 * k, glide: i ? HOOKF[i - 1] : null, r: i === 4 ? 0.7 : 0.06, seed: 310 + i }));
		return;
	}
	const warm = theme === 3;
	R.forEach((r, i) => {
		const len = i < 4 ? R[i + 1] - r - 0.02 : 1.0;
		if (warm) e.brass(b, when + r, [HOOKF[i] / 2, HOOKF[i]], len, { gain: 0.13 * k, bright: 0.55, a: 0.04, r: i === 4 ? 1.2 : 0.12, verb: 0.5, seed: 320 + i });
		else {
			e.brass(b, when + r, [HOOKF[i]], len, { gain: 0.13 * k, bright: 0.95, a: 0.03, r: i === 4 ? 1.1 : 0.1, verb: 0.4, seed: 330 + i });
			e.brass(b, when + r, [HOOKF[i] / 2], len, { gain: 0.1 * k, bright: 0.45, a: 0.04, r: i === 4 ? 1.1 : 0.12, verb: 0.45, seed: 340 + i });
		}
		e.glock(b, when + r, HOOKF[i] * 2, { gain: 0.035 * k, decay: i === 4 ? 2 : 0.8 });
	});
	const chords = warm ? [['Dhit', 0, R[2]], ['GD', R[2], R[4]]] : [['Dstr', 0, R[2]], ['CDstr', R[2], R[4]]];
	const CC = warm ? C3 : C1;
	for (const [key, a, z] of chords) e.strings(b, when + a, CC(key), z - a - 0.02, { gain: 0.07 * k, a: 0.04, r: 0.12, cutoff: 4000, seed: 350 + a * 10 });
	e.strings(b, n5, CC(warm ? 'Dfinal' : 'Dfinal'), 0.5, { gain: 0.085 * k, a: 0.04, r: tail, cutoff: 4200, seed: 360 });
	e.choir(b, n5, CC(warm ? 'Dchoir' : 'Dchoir2'), 0.5, { gain: 0.05 * k, a: 0.1, r: tail });
	e.subNote(b, n5, hz('D1'), 0.5, { gain: 0.2 * k, r: 1.1 });
	if (warm) {
		e.timpani(b, when, hz('D2'), { gain: 0.22 * k, decay: 1.6 });
		e.timpani(b, n5, hz('D2'), { gain: 0.3 * k, decay: 2.2 });
		e.gliss(b, when + R[3], n5 - 0.02, C3('gliss').slice(3), { gain: 0.035 * k, decay: 1.2 });
	} else {
		e.taikos(b, when, { gain: 0.3 * k, f: 50, decay: 1.1 });
		e.taikos(b, n5, { gain: 0.42 * k, f: 46, decay: 1.6 });
		e.crash(b, n5, { gain: 0.08 * k, decay: 2.2 });
	}
}
