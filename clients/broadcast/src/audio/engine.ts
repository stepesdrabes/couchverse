// The audio graph. Beds are decoded buffers in the graph (never <audio>), so every gain move is
// sample-accurate automation; voices take an absolute context time, so one Timeline drives
// picture and sound, live (AudioContext) or rendered (OfflineAudioContext).
//
// Graph: per-piece buses (music, sfx, voice, reverb send) -> the shared music, sfx and voice
// buses -> glue compressor -> fast limiter -> trim -> master. The voice bus is reserved for a
// later announcer (D26): it bypasses nothing and nothing writes to it yet.

import { rng } from '../core/rng';

export const SAMPLE_RATE = 48000;

/** One playing piece's buses; fading them out is how pieces crossfade. */
export interface Bus {
	music: GainNode;
	sfx: GainNode;
	voice: GainNode;
	verb: GainNode;
	sources: Set<AudioScheduledSourceNode>;
	/** Ramp every bus of the piece to `to` from `at` over `dur` seconds. */
	fade(to: number, at: number, dur: number): void;
	/** Stop now (live playback): a short fade, then the sources stop. */
	stop(fade?: number): void;
}

/** A soundtrack excerpt ready to play: the decoded file and the window the spec allows. */
export interface LoadedBed {
	buffer: AudioBuffer;
	/** Seconds into the file where the excerpt starts. */
	in: number;
	/** Seconds the excerpt lasts. */
	length: number;
	gain: number;
	/** Detected onsets relative to the excerpt's start. */
	onsets: Onset[];
}

export interface Onset {
	t: number;
	/** Strength, 1 = a strong hit (normalized to the 90th percentile). */
	s: number;
}

export interface Duck {
	at: number;
	depth: number;
	attack?: number;
	hold?: number;
	release?: number;
}

export interface BedOptions {
	/** Seconds into the excerpt. */
	from?: number;
	/** Seconds of the excerpt to play (from `from`). */
	end?: number;
	gain?: number;
	fadeIn?: number;
	fadeOut?: number;
	ducks?: Duck[];
	tapeStop?: { at: number; dur: number } | null;
}

export class AudioEngine {
	ctx!: BaseAudioContext;
	muted = false;
	rand: () => number = rng(4242);
	protected buses = new Set<Bus>();
	protected master!: GainNode;
	protected sum!: GainNode;
	musicBus!: GainNode;
	sfxBus!: GainNode;
	voiceBus!: GainNode;
	protected verbSend!: GainNode;
	protected white!: AudioBuffer;
	protected pink!: AudioBuffer;
	protected projectorBuf!: AudioBuffer;

	/** Build the graph on a context (a live one or an offline render). */
	build(ctx: BaseAudioContext, { raw = false } = {}) {
		this.ctx = ctx;
		this.buses = new Set();
		this.rand = rng(4242);
		// gentle glue, then a fast limiter; both apply the spec's automatic makeup gain, which
		// the trim takes back
		const glue = ctx.createDynamicsCompressor();
		glue.threshold.value = -16;
		glue.knee.value = 10;
		glue.ratio.value = 2.2;
		glue.attack.value = 0.02;
		glue.release.value = 0.28;
		const limit = ctx.createDynamicsCompressor();
		limit.threshold.value = -2.5;
		limit.knee.value = 0;
		limit.ratio.value = 20;
		limit.attack.value = 0.002;
		limit.release.value = 0.12;
		const trim = this.gain(0.72);
		this.master = this.gain(this.muted ? 0 : 1);
		this.sum = this.gain(1);
		// raw: dynamics bypassed, to measure the headroom going into them
		if (raw) this.sum.connect(this.master).connect(ctx.destination);
		else
			this.sum
				.connect(glue)
				.connect(limit)
				.connect(trim)
				.connect(this.master)
				.connect(ctx.destination);
		this.musicBus = this.gain(0.92);
		this.sfxBus = this.gain(1);
		this.voiceBus = this.gain(1);
		this.musicBus.connect(this.sum);
		this.sfxBus.connect(this.sum);
		this.voiceBus.connect(this.sum);
		const verb = ctx.createConvolver();
		verb.buffer = makeImpulse(ctx, 3.4, 0.82);
		this.verbSend = this.gain(1);
		const verbReturn = this.gain(0.55);
		// keep mud out of the tail
		const verbHp = this.filter('highpass', 180);
		this.verbSend.connect(verbHp).connect(verb).connect(verbReturn).connect(this.sum);
		this.white = makeNoise(ctx, 'white');
		this.pink = makeNoise(ctx, 'pink');
		this.projectorBuf = makeProjector(ctx);
	}

	/** Per-cue seed, so a cue's noise is the same however playback reaches it. */
	reseed(index: number) {
		this.rand = rng(4242 + index * 7919);
	}

	setMuted(m: boolean) {
		this.muted = m;
		if (!this.master) return;
		const t = this.ctx.currentTime;
		this.master.gain.cancelScheduledValues(t);
		this.master.gain.setTargetAtTime(m ? 0 : 1, t, 0.04);
	}

	setVolume(v: number) {
		if (!this.master || this.muted) return;
		const t = this.ctx.currentTime;
		this.master.gain.cancelScheduledValues(t);
		this.master.gain.setTargetAtTime(v, t, 0.04);
	}

	bus(): Bus {
		const ctx = this.ctx;
		const mk = (dest: AudioNode) => {
			const n = ctx.createGain();
			n.connect(dest);
			return n;
		};
		const gains: GainNode[] = [];
		const b: Bus = {
			music: mk(this.musicBus),
			sfx: mk(this.sfxBus),
			voice: mk(this.voiceBus),
			verb: mk(this.verbSend),
			sources: new Set(),
			fade: (to, at, dur) => {
				for (const n of gains) {
					n.gain.cancelScheduledValues(at);
					n.gain.setValueAtTime(n.gain.value, at);
					n.gain.linearRampToValueAtTime(to, at + dur);
				}
			},
			stop: (fade = 0.12) => {
				const t = ctx.currentTime;
				for (const n of gains) {
					n.gain.cancelScheduledValues(t);
					n.gain.setValueAtTime(n.gain.value, t);
					n.gain.linearRampToValueAtTime(0, t + fade);
				}
				for (const s of b.sources) {
					try {
						s.stop(t + fade + 0.02);
					} catch {
						// never started
					}
				}
				setTimeout(() => gains.forEach((n) => n.disconnect()), (fade + 0.3) * 1000);
				this.buses.delete(b);
			}
		};
		gains.push(b.music, b.sfx, b.voice, b.verb);
		this.buses.add(b);
		return b;
	}

	stopAll() {
		for (const b of [...this.buses]) b.stop();
	}

	// --- building blocks -----------------------------------------------------------------

	track<T extends AudioScheduledSourceNode>(b: Bus, src: T): T {
		b.sources.add(src);
		src.onended = () => b.sources.delete(src);
		return src;
	}

	gain(v = 1): GainNode {
		const n = this.ctx.createGain();
		n.gain.value = v;
		return n;
	}

	filter(type: BiquadFilterType, freq: number, q = 0.707): BiquadFilterNode {
		const f = this.ctx.createBiquadFilter();
		f.type = type;
		f.frequency.value = freq;
		f.Q.value = q;
		return f;
	}

	pan(v = 0): StereoPannerNode {
		const p = this.ctx.createStereoPanner();
		p.pan.value = v;
		return p;
	}

	osc(b: Bus, type: OscillatorType, freq: number, when: number, dur: number): OscillatorNode {
		const o = this.ctx.createOscillator();
		o.type = type;
		o.frequency.value = freq;
		o.start(when);
		o.stop(when + dur);
		return this.track(b, o);
	}

	noise(b: Bus, kind: 'white' | 'pink', when: number, dur: number, rate = 1) {
		const s = this.ctx.createBufferSource();
		s.buffer = kind === 'pink' ? this.pink : this.white;
		s.loop = true;
		s.playbackRate.value = rate;
		s.start(when, this.rand() * 1.5);
		s.stop(when + dur);
		return this.track(b, s);
	}

	send(b: Bus, node: AudioNode, amount: number) {
		if (amount <= 0) return;
		const s = this.gain(amount);
		node.connect(s).connect(b.verb);
	}

	/** An attack/decay envelope on a fresh gain. */
	ad(when: number, peak: number, attack: number, decay: number): GainNode {
		const g = this.gain(0);
		g.gain.setValueAtTime(0, when);
		g.gain.linearRampToValueAtTime(peak, when + attack);
		g.gain.exponentialRampToValueAtTime(Math.max(peak * 0.0005, 1e-5), when + attack + decay);
		return g;
	}

	shaper(drive: number): WaveShaperNode {
		const s = this.ctx.createWaveShaper();
		s.curve = softClip(drive);
		return s;
	}

	// --- beds ------------------------------------------------------------------------------

	/**
	 * Play a bed. Option times are relative to the bed cue. `ducks` dip it under stingers and
	 * the channel tag; `tapeStop` winds it down like a stopped deck.
	 */
	bed(b: Bus, when: number, bed: LoadedBed | null, skip = 0, o: BedOptions = {}) {
		if (!bed) return;
		const buf = bed.buffer;
		const { from = 0, gain = 1, fadeIn = 0.25, fadeOut = 1.2, ducks = [], tapeStop = null } = o;
		const avail = bed.length - from;
		const end = Math.min(o.end ?? avail, avail);
		if (end <= 0 || skip >= end) return;
		const level = gain * bed.gain;
		const src = this.ctx.createBufferSource();
		src.buffer = buf;
		const fade = this.gain(0);
		const duck = this.gain(1);
		src.connect(fade).connect(duck).connect(b.music);
		applyEnv(
			fade.gain,
			[
				[0, 0],
				[fadeIn, level],
				[end - fadeOut, level],
				[end, 0]
			],
			when,
			skip
		);
		const duckPts: [number, number][] = [[0, 1]];
		for (const d of [...ducks].sort((x, y) => x.at - y.at)) {
			const att = d.attack ?? 0.15;
			const rel = d.release ?? 0.7;
			const hold = d.hold ?? 1;
			duckPts.push(
				[d.at - att, 1],
				[d.at, d.depth],
				[d.at + hold, d.depth],
				[d.at + hold + rel, 1]
			);
		}
		applyEnv(duck.gain, duckPts, when, skip);
		if (tapeStop && tapeStop.at > skip) {
			const t0 = when + tapeStop.at - skip;
			src.playbackRate.setValueAtTime(1, t0);
			src.playbackRate.exponentialRampToValueAtTime(0.08, t0 + tapeStop.dur);
			fade.gain.cancelScheduledValues(t0);
			fade.gain.setValueAtTime(level, t0);
			fade.gain.linearRampToValueAtTime(level * 0.8, t0 + tapeStop.dur * 0.6);
			fade.gain.linearRampToValueAtTime(0, t0 + tapeStop.dur);
		}
		src.start(when, bed.in + from + skip);
		src.stop(when + (end - skip) + 0.05);
		this.track(b, src);
	}

	/** Seconds-from-now to context time and the matching document time (ms), aligned to what
	 *  is actually leaving the speakers. */
	clockPair(lead = 0.12): { ctxStart: number; docStart: number } {
		const ctx = this.ctx as AudioContext;
		const ctxStart = ctx.currentTime + lead;
		const ts = ctx.getOutputTimestamp ? ctx.getOutputTimestamp() : null;
		let docStart: number;
		if (ts && ts.performanceTime && ts.performanceTime > 0 && ts.contextTime != null)
			docStart = ts.performanceTime + (ctxStart - ts.contextTime) * 1000;
		else docStart = performance.now() + (lead + (ctx.outputLatency || ctx.baseLatency || 0)) * 1000;
		return { ctxStart, docStart };
	}
}

// --- helpers ---------------------------------------------------------------------------------

/** A piecewise-linear envelope applied from `skip` seconds in (playback may start mid-way). */
export function applyEnv(
	param: AudioParam,
	points: [number, number][],
	when: number,
	skip: number
) {
	const pts = points.filter((p) => isFinite(p[0])).sort((a, b) => a[0] - b[0]);
	const valueAt = (t: number) => {
		if (t <= pts[0][0]) return pts[0][1];
		for (let i = 1; i < pts.length; i++) {
			const [t1, v1] = pts[i];
			const [t0, v0] = pts[i - 1];
			if (t <= t1) return t1 === t0 ? v1 : v0 + ((v1 - v0) * (t - t0)) / (t1 - t0);
		}
		return pts[pts.length - 1][1];
	};
	param.cancelScheduledValues(when);
	param.setValueAtTime(valueAt(skip), when);
	for (const [t, v] of pts) if (t > skip) param.linearRampToValueAtTime(v, when + (t - skip));
}

/** A bell-shaped gain curve peaking at `peak` (0..1 of its length). */
export function bellCurve(peak: number, amp: number, n = 64): Float32Array<ArrayBuffer> {
	const c = new Float32Array(n);
	for (let i = 0; i < n; i++) {
		const x = i / (n - 1);
		const y = x < peak ? Math.pow(x / peak, 2) : Math.pow((1 - x) / (1 - peak), 1.6);
		c[i] = y * amp;
	}
	c[n - 1] = 0;
	return c;
}

export function softClip(drive: number): Float32Array<ArrayBuffer> {
	const n = 1024;
	const c = new Float32Array(n);
	for (let i = 0; i < n; i++) {
		const x = (i / (n - 1)) * 2 - 1;
		c[i] = Math.tanh(x * drive) / Math.tanh(drive);
	}
	return c;
}

function makeNoise(ctx: BaseAudioContext, kind: 'white' | 'pink'): AudioBuffer {
	const len = ctx.sampleRate * 2;
	const buf = ctx.createBuffer(2, len, ctx.sampleRate);
	const r = rng(kind === 'pink' ? 11 : 5);
	for (let ch = 0; ch < 2; ch++) {
		const d = buf.getChannelData(ch);
		let b0 = 0,
			b1 = 0,
			b2 = 0,
			b3 = 0,
			b4 = 0,
			b5 = 0,
			b6 = 0;
		for (let i = 0; i < len; i++) {
			const w = r() * 2 - 1;
			if (kind === 'pink') {
				// Paul Kellet's refined pink filter
				b0 = 0.99886 * b0 + w * 0.0555179;
				b1 = 0.99332 * b1 + w * 0.0750759;
				b2 = 0.969 * b2 + w * 0.153852;
				b3 = 0.8665 * b3 + w * 0.3104856;
				b4 = 0.55 * b4 + w * 0.5329522;
				b5 = -0.7616 * b5 - w * 0.016898;
				d[i] = (b0 + b1 + b2 + b3 + b4 + b5 + b6 + w * 0.5362) * 0.11;
				b6 = w * 0.115926;
			} else d[i] = w * 0.5;
		}
	}
	return buf;
}

/** A generated room: decorrelated stereo noise, exponential decay, darkening over time. */
function makeImpulse(ctx: BaseAudioContext, seconds: number, decay: number): AudioBuffer {
	const sr = ctx.sampleRate;
	const len = Math.floor(sr * seconds);
	const pre = Math.floor(sr * 0.018);
	const buf = ctx.createBuffer(2, len, sr);
	const r = rng(23);
	for (let ch = 0; ch < 2; ch++) {
		const d = buf.getChannelData(ch);
		let lp = 0;
		for (let i = pre; i < len; i++) {
			const t = (i - pre) / sr;
			const k = 0.92 - 0.82 * Math.min(1, t / seconds);
			lp += k * (r() * 2 - 1 - lp);
			d[i] = lp * Math.exp(-t / decay) * 0.6;
		}
		for (const [ms, g] of [
			[11, 0.5],
			[19, 0.35],
			[31, 0.28],
			[43, 0.2]
		]) {
			const i = pre + Math.floor((ms + ch * 3) * sr * 0.001);
			if (i < len) d[i] += g * (ch ? -1 : 1);
		}
	}
	return buf;
}

function makeProjector(ctx: BaseAudioContext): AudioBuffer {
	const sr = ctx.sampleRate;
	const len = sr; // one second loops cleanly at 24 frames
	const buf = ctx.createBuffer(2, len, sr);
	const r = rng(41);
	for (let ch = 0; ch < 2; ch++) {
		const d = buf.getChannelData(ch);
		for (let i = 0; i < len; i++) d[i] = (r() * 2 - 1) * 0.06;
		for (let f = 0; f < 24; f++) {
			const at = Math.floor((f / 24) * len + r() * 60);
			const amp = 0.5 + r() * 0.5;
			for (let j = 0; j < sr * 0.006; j++) {
				const i = at + j;
				if (i < len) d[i] += (r() * 2 - 1) * amp * Math.exp(-j / (sr * 0.0012));
			}
		}
	}
	return buf;
}
