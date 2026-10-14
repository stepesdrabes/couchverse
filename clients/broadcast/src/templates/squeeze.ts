// The credits squeeze: the ending program pulls back from the frame into the channel's dark
// room, a sharp panel in the night, as a camera leaving a screen; the next program arrives
// beside it as a card playing scenes from itself under its logo (or its name); then either the
// camera pushes into the card until it is the frame (`expand`: up next continues from that
// frame), or the room clears and the program shrinks onto the couch in its place in the lockup
// and dissolves into it over the night field (`ident`: the short ident's first frame).
//
// The panel's motion is data (segments of translate/scale and opacity) that drives both the
// animations and `keyRect(t)`, so the rectangle a compositor needs is exact for every frame.
// What fills the panel is the spec's `program`: the program's own frames, a stand-in (the
// ending title's backdrop under a credits roll), or nothing (`key`: the panel is transparent,
// showing the background behind it, and the `matte` layer carries the program's alpha). While
// the card holds, nothing in the page moves but the card's scenes.
//
// Sound: the program's own music carries on, ducked while the card arrives; air swells under
// the pull back, one glass note as the card settles; into the push a rising air as the music
// fades under it, or the music fading for the short ident's score.

import type { Bus } from '../audio/engine';
import type { Kit } from '../audio/kit';
import { air, glass, house, ringOut } from '../audio/voices';
import { Ending, STAGE } from '../brand/ending';
import { COUCH_BOX } from '../brand/glyph';
import { bug } from '../brand/lockup';
import { SHORT_IDENT_TIMES } from '../brand/short-ident';
import { END } from '../brand/sonic';
import { BRAND } from '../brand/wordmark';
import { css, div, img, span } from '../core/dom';
import { fitText } from '../core/fit';
import { framed, W } from '../core/framing';
import { bezierPoints, cubicBezierAt, easing, type Ease } from '../core/motion';
import { edge, windows } from '../core/moves';
import type { KeyRect } from '../core/timeline';
import type { SqueezeSpec } from '../spec';
import {
	conceal,
	GRADE,
	grade,
	label,
	labelIn,
	labelOut,
	masked,
	night,
	reveal,
	scenes,
	titleMark
} from './onair';
import { timeline, type Template, type TL } from './shared';
import { UPNEXT } from './upnext';

interface Box {
	x: number;
	y: number;
	sx: number;
	sy: number;
}

interface Seg<T> {
	at: number;
	dur: number;
	from: T;
	to: T;
	ease: Ease;
}

const tf = (b: Box) => `translate(${b.x}px, ${b.y}px) scale(${b.sx}, ${b.sy})`;

/** A value driven by segments: animated on the element and evaluated in JS alike. */
class Track<T> {
	readonly segs: Seg<T>[] = [];
	constructor(
		readonly rest: T,
		private readonly lerp: (a: T, b: T, k: number) => T
	) {}
	to(at: number, dur: number, to: T, ease: Ease) {
		this.segs.push({ at, dur, from: this.at(at), to, ease });
		return this;
	}
	at(t: number): T {
		let v = this.rest;
		for (const s of this.segs) {
			if (t < s.at) break;
			const k = s.dur > 0 ? Math.min(1, (t - s.at) / s.dur) : 1;
			v = this.lerp(s.from, s.to, cubicBezierAt(bezierPoints(s.ease), k));
		}
		return v;
	}
	animate(tl: TL, el: Element, frame: (v: T) => Keyframe) {
		for (const s of this.segs)
			tl.add(el, [frame(s.from), frame(s.to)], s.at, s.dur, easing(s.ease));
	}
}

const mixBox = (a: Box, b: Box, k: number): Box => ({
	x: a.x + (b.x - a.x) * k,
	y: a.y + (b.y - a.y) * k,
	sx: a.sx + (b.sx - a.sx) * k,
	sy: a.sy + (b.sy - a.sy) * k
});
const mixNum = (a: number, b: number, k: number) => a + (b - a) * k;

/** The program at the hold: today's geometry, so the compositor and the handover keep working. */
const HOLD: Box = { x: 96, y: 236, sx: 0.525, sy: 0.525 };
/** Full frame, overscaled so the panel's edges start outside it. */
const FULL: Box = { x: -48, y: -27, sx: 1.05, sy: 1.05 };
/** The next program's card beside it, as tall; its type 40 px in from the sides and the foot. */
const CARD = { x: 1168, y: 236, w: 656, h: 567, pad: 40 };
/** The card's logo: the same visual weight whatever its shape, within the card's type column. */
const LOGO = { area: 76000, width: 540, height: 200 };
/** When things happen (DESIGN.md 2.2). */
const T = {
	pull: [2.0, 3.6] as const,
	cardIn: 3.0,
	typeIn: 3.4,
	expand: 9.4,
	push: [9.6, 11.2] as const,
	ident: 9.2
};

// stand-in crew for the credits roll (the preview has no program video)
const CREDITS: [string, string][] = [
	['Directed by', 'Mara Lindqvist'],
	['Screenplay by', 'Tomas Reyes'],
	['Director of Photography', 'Ines Kowalczyk'],
	['Edited by', 'Jonah Whitfield'],
	['Production Designer', 'Adaeze Okafor'],
	['Music by', 'Lukas Hartmann'],
	['Costume Designer', 'Sofia Marchetti'],
	['Casting by', 'Priya Raman'],
	['Visual Effects Supervisor', 'Henrik Dahl'],
	['Sound Designer', 'Camille Laurent'],
	['Re-recording Mixer', 'Oskar Novak'],
	['First Assistant Director', 'Ruth Abernathy'],
	['Supervising Art Director', 'Diego Fuentes'],
	['Script Supervisor', 'Hana Sato'],
	['Gaffer', 'Peter Brandt'],
	['Key Grip', 'Elena Petrova']
];

interface Rect {
	x: number;
	y: number;
	w: number;
	h: number;
}
const mixRect = (a: Rect, b: Rect, k: number): Rect => ({
	x: mixNum(a.x, b.x, k),
	y: mixNum(a.y, b.y, k),
	w: mixNum(a.w, b.w, k),
	h: mixNum(a.h, b.h, k)
});

export const squeeze: Template<SqueezeSpec> = {
	version: 9,
	build({ root, sound, assets }, spec) {
		// `cursor` is this handover's old name, which squeezes composed before the rename carry
		const toIdent = spec.handover !== 'expand';
		const D = toIdent ? 10.6 : 11.52;
		const tl = timeline(root, D);
		root.classList.add('squeeze');
		const nx = spec.next;
		const bed = assets.bed(spec.bed);
		const [p0, p1] = T.pull;
		const [q0, q1] = T.push;
		const h0 = toIdent ? T.ident : T.expand;

		// the room: night with the dusk low on its horizon (static)
		const room = div('fill sq-room', root);

		// the ending program
		const prog = div('program', root);
		if (spec.program.kind === 'frames') {
			const frameImg = css(img(null, prog), { width: '100%', height: '100%', objectFit: 'cover' });
			tl.footage(frameImg, spec.program.frames, 0);
		} else if (spec.program.kind === 'standIn') {
			const shot = framed(prog, assets.picture(spec.ending.backdrop));
			div('dim', prog);
			const credits = div('credits', prog);
			for (const [role, name] of spec.program.credits ?? CREDITS) {
				div('role', credits, role);
				div('name', credits, name);
			}
			tl.add(
				credits,
				[{ transform: 'translateY(420px)' }, { transform: `translateY(${420 - 70 * D}px)` }],
				0,
				D,
				'linear'
			);
			const c = spec.ending.crops?.squeeze ?? [0, 0, 1, 1];
			tl.add(
				shot.img,
				[
					{ transform: shot.tf(c) },
					{ transform: shot.tf([c[0] + c[2] * 0.05, c[1] + c[3] * 0.05, c[2] * 0.9, c[3] * 0.9]) }
				],
				0,
				D,
				'linear'
			);
		} else {
			prog.classList.add('keyed');
		}
		// a hairline round the panel at the hold, so a dark program still reads as a screen in the
		// room (outside the key: the program covers what the page draws inside it)
		const rim = css(div('abs sq-rim', root), {
			left: `${HOLD.x - 4}px`,
			top: `${HOLD.y - 4}px`,
			width: `${W * HOLD.sx + 8}px`,
			height: `${1080 * HOLD.sy + 8}px`
		});

		// the next program's card: a window onto its scenes, which live in stage pixels inside it
		const card = css(div('abs sq-card', root), { width: `${CARD.w}px`, height: `${CARD.h}px` });
		const lens = div('abs sq-lens', card);
		// framed as up next frames it, so the push ends on its first frame
		const crop = nx.title.crops?.upNext?.[0] ?? UPNEXT.crop;
		scenes(tl, lens, nx.title, assets, { start: T.cardIn, end: D, crop });
		const cardGrade = grade(card, [
			`linear-gradient(0deg, ${night(0.95)} 0%, ${night(0.8)} 34%, ${night(0)} 70%)`
		]);
		const text = css(div('abs sq-text', card), {
			left: `${CARD.pad}px`,
			right: `${CARD.pad}px`,
			bottom: `${CARD.pad - 2}px`
		});
		const row = div('sq-row', text);
		const lbl = label(row, 'Up next', 'sq-label');
		const time = masked(row, 'sq-time tnum', nx.time);
		const mark = titleMark(
			text,
			nx.title,
			assets,
			'sq-title',
			{ width: CARD.w - 2 * CARD.pad, min: 40, lines: 3, leading: 1.0 },
			LOGO
		);
		let ep: HTMLElement | null = null;
		if (nx.episode) {
			ep = div('sq-ep', div('oa-mask sq-ep-mask', text));
			span('sq-code tnum', ep, nx.episode.code);
			ep.append(' ');
			span(null, ep, nx.episode.name);
			fitText(ep, { width: CARD.w - 2 * CARD.pad, min: 22, lines: 2, leading: 1.25 });
		}

		// what up next starts from: its picture and its grade (expand only)
		const after = toIdent
			? null
			: grade(root, [GRADE.bug, GRADE.left(66), GRADE.foot(58), GRADE.dim(0.14)]);
		const b = spec.clock ? bug(tl, root, spec.clock) : null;

		// --- the program panel, as data --------------------------------------------------------
		const box = new Track<Box>(FULL, mixBox).to(p0, p1 - p0, HOLD, 'dolly');
		const opacity = new Track<number>(1, mixNum);
		// the couch's box in its place in the lockup
		const lk = STAGE.logo.k;
		const onto: Box = {
			x: STAGE.logo.x + COUCH_BOX[0] * lk,
			y: STAGE.logo.y + COUCH_BOX[1] * lk,
			sx: ((COUCH_BOX[2] - COUCH_BOX[0]) * lk) / W,
			sy: ((COUCH_BOX[3] - COUCH_BOX[1]) * lk) / 1080
		};
		// the stage is night by then: the couch shows under the panel, which dissolves into it
		const handed = edge(h0 + 1.02);
		if (toIdent) {
			box.to(h0 + 0.1, 0.9, onto, 'move');
			opacity.to(handed, D - 0.04 - handed, 0, 'sine');
		} else {
			const k = HOLD.sx * 0.92;
			box.to(q0, q1 - q0, { x: -1180, y: 276, sx: k, sy: k }, 'move');
			opacity.to(10.2, 0.9, 0, 'sine');
		}
		tl.add(prog, [{ transform: tf(FULL) }, { transform: tf(FULL) }], 0, 0.01, 'linear');
		box.animate(tl, prog, (v) => ({ transform: tf(v) }));
		opacity.animate(tl, prog, (v) => ({ opacity: v }));

		tl.keyRect = (t: number): KeyRect | null => {
			const v = box.at(t);
			const alpha = opacity.at(t);
			if (alpha <= 0) return null;
			return {
				x: v.x,
				y: v.y,
				width: W * v.sx,
				height: 1080 * v.sy,
				radius: 0,
				opacity: alpha
			};
		};

		// --- the card, as data: its window and the picture inside it ----------------------------
		const fullCover: Rect = { x: -0.04 * 1920, y: -0.04 * 1080, w: 1.08 * 1920, h: 1.08 * 1080 };
		const cardBox = new Track<Rect>({ ...CARD, x: CARD.x + 64 }, mixRect).to(
			T.cardIn,
			1.1,
			CARD,
			'glide'
		);
		if (!toIdent) cardBox.to(q0, q1 - q0, fullCover, 'move');
		const zoom = new Track<number>(1.1, mixNum).to(T.cardIn, 1.6, 1, 'glide');
		const pushed = new Track<number>(0, mixNum).to(q0, q1 - q0, 1, 'move');
		// up next's first frame: its camera's zoom about the middle of the frame
		const Z = UPNEXT.zoom[0];
		const end = { x: (1920 * (1 - Z)) / 2, y: (1080 * (1 - Z)) / 2, k: Z };
		const moving: [number, number][] = [[T.cardIn, T.cardIn + 1.6]];
		if (!toIdent) moving.push([q0, q1]);
		// held frames share one value, so the renderer reuses them
		const still = (t: number) => Math.max(0, ...moving.filter(([, z]) => z < t).map(([, z]) => z));
		tl.dynamic(
			(t) => (moving.some(([a, z]) => t >= a && t <= z) ? t : `hold ${still(t)}`),
			(v) => {
				const t = typeof v === 'number' ? v : Number(String(v).slice(5));
				const c = cardBox.at(t);
				const sx = c.w / CARD.w;
				const sy = c.h / CARD.h;
				card.style.transform = `translate(${c.x.toFixed(2)}px, ${c.y.toFixed(2)}px) scale(${sx.toFixed(5)}, ${sy.toFixed(5)})`;
				// the picture covering the card at the hold, up next's first frame at the end of the
				// push, mixed between by the push's own progress (the mix of two covers covers the
				// mix of the two windows, so no edge ever shows)
				const k0 = Math.max(CARD.w / 1920, CARD.h / 1080) * zoom.at(t);
				let X = c.x + (c.w - 1920 * k0) / 2;
				let Y = c.y + (c.h - 1080 * k0) / 2;
				let K = k0;
				const p = pushed.at(t);
				if (p > 0) {
					const x0 = CARD.x + (CARD.w - 1920 * k0) / 2;
					const y0 = CARD.y + (CARD.h - 1080 * k0) / 2;
					X = mixNum(x0, end.x, p);
					Y = mixNum(y0, end.y, p);
					K = mixNum(k0, end.k, p);
				}
				lens.style.transform = `scale(${(1 / sx).toFixed(5)}, ${(1 / sy).toFixed(5)}) translate(${(X - c.x).toFixed(2)}px, ${(Y - c.y).toFixed(2)}px) scale(${K.toFixed(5)})`;
			}
		);

		// --- in ------------------------------------------------------------------------------------
		tl.cue('the program ends (credits)', 0);
		tl.cue('pull back', p0);
		tl.add(rim, [{ opacity: 0 }, { opacity: 1 }], p1 - 0.3, 0.6, 'sine');
		tl.cue('up next card', T.cardIn);
		tl.add(card, [{ opacity: 0 }, { opacity: 1 }], T.cardIn, 0.5, 'sine');
		labelIn(tl, lbl, T.typeIn);
		reveal(tl, [[time]], T.typeIn + 0.06, { dur: 0.8 });
		reveal(tl, mark.lines, T.typeIn + 0.1, { stagger: 0.07, dur: mark.logo ? 0.95 : 0.85 });
		if (ep) reveal(tl, [[ep]], T.typeIn + 0.36, { dur: 0.8 });

		// --- the handover --------------------------------------------------------------------------
		tl.cue(toIdent ? "handover: onto the short ident's couch" : 'handover: push into the card', h0);
		tl.add(rim, [{ opacity: 1 }, { opacity: 0 }], h0, 0.3, 'sine');
		if (toIdent) {
			// the stage settles into the ident's black field under the collapse
			const black = css(div('fill', root), { background: BRAND.night });
			root.insertBefore(black, prog);
			tl.add(black, [{ opacity: 0 }, { opacity: 1 }], h0, 0.9, 'sine');
			tl.add(room, [{ opacity: 1 }, { opacity: 0 }], h0, 0.6, 'sine');
			tl.add(card, [{ opacity: 1 }, { opacity: 0 }], h0, 0.4, 'leave');
			if (b) tl.add(b, [{ opacity: 1 }, { opacity: 0 }], h0, 0.4, 'sine');
			// the short ident's first frame, drawn as the ident draws it (pixel for pixel)
			const couch = new Ending(root, SHORT_IDENT_TIMES);
			couch.show(couch.key(0), false);
			root.insertBefore(couch.canvas, prog);
			windows(tl, couch.canvas, [[handed, D + 1]]);
		} else {
			labelOut(tl, lbl, h0);
			conceal(tl, [[time], ...mark.lines, ...(ep ? [[ep]] : [])], h0 + 0.03, { stagger: 0.03 });
			tl.add(cardGrade, [{ opacity: 1 }, { opacity: 0 }], q0, 0.8, 'sine');
			if (after) tl.add(after, [{ opacity: 0 }, { opacity: 1 }], 9.9, q1 - 9.9, 'sine');
			// up next's first frame: the same view of the same scenes, a frame after the card is it
			const view = div('fill oa-view', root);
			root.insertBefore(view, after);
			const cam = scenes(tl, view, nx.title, assets, {
				start: q1,
				end: D,
				offset: q1 - T.cardIn,
				crop
			});
			cam.style.transform = `scale(${Z})`;
			windows(tl, view, [[q1, D + 1]]);
			tl.show(card, 0, q1);
			tl.cue('up next', D);
		}

		// --- sound -------------------------------------------------------------------------------
		tl.sound(
			0,
			(w, bus, skip) =>
				sound.bed(bus, w, bed, skip, {
					fadeIn: 0.4,
					// into the short ident's silence, or under the push until up next's own bed
					fadeOut: toIdent ? 0.8 : D - q0,
					end: toIdent ? h0 + 0.8 : D,
					// down 6 dB as the card arrives, until the handover
					ducks: [{ at: 2.4 + 0.7, depth: 0.5, attack: 0.7, hold: D }]
				}),
			D
		);
		tl.sound(p0, (w, bus) => roomSounds(sound, bus, w, toIdent));
		return tl;
	}
};

/** The room's punctuation, from the pull back: air under it, a glass note as the card settles. */
function roomSounds(k: Kit, b: Bus, w: number, toIdent: boolean) {
	const h = house(k, b);
	const [p0, p1] = T.pull;
	air(h, w, p1 - p0 + 0.6, {
		peakAt: 1.0,
		band: [350, 2200],
		pans: [0.3, -0.3],
		gain: 0.55,
		seed: 801
	});
	glass(h, w + T.cardIn - p0, 'E5', {
		decay: 2.4,
		tone: 0.8,
		gain: 0.5,
		pan: 0.35,
		verb: 0.7,
		seed: 802
	});
	// the push breathes in, its air (and the hall's tail of it) settling before the last frame
	if (toIdent) return;
	const peak = T.push[1] - 0.3;
	air(h, w + T.expand - p0, 11.52 - T.expand - END, {
		peakAt: peak - T.expand,
		band: [500, 3200],
		pans: [0.35, 0],
		gain: 0.6,
		seed: 803
	});
	ringOut(h, w + peak - p0, 11.52 - END - peak, { depth: 10, close: 0.2 });
}
