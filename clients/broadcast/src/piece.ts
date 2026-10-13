// A loaded clip: its timeline built on a stage layer, the window of the timeline it shows, and
// the clip-relative operations both the renderer and the live player use. Also the offline
// audio render, for one clip, a clip as an item of a break, or a whole break.

import { Assets } from './assets';
import { SAMPLE_RATE, type Bus } from './audio/engine';
import { Kit } from './audio/kit';
import { normalize, type LoudnessReport } from './audio/loudness';
import { div } from './core/dom';
import type { Cue, KeyRect, PlacedSound } from './core/timeline';
import { FPS } from './core/timeline';
import type { BreakSpec, ClipSpec } from './spec';
import type { Stage } from './stage';
import { templateOf } from './templates';
import type { TL } from './templates/shared';

export const DEFAULT_LOUDNESS = -18;

export interface Piece {
	spec: ClipSpec;
	tl: TL;
	layer: HTMLElement;
	kit: Kit;
	assets: Assets;
	/** Where the clip starts in its timeline (a promo's end card starts after its body). */
	start: number;
	/** Seconds, a whole number of frames. */
	duration: number;
	/**
	 * Seconds the clip's sound lasts as an item of a break, a whole number of frames: its own
	 * length, but a promo body's sound runs on through its end card.
	 */
	soundDuration: number;
	version: number;
	loop: boolean;
	/** Cues relative to the clip. */
	cues: Cue[];
	seek(t: number): Promise<void>;
	isStatic(t0: number, t1: number): boolean;
	keyRect(t: number): KeyRect | null;
	dispose(): void;
}

/** Build a clip on a fresh stage layer once its media is loaded. */
export async function buildPiece(
	stage: Stage,
	spec: ClipSpec,
	kit: Kit,
	{ hidden = false, assets }: { hidden?: boolean; assets?: Assets } = {}
): Promise<Piece> {
	const template = templateOf(spec);
	const loaded = assets ?? (await Assets.load(spec));
	const layer = stage.layer(hidden);
	const tl = template.build({ root: layer, sound: kit, assets: loaded }, spec);
	const [a, b] = tl.parts.clip ?? [0, tl.duration];
	const start = a;
	const duration = Math.round((b - a) * FPS) / FPS;
	const soundDuration = Math.round(((tl.parts.sound?.[1] ?? b) - a) * FPS) / FPS;
	const keyRect = (t: number) => (tl.keyRect ? tl.keyRect(start + t) : null);
	if (tl.keyRect) {
		// the matte layer's shape: the key rectangle as a pure function of t
		const matte = div('key-matte', layer);
		tl.dynamic(
			(t) => JSON.stringify(tl.keyRect?.(t) ?? null),
			(v) => {
				const r = JSON.parse(String(v)) as KeyRect | null;
				matte.style.opacity = r ? String(r.opacity) : '0';
				if (!r) return;
				matte.style.transform = `translate(${r.x}px, ${r.y}px) scale(${r.width / 1920}, ${r.height / 1080})`;
				matte.style.borderRadius = `${r.radius / Math.max(1e-6, r.width / 1920)}px / ${r.radius / Math.max(1e-6, r.height / 1080)}px`;
			}
		);
	}
	const cues = tl.cues
		.filter((c) => c.at >= start - 1e-6 && c.at < start + duration)
		.map((c) => ({ name: c.name, at: c.at - start }))
		.sort((x, y) => x.at - y.at);
	return {
		spec,
		tl,
		layer,
		kit,
		assets: loaded,
		start,
		duration,
		soundDuration,
		version: template.version,
		loop: tl.loop,
		cues,
		async seek(t) {
			await tl.seek(start + t);
			// every image decoded before the frame is painted, whatever the machine's load
			await Promise.all(
				[...layer.querySelectorAll('img')].map((i) =>
					i.src ? i.decode().catch(() => undefined) : undefined
				)
			);
		},
		isStatic: (t0, t1) => !tl.changes(start + t0, start + t1),
		keyRect,
		dispose() {
			tl.cancel();
			layer.remove();
		}
	};
}

export interface AudioRender {
	channels: Float32Array[];
	sampleRate: number;
	report: LoudnessReport;
}

interface Scheduled {
	piece: Piece;
	/** Seconds into the render where the clip starts. */
	at: number;
}

/**
 * Render clips' sound offline: each clip's cues from its window start, placed at `at`, the
 * outgoing clip fading under the next over `crossfade`. A loop renders twice and keeps the
 * second pass, so tails from the end wrap into the start.
 */
async function renderScheduled(
	items: Scheduled[],
	length: number,
	loudness: number,
	{ crossfade = 0.3, loop = false } = {}
): Promise<AudioRender> {
	const passes = loop ? 2 : 1;
	const frames = Math.round(length * SAMPLE_RATE);
	const ctx = new OfflineAudioContext(2, frames * passes, SAMPLE_RATE);
	const kits = new Set(items.map((i) => i.piece.kit));
	for (const k of kits) k.build(ctx);
	const buses: { bus: Bus; at: number }[] = [];
	const sounds: PlacedSound[] = [];
	for (let pass = 0; pass < passes; pass++) {
		for (const { piece, at } of items) {
			const bus = piece.kit.bus();
			sounds.push(
				...piece.tl.soundsFrom(at + pass * length, piece.start, bus, (i) => piece.kit.reseed(i))
			);
			buses.push({ bus, at: at + pass * length });
		}
	}
	buses.sort((x, y) => x.at - y.at);
	buses.forEach((b, i) => {
		const next = buses[i + 1];
		if (next && next.at > b.at && !loop) b.bus.fade(0, next.at, Math.max(0.01, crossfade));
	});
	const buffer = await renderSounds(ctx, sounds);
	const channels = [0, 1].map((c) =>
		buffer.getChannelData(c).slice(frames * (passes - 1), frames * passes)
	);
	const report = normalize(channels, SAMPLE_RATE, loudness);
	return { channels, sampleRate: SAMPLE_RATE, report };
}

// a cue is scheduled between LOOKAHEAD - STEP and LOOKAHEAD seconds before it plays: well
// before its earliest node starts (an oscillator may start a cycle early, 0.22 s at most)
const LOOKAHEAD = 1;
const STEP = 0.5;

/**
 * Render an offline context with its sound cues scheduled shortly before they play, not all
 * up front: Chromium pulls every connected node every 128 frames, so a voice built at the
 * start costs from then on, and a break's thousands of voices rendered slower than real time.
 * A voice's nodes are silent until it starts either way, so the samples are the same.
 */
function renderSounds(ctx: OfflineAudioContext, sounds: PlacedSound[]): Promise<AudioBuffer> {
	const queue = [...sounds].sort((a, b) => a.when - b.when);
	let next = 0;
	const until = (t: number) => {
		while (next < queue.length && queue[next].when < t) queue[next++].play();
	};
	until(LOOKAHEAD);
	const last = queue.length ? queue[queue.length - 1].when : 0;
	const end = ctx.length / ctx.sampleRate;
	for (let t = STEP; t < end && t + LOOKAHEAD - STEP <= last; t += STEP) {
		const at = t;
		void ctx.suspend(at).then(() => {
			until(at + LOOKAHEAD);
			return ctx.resume();
		});
	}
	return ctx.startRendering();
}

export function renderPieceAudio(piece: Piece): Promise<AudioRender> {
	return renderScheduled(
		[{ piece, at: 0 }],
		piece.duration,
		piece.spec.loudness ?? DEFAULT_LOUDNESS,
		{
			loop: piece.loop
		}
	);
}

/** How long an item's sound may ring out past its end: the crossfade and the hall's 3.2 s. */
const ITEM_TAIL = 4;

// ring-out under this (-120 dBFS) is cut from an item's render
const SILENT = 1e-6;

// An item renders this far into its context, as every item after a break's first did: the
// compressors open a render clamped down for its first few hundred milliseconds, and a voice
// may start up to a cycle of a 4.7 Hz vibrato before its note, which at 0 starts on the note.
const PREROLL = 0.5;

export interface ItemRender {
	channels: Float32Array[];
	sampleRate: number;
}

/**
 * A clip's sound as an item of a break, the way one render of the whole break played it: its
 * cues from its start, its buses faded out over `crossfade` from where its sound ends (where
 * the next item starts) and the reverb ringing out under the next item, at the engine's own
 * level. A break mixes its items' renders and normalizes the mix, so each item renders once,
 * whatever break it airs in.
 */
export async function renderItemAudio(
	piece: Piece,
	{ crossfade = 0.3, tail = ITEM_TAIL } = {}
): Promise<ItemRender> {
	const length = Math.round(piece.soundDuration * SAMPLE_RATE);
	const pre = Math.round(PREROLL * SAMPLE_RATE);
	const ctx = new OfflineAudioContext(
		2,
		pre + length + Math.round(tail * SAMPLE_RATE),
		SAMPLE_RATE
	);
	piece.kit.build(ctx);
	const bus = piece.kit.bus();
	bus.fade(0, PREROLL + piece.soundDuration, Math.max(0.01, crossfade));
	const buffer = await renderSounds(
		ctx,
		piece.tl.soundsFrom(PREROLL, piece.start, bus, (i) => piece.kit.reseed(i))
	);
	let end = pre + length;
	for (let c = 0; c < 2; c++) {
		const d = buffer.getChannelData(c);
		let i = d.length;
		while (i > end && Math.abs(d[i - 1]) < SILENT) i--;
		end = i;
	}
	return {
		channels: [0, 1].map((c) => buffer.getChannelData(c).slice(pre, end)),
		sampleRate: SAMPLE_RATE
	};
}

export interface Break {
	duration: number;
	render(): Promise<AudioRender>;
	dispose(): void;
}

/** Build every item of a break (laid out, never shown) for one render of its sound. */
export async function buildBreak(stage: Stage, spec: BreakSpec): Promise<Break> {
	const kit = new Kit();
	const pieces = await Promise.all(
		spec.items.map((it) => buildPiece(stage, it.spec, kit, { hidden: true }))
	);
	const items = spec.items.map((it, i) => ({ piece: pieces[i], at: it.at }));
	const duration = Math.round(Math.max(...items.map((i) => i.at + i.piece.duration)) * FPS) / FPS;
	return {
		duration,
		render: () =>
			renderScheduled(items, duration, spec.loudness ?? DEFAULT_LOUDNESS, {
				crossfade: spec.crossfade ?? 0.3
			}),
		dispose() {
			for (const p of pieces) p.dispose();
		}
	};
}
