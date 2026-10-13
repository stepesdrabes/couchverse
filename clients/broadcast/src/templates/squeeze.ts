// The credits squeeze: the program's last seconds shrink into a panel while "Up next" fills
// the rest, then hand over (the next picture expands, or the program shrinks onto the couch in
// its place in the lockup and dissolves into it, over the black field: the short ident's first
// frame).
//
// The panel's motion is data (segments of translate/scale and opacity) that drives both the
// animations and `keyRect(t)`, so the rectangle a compositor needs is exact for every frame.
// What fills the panel is the spec's `program`: the program's own frames, a stand-in (the
// ending title's backdrop under a credits roll), or nothing (`key`: the panel is transparent,
// showing the background behind it, and the `matte` layer carries the program's alpha).

import { SPARKLE } from '../audio/kit';
import { Ending, STAGE } from '../brand/ending';
import { COUCH_BOX } from '../brand/glyph';
import { bug } from '../brand/lockup';
import { SHORT_IDENT_TIMES } from '../brand/short-ident';
import { BRAND } from '../brand/wordmark';
import { css, div, fadeOut, fadeUp, img, rise, span, words } from '../core/dom';
import { fitText } from '../core/fit';
import { framed, W } from '../core/framing';
import { bezierPoints, cubicBezierAt, easing, type Ease } from '../core/motion';
import { edge, windows } from '../core/moves';
import { applyPalette } from '../core/palette';
import type { KeyRect } from '../core/timeline';
import type { SqueezeSpec } from '../spec';
import { glow, timeline, titlePalette, type Template, type TL } from './shared';

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

export const squeeze: Template<SqueezeSpec> = {
	version: 6,
	build({ root, sound, assets }, spec) {
		// `cursor` is this handover's old name, which squeezes composed before the rename carry
		const toIdent = spec.handover !== 'expand';
		const D = toIdent ? 10.6 : 11.52;
		const tl = timeline(root, D);
		root.classList.add('squeeze');
		const nx = spec.next;
		applyPalette(root, titlePalette(nx.title));
		const bed = assets.bed(spec.bed);

		const gV = glow(root, 'violet', 400, 160, 760);
		const gT = glow(root, 'teal', 700, 1100, 720);
		const gA = glow(root, 'accent', 1500, 560, 720);

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
		const frame = css(div('sq-frame', root), {
			left: '96px',
			top: '236px',
			width: '1008px',
			height: '567px'
		});

		const col = div('sq-col', root);
		const ebRow = css(div('un-eyebrow', col), { marginBottom: '0' });
		const rail = css(div('rail', ebRow), { width: '48px' });
		const eb = div('eyebrow', ebRow, 'Up next');
		const slot = div('sq-card-slot', col);
		const title = words(div('hero sq-title', col), nx.title.name);
		fitText(title.root.parentElement as HTMLElement, { width: 648, min: 54, lines: 3 });
		const epRow = nx.episode ? div('sq-ep', col) : null;
		const chip = epRow ? span('chip solid', epRow, nx.episode?.code) : null;
		const epName = epRow ? span(null, epRow, nx.episode?.name) : null;
		// beside the chip, within the column
		if (chip && epName)
			fitText(epName, { width: 630 - chip.offsetWidth, min: 24, lines: 2, leading: 1.15 });
		const whenEl = div('sq-when', col);
		span('tnum when-time', whenEl, nx.time);
		whenEl.append(' · after the break');
		const b = spec.clock ? bug(tl, root, spec.clock, { scrim: true }) : null;

		// the next program's picture lives at stage level so it can expand to full frame
		let slotX = 0;
		let slotY = 0;
		for (
			let e: HTMLElement | null = slot;
			e && e !== root;
			e = e.offsetParent as HTMLElement | null
		) {
			slotX += e.offsetLeft;
			slotY += e.offsetTop;
		}
		const k = 648 / W;
		const tvEl = div('sq-tv', root);
		const nshot = framed(tvEl, assets.picture(nx.title.backdrop));
		const slotTf = `translate(${slotX}px, ${slotY}px) scale(${k})`;
		tvEl.style.transform = slotTf;

		const full: Box = { x: -48, y: -27, sx: 1.05, sy: 1.05 };
		const sq: Box = { x: 96, y: 236, sx: 0.525, sy: 0.525 };
		const s0 = 2.0;
		const s1 = 3.3;
		const h0 = 9.2;
		const box = new Track<Box>(full, mixBox).to(s0, s1 - s0, sq, 'cubic-bezier(0.65, 0, 0.25, 1)');
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
			box.to(h0, 0.8, { x: -40, y: 236, sx: 0.525, sy: 0.525 }, 'cubic-bezier(0.6, 0, 0.8, 0.5)');
			opacity.to(h0, 0.8, 0, 'cubic-bezier(0.6, 0, 0.8, 0.5)');
		}
		tl.add(prog, [{ transform: tf(full) }, { transform: tf(full) }], 0, 0.01, 'linear');
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

		tl.cue('program ends (credits)', 0);
		tl.cue('squeeze', s0);
		tl.add(frame, [{ opacity: 0 }, { opacity: 1 }], s1 - 0.2, 0.5, 'sine');
		for (const g of [gV, gT, gA])
			tl.add(g, [{ opacity: 0 }, { opacity: 1 }], s0 + 0.3, 1.6, 'sine');
		if (b) fadeUp(tl, b, s0 + 0.6, { y: -10 });

		const c0 = s0 + 0.75;
		tl.cue('up next panel', c0);
		tl.add(rail, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], c0, 0.7, 'enter');
		tl.add(
			eb,
			[
				{ opacity: 0, transform: 'translateX(-14px)' },
				{ opacity: 1, transform: 'translateX(0)' }
			],
			c0 + 0.1,
			0.7,
			'enter'
		);
		tl.add(tvEl, [{ opacity: 0 }, { opacity: 1 }], c0 + 0.2, 0.8, 'sine');
		tl.add(
			nshot.el,
			[{ transform: 'scale(1.1)' }, { transform: 'scale(1)' }],
			c0 + 0.2,
			1.4,
			'enter'
		);
		tl.add(
			nshot.img,
			[{ transform: nshot.tf([0, 0, 1, 1]) }, { transform: nshot.tf([0.06, 0.04, 0.9, 0.9]) }],
			c0,
			D - c0,
			'linear'
		);
		rise(tl, title.items, c0 + 0.45, { stagger: 0.08, dur: 0.95 });
		if (chip && epName) {
			tl.add(
				chip,
				[
					{ opacity: 0, transform: 'scale(0.7)' },
					{ opacity: 1, transform: 'scale(1)' }
				],
				c0 + 0.8,
				0.5,
				'pop'
			);
			tl.add(
				epName,
				[
					{ opacity: 0, transform: 'translateX(-16px)' },
					{ opacity: 1, transform: 'translateX(0)' }
				],
				c0 + 0.9,
				0.7,
				'enter'
			);
		}
		fadeUp(tl, whenEl, c0 + 1.05, { y: 10 });

		tl.cue(
			toIdent ? "handover: onto the short ident's couch" : 'handover: the next program expands',
			h0
		);
		[ebRow, title.root.parentElement as HTMLElement, epRow, whenEl].forEach((e, i) => {
			if (e) fadeOut(tl, e, h0 + i * 0.05, { dur: 0.4, y: -10 });
		});
		tl.add(frame, [{ opacity: 1 }, { opacity: 0 }], h0, 0.3, 'sine');
		if (toIdent) {
			// the stage settles into the ident's black field under the collapse
			const black = css(div('fill', root), { background: BRAND.night });
			root.insertBefore(black, prog);
			tl.add(black, [{ opacity: 0 }, { opacity: 1 }], h0, 0.9, 'sine');
			tl.add(tvEl, [{ opacity: 1 }, { opacity: 0 }], h0, 0.45, 'sine');
			if (b) fadeOut(tl, b, h0, { dur: 0.4, y: -10 });
			for (const g of [gV, gT, gA]) tl.add(g, [{ opacity: 1 }, { opacity: 0 }], h0, 0.9, 'sine');
			// the short ident's first frame, drawn as the ident draws it (pixel for pixel)
			const couch = new Ending(root, SHORT_IDENT_TIMES);
			couch.show(couch.key(0), false);
			root.insertBefore(couch.canvas, prog);
			windows(tl, couch.canvas, [[handed, D + 1]]);
			tl.sound(h0 + 0.1, (w, bus) =>
				sound.whoosh(bus, w, { dur: 0.9, pan: [-0.5, 0], gain: 0.05 })
			);
		} else {
			tl.add(
				tvEl,
				[{ transform: slotTf }, { transform: 'translate(-57.6px, -32.4px) scale(1.06)' }],
				h0 + 0.35,
				1.25,
				'cubic-bezier(0.7, 0, 0.2, 1)'
			);
			tl.sound(h0 + 0.3, (w, bus) =>
				sound.whoosh(bus, w, { dur: 1.2, pan: [0.6, -0.1], gain: 0.07 })
			);
			tl.sound(h0 + 1.5, (w, bus) => sound.impact(bus, w, { gain: 0.15, size: 0.7, click: 0.2 }));
		}

		// the credits music keeps playing, ducked while "Up next" talks over it
		tl.sound(
			0,
			(w, bus, skip) =>
				sound.bed(bus, w, bed, skip, {
					fadeIn: 0.4,
					fadeOut: 1.4,
					end: h0 + 1.5,
					gain: 1,
					ducks: [{ at: s0 + 0.5, depth: 0.42, hold: h0 - s0 - 0.5, attack: 0.7, release: 0.01 }]
				}),
			h0 + 1.5
		);
		tl.sound(s0, (w, bus) =>
			sound.whoosh(bus, w, { dur: 1.3, pan: [0.4, -0.5], gain: 0.05, lo: 300, hi: 1800 })
		);
		tl.sound(c0, (w, bus) => sound.tick(bus, w, { freq: SPARKLE[1], gain: 0.04, pan: 0.5 }));
		tl.sound(c0 + 0.2, (w, bus) =>
			sound.shimmer(bus, w, { dur: 0.9, gain: 0.018, count: 8, seed: 41 })
		);
		if (chip)
			tl.sound(c0 + 0.8, (w, bus) =>
				sound.tick(bus, w, { freq: SPARKLE[3], gain: 0.035, pan: 0.55 })
			);
		return tl;
	}
};
