// Live playback of one clip in a browser (the admin preview): the same timeline the renderer
// seeks, played against the audio clock so picture and sound share one zero.

import type { Bus } from './audio/engine';
import { Kit } from './audio/kit';
import type { Cue } from './core/timeline';
import { FPS } from './core/timeline';
import { buildPiece, type Piece } from './piece';
import type { ClipSpec } from './spec';
import { Stage } from './stage';

export type PlayerState = 'empty' | 'loading' | 'paused' | 'playing';

export interface PlayerSnapshot {
	state: PlayerState;
	time: number;
	duration: number;
	cue: Cue | null;
}

export class Player {
	readonly stage: Stage;
	private readonly kit = new Kit();
	private ctx: AudioContext | null = null;
	private piece: Piece | null = null;
	private bus: Bus | null = null;
	private state: PlayerState = 'empty';
	private frozenAt = 0;
	private raf = 0;
	private generation = 0;
	private muted = false;
	private volume = 1;
	/** Called on every frame while playing and on every change. */
	onUpdate: ((s: PlayerSnapshot) => void) | null = null;

	constructor(host: HTMLElement) {
		this.stage = new Stage(host, { fit: true });
	}

	get duration() {
		return this.piece?.duration ?? 0;
	}

	get cues(): Cue[] {
		return this.piece?.cues ?? [];
	}

	get time(): number {
		if (!this.piece) return 0;
		if (this.state !== 'playing') return this.frozenAt;
		const t = this.piece.tl.time();
		return t == null ? this.frozenAt : t - this.piece.start;
	}

	snapshot(): PlayerSnapshot {
		const t = this.time;
		const cue = this.piece
			? ([...this.piece.cues].reverse().find((c) => c.at <= t + 1e-3) ?? null)
			: null;
		return { state: this.state, time: t, duration: this.duration, cue };
	}

	/** Load a clip (replacing the current one) and show its first frame, or `at`. */
	async load(spec: ClipSpec, at = 0): Promise<void> {
		const gen = ++this.generation;
		this.stop();
		this.state = 'loading';
		this.emit();
		const piece = await buildPiece(this.stage, spec, this.kit);
		if (gen !== this.generation) {
			piece.dispose();
			return;
		}
		this.piece?.dispose();
		this.piece = piece;
		this.state = 'paused';
		await this.seek(Math.min(at, piece.duration));
	}

	/** Freeze on the frame at t. */
	async seek(t: number) {
		if (!this.piece) return;
		this.stop();
		this.frozenAt = Math.max(0, Math.min(this.piece.duration, t));
		this.state = 'paused';
		await this.piece.seek(this.frozenAt);
		this.emit();
	}

	/** Step whole frames while frozen. */
	step(frames: number) {
		return this.seek(Math.round((this.frozenAt + frames / FPS) * FPS) / FPS);
	}

	async play() {
		const piece = this.piece;
		if (!piece) return;
		if (this.frozenAt >= piece.duration - 0.05) this.frozenAt = 0;
		const ctx = await this.audio();
		this.stop();
		const lead = 0.12;
		const { ctxStart, docStart } = ctx
			? this.kit.clockPair(lead)
			: { ctxStart: 0, docStart: Number(document.timeline.currentTime ?? 0) + lead * 1000 };
		const offset = piece.start + this.frozenAt;
		piece.tl.play(docStart, offset);
		if (ctx) {
			this.bus = this.kit.bus();
			piece.tl.scheduleSounds(ctxStart, offset, this.bus, (i) => this.kit.reseed(i));
		}
		this.state = 'playing';
		this.emit();
		this.raf = requestAnimationFrame(this.frame);
	}

	pause() {
		if (this.state !== 'playing') return;
		const t = this.time;
		void this.seek(t);
	}

	setMuted(m: boolean) {
		this.muted = m;
		if (this.ctx) this.kit.setMuted(m);
	}

	setVolume(v: number) {
		this.volume = v;
		if (this.ctx) this.kit.setVolume(v);
	}

	destroy() {
		this.generation++;
		this.stop();
		this.piece?.dispose();
		this.piece = null;
		this.stage.destroy();
		void this.ctx?.close();
		this.ctx = null;
	}

	private frame = () => {
		const piece = this.piece;
		if (!piece || this.state !== 'playing') return;
		const t = this.time;
		if (t >= piece.duration) {
			if (piece.loop) {
				this.frozenAt = 0;
				void this.play();
				return;
			}
			void this.seek(piece.duration);
			return;
		}
		piece.tl.update(piece.start + Math.max(0, t));
		this.emit();
		this.raf = requestAnimationFrame(this.frame);
	};

	private stop() {
		cancelAnimationFrame(this.raf);
		this.piece?.tl.pause();
		this.bus?.stop(0.06);
		this.bus = null;
	}

	/** The live audio context, created and resumed on the first play (a user gesture). */
	private async audio(): Promise<AudioContext | null> {
		try {
			if (!this.ctx) {
				this.ctx = new AudioContext({ latencyHint: 'interactive' });
				this.kit.muted = this.muted;
				this.kit.build(this.ctx);
				this.kit.setVolume(this.volume);
			}
			if (this.ctx.state !== 'running') await this.ctx.resume();
			return this.ctx;
		} catch (err) {
			console.warn('audio unavailable', err);
			return null;
		}
	}

	private emit() {
		this.onUpdate?.(this.snapshot());
	}
}
