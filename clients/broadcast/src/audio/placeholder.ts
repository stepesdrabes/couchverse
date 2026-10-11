// Built-in placeholder beds (`synth:groove`, `synth:pulse`, `synth:drone`): rendered offline
// from the kit, so the admin preview and the tests have a bed with real beats before any
// soundtrack is analysed. Deterministic like everything else.

import { SAMPLE_RATE } from './engine';
import { Kit } from './kit';

export const PLACEHOLDER_BEDS = ['synth:groove', 'synth:pulse', 'synth:drone'] as const;

const D2 = 73.42;
const Bb1 = 58.27;
const C2 = 65.41;
const A1 = 55.0;

export async function placeholderBed(url: string): Promise<AudioBuffer> {
	const kind = url.slice('synth:'.length);
	const seconds = 32;
	const ctx = new OfflineAudioContext(2, seconds * SAMPLE_RATE, SAMPLE_RATE);
	const kit = new Kit();
	kit.build(ctx);
	const b = kit.bus();
	if (kind === 'drone') {
		kit.ambient(b, 0, seconds, 0, { gain: 0.3 });
		for (let bar = 0; bar < 8; bar++) kit.bell(b, bar * 4 + 0.5, 587.33, { gain: 0.08, decay: 3 });
	} else {
		const bpm = kind === 'pulse' ? 100 : 120;
		const beat = 60 / bpm;
		const roots = kind === 'pulse' ? [D2, D2, Bb1, A1] : [D2, Bb1, C2, A1];
		for (let i = 0; i * beat < seconds - 0.5; i++) {
			const t = i * beat;
			const bar = Math.floor(i / 4);
			kit.reseed(i);
			kit.impact(b, t, {
				gain: i % 16 === 0 ? 0.7 : 0.42,
				size: 0.4,
				tone: 50,
				click: 0.5,
				verb: 0.05
			});
			if (i % 4 === 1 || i % 4 === 3) kit.punch(b, t, { gain: kind === 'pulse' ? 0.28 : 0.34 });
			for (const half of [0, 0.5])
				kit.tick(b, t + half * beat, { freq: 7000, gain: 0.018, decay: 0.03, pan: 0.3 });
			const root = roots[bar % roots.length];
			const bass = kit.osc(b, 'sawtooth', root, t, beat * 0.9);
			const lp = kit.filter('lowpass', 420, 2);
			bass
				.connect(lp)
				.connect(kit.ad(t, 0.16, 0.005, beat * 0.8))
				.connect(b.music);
			if (i % 8 === 0)
				kit.pad(b, t, [root * 2, root * 3, root * 4, root * 5], {
					gain: 0.04,
					attack: 0.3,
					hold: beat * 6,
					release: 1.5
				});
		}
	}
	return ctx.startRendering();
}
