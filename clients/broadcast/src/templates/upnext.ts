// "Up next": the next program's picture, its title and a countdown ring into it.

import { SPARKLE } from '../audio/kit';
import { bug } from '../brand/lockup';
import { css, div, fadeOut, fadeUp, fit, rise, span, svg, words } from '../core/dom';
import { framed } from '../core/framing';
import { applyPalette } from '../core/palette';
import type { UpNextSpec } from '../spec';
import { timeline, titlePalette, type Template } from './shared';

export const upNext: Template<UpNextSpec> = {
	version: 2,
	build({ root, sound, assets }, spec) {
		const t = spec.title;
		const D = spec.duration ?? 8;
		const tl = timeline(root, D);
		applyPalette(root, titlePalette(t));
		root.classList.add('upnext');
		const continuing = spec.enter === 'continue';
		const bed = assets.bed(spec.bed);

		const shot = framed(root, assets.picture(t.backdrop));
		const fr = t.crops?.upNext ?? [
			[0, 0, 1, 1],
			[0.08, 0.04, 0.88, 0.88]
		];
		tl.add(
			shot.img,
			[{ transform: shot.tf(fr[0], continuing ? 1.06 : 1) }, { transform: shot.tf(fr[1]) }],
			0,
			D,
			'cubic-bezier(0.3, 0.2, 0.6, 1)'
		);
		const scrimL = div('fill scrim-left', root);
		const scrimB = div('fill scrim-bottom', root);
		if (!continuing) tl.add(shot.el, [{ opacity: 0 }, { opacity: 1 }], 0, 1.0, 'sine');
		tl.add(scrimL, [{ opacity: 0 }, { opacity: 0.85 }], 0.1, 1.0, 'sine');
		tl.add(scrimB, [{ opacity: 0 }, { opacity: 1 }], 0.1, 1.0, 'sine');
		if (spec.clock) fadeUp(tl, bug(tl, root, spec.clock), 0.5, { y: -10 });

		const info = div('un-info', root);
		const ebRow = div('un-eyebrow', info);
		const rail = div('rail', ebRow);
		const eb = div('eyebrow', ebRow, 'Up next');
		const title = words(div('hero un-title', info), t.name);
		fit(title.root.parentElement as HTMLElement, 1100);
		const epRow = spec.episode ? div('un-ep', info) : null;
		const chip = epRow ? span('chip solid', epRow, spec.episode?.code) : null;
		const epName = epRow ? span(null, epRow, spec.episode?.name) : null;
		const meta = div('un-meta', info);
		const runtime = spec.episode?.runtimeMinutes ?? t.runtimeMinutes;
		const metaItems = [
			spec.time,
			...(spec.episode ? [] : t.year ? [String(t.year)] : []),
			...t.genres.slice(0, 2),
			...(runtime ? [`${runtime} min`] : [])
		];
		metaItems.forEach((x, i) => span(i ? 'dot-sep' : 'tnum', meta, x));

		const cd = div('countdown', root);
		div('cd-disc', cd);
		const lbl = div('cd-label', cd);
		div('eyebrow', lbl, 'Starts in');
		const ringBox = css(div(null, cd), { position: 'relative', width: '168px', height: '168px' });
		const C = 2 * Math.PI * 78;
		const ring = svg('svg', { class: 'ring-svg', viewBox: '0 0 168 168' }, ringBox);
		svg('circle', { class: 'track', cx: 84, cy: 84, r: 78 }, ring);
		const prog = svg(
			'circle',
			{ class: 'prog', cx: 84, cy: 84, r: 78, 'stroke-dasharray': C.toFixed(1) },
			ring
		);
		const num = div('ring-num tnum', ringBox);
		const line = div('progress-line', root);

		const T0 = continuing ? 0.25 : 0.45;
		tl.cue('eyebrow + rail', T0);
		tl.add(rail, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], T0, 0.75, 'enter');
		tl.add(
			eb,
			[
				{ opacity: 0, transform: 'translateX(-14px)' },
				{ opacity: 1, transform: 'translateX(0)' }
			],
			T0 + 0.15,
			0.8,
			'enter'
		);
		tl.cue('title', T0 + 0.35);
		rise(tl, title.items, T0 + 0.35, { stagger: 0.09, dur: 1.0 });
		if (chip && epName) {
			tl.cue('episode', T0 + 0.9);
			tl.add(
				chip,
				[
					{ opacity: 0, transform: 'scale(0.7)' },
					{ opacity: 1, transform: 'scale(1)' }
				],
				T0 + 0.9,
				0.5,
				'pop'
			);
			tl.add(
				epName,
				[
					{ opacity: 0, transform: 'translateX(-20px)' },
					{ opacity: 1, transform: 'translateX(0)' }
				],
				T0 + 1.0,
				0.8,
				'enter'
			);
		}
		fadeUp(tl, meta, T0 + 1.2, { y: 12 });

		tl.cue('countdown ring', T0 + 0.6);
		tl.add(
			cd,
			[
				{ opacity: 0, transform: 'scale(0.86)' },
				{ opacity: 1, transform: 'scale(1)' }
			],
			T0 + 0.6,
			0.8,
			'enter'
		);
		tl.add(prog, [{ strokeDashoffset: C }, { strokeDashoffset: 0 }], T0 + 0.6, 0.7, 'enter');
		tl.add(
			prog,
			[{ strokeDashoffset: 0 }, { strokeDashoffset: C }],
			T0 + 1.3,
			D - T0 - 1.3,
			'linear'
		);
		tl.add(line, [{ transform: 'scaleX(0)' }, { transform: 'scaleX(1)' }], 0, D, 'linear');
		tl.add(
			num,
			[
				{ transform: 'none', opacity: 1 },
				{ transform: 'none', opacity: 1 }
			],
			0,
			0.01,
			'linear'
		);
		for (let s = 1; s < D; s++)
			tl.add(
				num,
				[
					{ transform: 'translateY(12px)', opacity: 0.25 },
					{ transform: 'translateY(0)', opacity: 1 }
				],
				s,
				0.4,
				'enter'
			);
		tl.text(num, (tt) => `0:${String(Math.max(0, Math.ceil(D - tt - 1e-3))).padStart(2, '0')}`);

		const out = D - 0.9;
		tl.cue('handover', out);
		fadeOut(tl, info, out, { dur: 0.6, x: -40, y: 0 });
		fadeOut(tl, cd, out + 0.05, { dur: 0.55, y: 0, scale: 0.9 });
		tl.add(scrimL, [{ opacity: 0.85 }, { opacity: 0.2 }], out, 0.9, 'sine');
		tl.add(scrimB, [{ opacity: 1 }, { opacity: 0.4 }], out, 0.9, 'sine');

		tl.sound(
			0,
			(w, b, skip) =>
				sound.bed(b, w, bed, skip, { fadeIn: 0.35, fadeOut: 1.3, end: D, gain: 0.95 }),
			D
		);
		tl.sound(T0 - 0.05, (w, b) =>
			sound.whoosh(b, w, { dur: 0.75, pan: [-0.7, -0.2], gain: 0.05, lo: 600, hi: 3200, body: 0.2 })
		);
		tl.sound(T0 + 0.35, (w, b) =>
			sound.impact(b, w, { gain: 0.16, size: 0.6, tone: 52, click: 0.3 })
		);
		if (chip)
			tl.sound(T0 + 0.9, (w, b) => sound.tick(b, w, { freq: SPARKLE[0], gain: 0.045, pan: -0.5 }));
		for (const s of [D - 3, D - 2, D - 1])
			tl.sound(s, (w, b) =>
				sound.tick(b, w, {
					freq: s === D - 1 ? SPARKLE[4] : SPARKLE[2],
					gain: 0.04,
					pan: 0.6,
					decay: 0.09
				})
			);
		tl.sound(D - 1.6, (w, b) => sound.riser(b, w, { dur: 1.5, gain: 0.035, tone: 0.3 }));
		return tl;
	}
};
