// The trailer: the long promo (60 to 90 s), cut from the title's own scenes with their own sound.
// The plan (trailer-plan.ts) decides what plays when; this draws it and the score
// (trailer-score.ts) sounds it, then the promo's own end card (the airing, the channel tag on
// the sonic logo) continues on the body's next frame. The body is timeless like a promo's: it
// never shows anything about when the title airs.
//
// The moods are the promo's three lights. Cinematic is dusk (dissolves between scenes, cards on
// night with the dusk rising, quiet type), grit is night (letterboxed, hard cuts, big words cut
// in whole), bright is day (the red field sliding across for every card, paper type). Cards are
// flat fields with type rising out of its own mask; scenes are the title's own footage, cover
// cropped toward their focus; the title is its logo over the hero scene's grade, else its name.
// Only transform and opacity move, and a take holds an image only around its own time.

import type { LoadedBed } from '../audio/engine';
import { lockup } from '../brand/lockup';
import { css, div, img } from '../core/dom';
import { edge } from '../core/moves';
import { FPS, frame } from '../core/timeline';
import type { Airing, Crop, PromoContent, TrailerContent, TrailerSpec } from '../spec';
import {
	BAND,
	BODY_AIRING,
	END_CARD,
	FULL,
	NIGHT,
	W,
	buildPromo,
	centred,
	defaultWords,
	fitted,
	metaLine,
	place,
	rise,
	shade,
	sink,
	titleMark,
	type Rising,
	type View
} from './promo';
import { logoImage } from './reel';
import { timeline, type Ctx, type Template, type TL } from './shared';
import { planTrailer, type Mood, type SceneSection, type Section } from './trailer-plan';
import { scoreTrailer } from './trailer-score';

export const trailer: Template<TrailerSpec> = {
	version: 1,
	build(ctx, spec) {
		const airing = spec.part === 'body' ? BODY_AIRING : (spec.airing ?? null);
		const tl = buildTrailer(ctx, spec, airing);
		if (spec.part !== 'full') {
			const [a, b] = tl.parts[spec.part];
			// in a break the body's sound runs on through its end card (the break plays the full
			// trailer there), and an end card's words are seen, never heard, so it is the same sound
			tl.parts = spec.part === 'body' ? { clip: [a, b], sound: tl.parts.full } : { clip: [a, b] };
		}
		return tl;
	}
};

const fx = (n: number) => n.toFixed(2);

/** The dusk rising behind a card's type, the premiere card's. */
const DUSK =
	'radial-gradient(1500px 640px at 960px 1240px, rgb(86 6 16 / 0.85), rgb(38 3 6 / 0.64) 38%, rgb(20 2 3 / 0.34) 62%, transparent)';

/** A take's slow push (zoom from, to) by mood. */
const PUSH: Record<Mood, [number, number]> = {
	cinematic: [1, 1.05],
	grit: [1.02, 1.07],
	bright: [1, 1.05]
};

interface Shot {
	frames: string[];
	focus?: Crop;
}

interface TakeOptions {
	view?: View;
	/** Seconds of dissolve centred on the cut (0: a cut). */
	dissolve?: number;
	/** Seconds the first picture fades up from night. */
	fade?: number;
	push?: [number, number];
	focus?: Crop;
}

function buildTrailer(
	{ root, sound, assets }: Ctx,
	spec: TrailerContent,
	airing: Airing | null
): TL {
	const t = spec.title;
	const mood = spec.mood;
	const grit = mood === 'grit';
	const bright = mood === 'bright';
	const cinematic = mood === 'cinematic';
	const bed: LoadedBed | null = assets.bed(spec.bed);

	// the cards: the title's taglines in order, else its words
	const taglines = (t.taglines ?? []).filter(Boolean);
	const texts = (taglines.length ? taglines : defaultWords(spec)).slice(0, 3);
	// the lines as the plan and the sound both read them: inside their scene
	const spoken = spec.scenes.map((s) =>
		s.lines
			.map(([a, b]): [number, number] => [Math.max(0, a), Math.min(s.frames.length / FPS, b)])
			.filter(([a, b]) => b > a)
	);
	const plan = planTrailer({
		mood,
		premiere: !!spec.premiere,
		scenes: spec.scenes.map((s, i) => ({ len: s.frames.length / FPS, lines: spoken[i] })),
		cards: texts.map((x) => x.length),
		bed: bed ? { onsets: bed.onsets } : null
	});
	const B = plan.end;
	const D = frame(B + END_CARD);
	const tl = timeline(root, D);
	tl.parts = { body: [0, B], end: [B, D], full: [0, D] };
	root.classList.add('pm', `pm-${mood}`);

	const pics = div('pm-fill', root);
	const grades = div('pm-fill', root);
	const fields = div('pm-fill', root);
	const text = div('pm-fill', root);
	const view = grit ? BAND : FULL;

	// --- pictures -------------------------------------------------------------------------------

	/** One take of `frames` from frame `first`, on screen over [a, b). */
	function take(frames: string[], first: number, a: number, b: number, o: TakeOptions = {}) {
		const v = o.view ?? view;
		const d = a > 0 ? (o.dissolve ?? 0) : 0;
		const inT = a - d / 2;
		const outT = b + d / 2;
		const [z0, z1] = o.push ?? PUSH[mood];
		const box = place(div('pm-take', pics), v);
		const im = img('pm-footage', box);
		const [x, y, w, h] = o.focus ?? [0, 0, 1, 1];
		const focus = `${fx((x + w / 2) * 100)}% ${fx((y + h / 2) * 100)}%`;
		css(im, { objectPosition: focus, transformOrigin: focus });
		tl.footage(im, frames.slice(first), a, [inT - 0.2, outT + 0.2]);
		tl.add(
			im,
			[{ transform: `scale(${z0})` }, { transform: `scale(${z1})` }],
			inT,
			outT - inT,
			'linear'
		);
		tl.show(box, edge(inT), edge(outT));
		if (d) tl.add(box, [{ opacity: 0 }, { opacity: 1 }], inT, d, 'sine');
		else if (o.fade) tl.add(box, [{ opacity: 0 }, { opacity: 1 }], a, o.fade, 'sine');
		return box;
	}

	// the montage's shots: the title's footage, else the scenes' own pictures
	const pool: Shot[] = t.footage?.length
		? t.footage
		: spec.scenes.map((s) => ({ frames: s.frames, focus: s.focus }));
	// where each shot was left, so one used again goes on rather than repeating
	const cursor = new Map<Shot, number>();
	let playing: Shot | null = null;
	const left = (s: Shot) => s.frames.length - (cursor.get(s) ?? 0);
	/** The shot with the most left to show that is not on screen. */
	const freshest = () => {
		let best: Shot | undefined;
		for (const s of pool) if (s !== playing && (!best || left(s) > left(best))) best = s;
		return best ?? pool[0];
	};
	/** A take of `shot` over [a, b) from where it was left. */
	function cutTo(shot: Shot, a: number, b: number, o: TakeOptions = {}) {
		const d = a > 0 ? (o.dissolve ?? 0) : 0;
		const need = Math.ceil((b - a + d) * FPS) + 1;
		let from = cursor.get(shot) ?? 0;
		if (from + need > shot.frames.length) from = 0;
		cursor.set(shot, Math.min(shot.frames.length, from + need));
		playing = shot;
		return take(shot.frames, from, a, b, { focus: shot.focus, ...o });
	}
	/** Takes over [a, b): one long take, going on in the next shot where one runs out. */
	function longTake(a: number, b: number, o: TakeOptions = {}) {
		let at = a;
		for (let n = 0; b - at > 0.02; n++) {
			const shot = freshest();
			// what it can show: the frames after where it was left, else all of it again
			const need = Math.ceil((b - at) * FPS);
			const from = cursor.get(shot) ?? 0;
			const have =
				(from + need <= shot.frames.length ? need : Math.min(need, shot.frames.length)) / FPS;
			let end = have >= b - at - 1e-6 ? b : at + have;
			// the take after it is long enough to read
			if (end < b && b - end < 0.6 && b - at > 1.4) end = b - 0.6;
			end = Math.max(frame(end), frame(at) + 1 / FPS);
			cutTo(shot, at, end, { ...o, dissolve: n ? (cinematic ? 0.5 : 0) : o.dissolve });
			at = end;
		}
	}

	// --- fields and type -------------------------------------------------------------------------

	/** When the last word before `t` ends (a field must not close over it); never without one. */
	const spokenUntil = (t: number) => {
		const prev = plan.sections.find((s) => s.kind === 'scene' && Math.abs(s.end - t) < 1e-6);
		const last = prev?.kind === 'scene' ? prev.lines.at(-1)?.[1] : undefined;
		return last === undefined ? -Infinity : last + 0.03;
	};
	/** When the first word after `t` starts (a field must be gone by then). */
	const speaksFrom = (t: number) => {
		const next = plan.sections.find((s) => s.kind === 'scene' && Math.abs(s.at - t) < 1e-6);
		return next?.kind === 'scene' && next.lines.length ? next.lines[0][0] : Infinity;
	};

	/**
	 * A night field over [a, b), fading in and out over `d` seconds centred on its edges, or cut
	 * where a fade would cover a word the scene before it is still speaking, or where the scene
	 * after it already speaks.
	 */
	const night = (a: number, b: number, d = 0) => {
		const f = div('pm-fill pm-night', fields);
		const inAt = Math.max(a - d / 2, spokenUntil(a));
		const fadeIn = d > 0 && a + d / 2 - inAt >= 0.08;
		const outEnd = Math.min(b + d / 2, speaksFrom(b));
		const fadeOut = d > 0 && outEnd - (b - d / 2) >= 0.08;
		tl.show(f, edge(fadeIn ? inAt : a), edge(fadeOut ? outEnd : b));
		if (fadeIn) tl.add(f, [{ opacity: 0 }, { opacity: 1 }], inAt, a + d / 2 - inAt, 'sine');
		if (fadeOut)
			tl.add(f, [{ opacity: 1 }, { opacity: 0 }], b - d / 2, outEnd - (b - d / 2), 'sine');
		return f;
	};
	/**
	 * The red field: a sheet that slides in from the right over what plays (once the scene before
	 * has said its last word) and away to the left.
	 */
	const sheet = (a: number, b: number) => {
		const f = div('pm-fill pm-red', fields);
		const start = Math.min(a - 0.05, Math.max(a - 0.4, spokenUntil(a)));
		tl.show(f, edge(start - 0.05), edge(b + 0.6));
		tl.add(
			f,
			[{ transform: `translateX(${W}px)` }, { transform: 'translateX(0px)' }],
			start,
			a + 0.15 - start,
			'move'
		);
		tl.add(
			f,
			[{ transform: 'translateX(0px)' }, { transform: `translateX(${-W}px)` }],
			b,
			0.55,
			'move'
		);
		return f;
	};
	const rule = (box: HTMLElement, at: number, width: number, margin: number, out?: number) => {
		const r = css(div('pm-rule', box), {
			width: `${width}px`,
			marginTop: `${margin}px`,
			transformOrigin: '0 50%'
		});
		tl.add(r, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], at, 0.9, 'glide');
		if (out !== undefined)
			tl.add(r, [{ transform: 'scaleX(1)' }, { transform: 'scaleX(0)' }], out, 0.36, 'leave');
		return r;
	};

	const sceneOf = (s: SceneSection) => spec.scenes[s.scene];

	function scene(s: SceneSection, o: { dissolve: number; fade?: number }) {
		const sc = sceneOf(s);
		take(sc.frames, Math.round(s.from * FPS), s.at, s.end, { ...o, focus: sc.focus });
	}

	/** The channel's moment: a red rule drawing under the row lockup, nothing else. */
	function presents(a: number, b: number) {
		night(a, b, cinematic ? 0.4 : 0);
		const box = centred(text);
		tl.show(box, edge(a), edge(b));
		const group = css(div(null, box), {
			display: 'flex',
			flexDirection: 'column',
			alignItems: 'center'
		});
		const mask = div('tr-mask', group);
		const mark = lockup(mask, 128, 'tr-lockup', 'row');
		tl.add(
			mark,
			[{ transform: 'translateY(130%)' }, { transform: 'translateY(0%)' }],
			a + 0.3,
			1.0,
			'glide'
		);
		tl.add(
			mark,
			[{ transform: 'translateY(0%)' }, { transform: 'translateY(-130%)' }],
			b - 0.45,
			0.42,
			'leave'
		);
		rule(group, a + 0.8, 96, 40, b - 0.45);
	}

	/** A tagline between scenes, in the mood's own field. */
	function card(line: string, words: boolean, a: number, b: number, size?: number) {
		// the type is a layer of its own over the field; it rises only once the sheet covers its place
		let [from, to] = [a, b];
		if (bright) {
			sheet(a, b);
			[from, to] = [a - 0.45, b + 0.6];
		} else {
			const f = night(a, b, cinematic ? 0.4 : 0);
			if (cinematic) shade(f, DUSK);
		}
		const holder = centred(text);
		tl.show(holder, edge(from), edge(to));
		const cls = grit ? (words ? 'pm-word' : 'pm-display2') : bright ? 'pm-display2' : 'tr-line';
		const r = fitted(
			holder,
			cls,
			line,
			grit && words ? { width: 1640, min: 120 } : { width: 1560, min: 84, lines: 2 },
			size
		);
		if (grit) {
			// cut in whole on the beat; the rule draws itself under it
			rule(holder, a + 0.04, grit && words ? Math.round(r.w) : 96, grit && words ? 18 : 30);
		} else {
			rise(tl, r, a + (bright ? 0.25 : 0.3), 0.1, 1.0);
			sink(tl, r, b - (bright ? 0.7 : 0.5));
			if (cinematic) rule(holder, a + 0.8, 96, 36, b - 0.5);
		}
	}

	/** The premiere card: the mood's word with its own weight. */
	function premiereCard(a: number, b: number) {
		if (grit) return card('Premiere', true, a, b);
		if (bright) return card('Premiere', false, a, b, 168);
		const f = night(a, b, 0.4);
		shade(f, DUSK);
		const box = centred(text);
		tl.show(box, edge(a), edge(b));
		const r = fitted(box, 'pm-display1', 'Premiere', { width: 1640, min: 120 }, 184);
		rise(tl, r, a + 0.2, 0, 1.0);
		rule(box, a + 0.5, 520, 28, b - 0.4);
		sink(tl, r, b - 0.4);
	}

	/** The mark of the title: its logo (readable on night), else its name in type. */
	function mark(parent: Element): Rising {
		const box = { width: 1100, height: 360, area: 300000 };
		if (t.logo?.url) {
			const holder = div('pm-logo', parent);
			const got = logoImage(holder, assets, t.logo, '#050505', box);
			if (got) {
				css(holder, { width: `${got.w + 24}px`, height: `${got.h + 24}px` });
				return { el: holder, parts: [got.el], from: `${got.h + 16}px`, h: got.h, w: got.w };
			}
			holder.remove();
		}
		return titleMark(
			parent,
			{ ...t, logo: undefined },
			{ w: 1100, h: 360, area: 300000 },
			'pm-display1',
			{
				width: 1640,
				min: 104,
				lines: 3,
				height: 560
			}
		);
	}

	/** The hit: the title over the hero scene's grade, its logo big, a red rule, the meta line. */
	function titleMoment(a: number, b: number) {
		longTake(a, b, { dissolve: 0, push: [1.04, 1.1] });
		const dim = shade(
			grades,
			`radial-gradient(1500px 640px at 960px 540px, rgb(${NIGHT} / 0.3), rgb(${NIGHT} / 0) 72%), rgb(${NIGHT} / ${grit ? 0.55 : bright ? 0.35 : 0.4})`
		);
		tl.show(dim, edge(a), edge(b));
		const box = centred(text);
		tl.show(box, edge(a), edge(b));
		const group = css(div(null, box), {
			display: 'flex',
			flexDirection: 'column',
			alignItems: 'center'
		});
		const logo = mark(group);
		const bar = css(div('pm-rule', group), { width: '96px', marginTop: t.logo ? '40px' : '34px' });
		const meta = css(metaLine(group, t), { marginTop: '26px' });
		// grit cuts it in whole on the hit, rule and all
		if (grit) return;
		// the logo rises once the red field has mostly gone
		const lag = bright ? 0.3 : 0;
		rise(tl, logo, a + lag + 0.12, 0.1, 1.0);
		tl.add(
			bar,
			[{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }],
			a + lag + 0.6,
			0.9,
			'glide'
		);
		tl.add(
			meta,
			[
				{ opacity: 0, transform: 'translateY(14px)' },
				{ opacity: 1, transform: 'translateY(0px)' }
			],
			a + lag + 0.8,
			0.7,
			'glide'
		);
		tl.add(group, [{ transform: 'scale(1)' }, { transform: 'scale(1.03)' }], a, b - a, 'linear');
		sink(tl, logo, b - 0.45);
		tl.add(bar, [{ transform: 'scaleX(1)' }, { transform: 'scaleX(0)' }], b - 0.45, 0.36, 'leave');
		tl.add(meta, [{ opacity: 1 }, { opacity: 0 }], b - 0.45, 0.3, 'leave');
	}

	/** Cuts on the montage's beats: each a take of the shot with the most left, a cut or a hard step. */
	function montage(s: Extract<Section, { kind: 'montage' }>) {
		s.cuts.forEach((c, i) => {
			const e = i + 1 < s.cuts.length ? s.cuts[i + 1] : s.end;
			cutTo(freshest(), c, e, { push: [1.02, 1.08] });
		});
	}

	for (const s of plan.sections) {
		switch (s.kind) {
			case 'scene': {
				tl.cue(
					s.role === 'open' ? 'the cold open' : s.role === 'button' ? 'the button' : 'scene',
					s.at
				);
				const dissolve =
					cinematic && s.role === 'act' ? 0.5 : cinematic && s.role === 'button' ? 0.4 : 0;
				const first = sceneOf(s).lines.find((l) => l[0] >= s.from);
				const fade =
					s.role === 'open'
						? Math.min(1.0, Math.max(0.3, first ? first[0] - s.from - 0.06 : 1))
						: 0;
				scene(s, { dissolve, fade });
				break;
			}
			case 'presents':
				tl.cue('the channel', s.at);
				presents(s.at, s.end);
				break;
			case 'card': {
				tl.cue('card', s.at);
				const line = texts[s.card] ?? '';
				if (line) card(line, !taglines.length, s.at, s.end);
				break;
			}
			case 'premiere':
				tl.cue('premiere card', s.at);
				premiereCard(s.at, s.end);
				break;
			case 'montage':
				tl.cue('montage, cuts on the beat', s.at);
				montage(s);
				break;
			case 'title':
				tl.cue('the title', s.at);
				// the hit lands on a breath: the picture goes to night for two frames, as the music
				// drops out, and bright's red field crosses the frame instead and leaves the title
				if (bright) sheet(s.at - 0.05, s.at + 0.1);
				else night(s.at - 2 / FPS, s.at);
				titleMoment(s.at, s.end);
				break;
		}
	}

	// --- the end card is the promo's -------------------------------------------------------------

	const end: PromoContent = {
		mood,
		premiere: spec.premiere,
		title: t,
		shots: spec.scenes.map((s) => ({ kind: 'footage', frames: s.frames, focus: s.focus })),
		loudness: spec.loudness
	};
	buildPromo({ root, sound, assets }, end, airing, { tl, at: B });

	// --- the sound -------------------------------------------------------------------------------

	const tag = tl.cues.find((c) => c.name.startsWith('channel tag'));
	scoreTrailer({
		tl,
		kit: sound,
		mood,
		plan,
		D,
		tagAt: tag?.at ?? B + 1.3,
		bed,
		scenes: spec.scenes.map((s, i) => ({ sound: assets.bed(s.sound), lines: spoken[i] })),
		premiere: !!spec.premiere
	});
	return tl;
}
