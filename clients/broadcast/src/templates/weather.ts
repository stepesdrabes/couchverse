// The weather (DESIGN.md 2.10, sharp): Czechia as a deep red country of kraje on the night,
// every configured city's sky and temperature where the city is, ČHMÚ's warnings filling the
// kraje they cover. Three scenes, each still once it has entered: now (the warnings beside the
// map), tomorrow (a red front crosses the country west to east and every city turns to
// tomorrow's sky as it passes) and the next days (a red bar sweeps the map away into a table).
// English chrome; city names as configured. The music is the house pad on D with glass at
// every change of scene and air under the sweeps, all of it over before the clip ends.

import { air, glass, house, pad, padSweep, piano, shimmer, sub } from '../audio/voices';
import { BRAND } from '../brand/wordmark';
import { css, div, span, svg } from '../core/dom';
import { fitText } from '../core/fit';
import { czechiaMap, type KrajShape } from '../core/geo';
import { bezierPoints, cubicBezierAt } from '../core/motion';
import type { KrajCode, WeatherSpec, WeatherWarning } from '../spec';
import {
	BEAT,
	bulletinHouse,
	credits,
	exit,
	field,
	head,
	headIn,
	headOut,
	mark,
	onBeat,
	SAFE_RIGHT
} from './bulletin';
import { timeline, type Template } from './shared';
import { weatherIcon } from './weather-icons';

/** `14°`, with a true minus sign below zero. */
export const degrees = (t: number) => `${t < 0 ? '−' : ''}${Math.abs(t)}°`;

/** `12–16°`, or `−3° to 2°` where a minus would read as the dash. */
export function range(values: number[]) {
	const [lo, hi] = [Math.min(...values), Math.max(...values)];
	if (lo === hi) return degrees(lo);
	return lo < 0 ? `${degrees(lo)} to ${degrees(hi)}` : `${lo}–${hi}°`;
}

const when = (t: { day?: string; time: string }) => (t.day ? `${t.day} ${t.time}` : t.time);

/** `Now until Wednesday 02:00`, `From 16:00 until further notice`. */
export function warningTime(w: WeatherWarning): string {
	const until = w.until ? `until ${when(w.until)}` : 'until further notice';
	return w.active ? `Now ${until}` : `From ${when(w.from)} ${until}`;
}

const list = (names: string[]) =>
	names.length < 2 ? names.join('') : `${names.slice(0, -1).join(', ')} and ${names.at(-1)}`;

/**
 * Where a warning is, in ČHMÚ's English kraj names: the whole kraje, then the ones it covers in
 * part (`Vysočina Region and parts of the South Bohemian and Olomouc Regions`).
 */
export function regionsPhrase(w: WeatherWarning): string {
	const whole = w.regions.filter((r) => !r.partial).map((r) => r.name);
	const parts = w.regions.filter((r) => r.partial).map((r) => r.name);
	const regions = parts.every((n) => n.endsWith(' Region'));
	let some = '';
	if (parts.length === 1) some = `${regions ? 'parts of the' : 'parts of'} ${parts[0]}`;
	else if (parts.length > 1)
		some = regions
			? `parts of the ${list(parts.map((n) => n.slice(0, -' Region'.length)))} Regions`
			: `parts of ${list(parts)}`;
	return [list(whole), some].filter(Boolean).join(' and ');
}

const LEVELS = ['yellow', 'orange', 'red'] as const;
const LEVEL_NAME = { yellow: 'Yellow warning', orange: 'Orange warning', red: 'Red warning' };
const LEVEL_COLOUR = { yellow: '#f5c518', orange: '#f57c00', red: BRAND.red };

/** Each warned kraj at its most severe level, partial when no warning at it covers it whole. */
export function warnedKraje(warnings: WeatherWarning[]) {
	const out = new Map<KrajCode, { level: WeatherWarning['level']; partial: boolean }>();
	for (const w of warnings)
		for (const r of w.regions) {
			const had = out.get(r.code);
			const rank = LEVELS.indexOf(w.level);
			if (!had || rank > LEVELS.indexOf(had.level))
				out.set(r.code, { level: w.level, partial: r.partial });
			else if (had.level === w.level && !r.partial) had.partial = false;
		}
	return out;
}

// the warnings the card has room for; the rest are counted
const WARNINGS_SHOWN = 2;

// the map: the country 1184 px wide from x 640, 24 px under the middle (DESIGN.md 2.10)
const MAP = { x: 640, y: 221, width: 1184, height: 679 };
// what the sweeps cross: right of the left column, above the credits
const REGION = { x: 560, y: 140, width: 1360, height: 835 };
// the next days' wipe: a red bar this wide pushing the map out
const WIPE = 96;
const ICON = 64;

type Box = { l: number; t: number; r: number; b: number };
const overlap = (a: Box, b: Box) =>
	Math.max(0, Math.min(a.r, b.r) - Math.max(a.l, b.l)) *
	Math.max(0, Math.min(a.b, b.b) - Math.max(a.t, b.t));
const SAFE: Box = { l: 96, t: 64, r: SAFE_RIGHT, b: 1016 };
const outside = (a: Box) =>
	Math.max(0, SAFE.l - a.l) +
	Math.max(0, a.r - SAFE.r) +
	Math.max(0, SAFE.t - a.t) +
	Math.max(0, a.b - SAFE.b);

interface City {
	x: number;
	y: number;
	name: HTMLElement;
	/** Per scene: the icon's wrapper and the temperature's clip. */
	now: { icon: HTMLElement; temp: HTMLElement; clip: HTMLElement };
	next?: { icon: HTMLElement; temp: HTMLElement; clip: HTMLElement };
}

/**
 * Labels without collisions, greedily in the configured order (DESIGN.md's `placeLabels`): the
 * icons are fixed first; each name goes under its icon, else above; then for each scene each
 * temperature goes right of its icon, else left. A place that leaves the title-safe area or
 * covers what is placed loses to one that does not; when none is clear, the least covered wins.
 */
function placeLabels(cities: City[]) {
	const icons: Box[] = cities.map((c) => ({ l: c.x - 30, t: c.y - 30, r: c.x + 30, b: c.y + 28 }));
	const placed: Box[] = [...icons];
	const cost = (bx: Box, own: Box) =>
		placed.filter((p) => p !== own).reduce((a, p) => a + overlap(bx, p), 0) + outside(bx) * 1000;
	cities.forEach((c, i) => {
		const w = c.name.offsetWidth;
		const h = c.name.offsetHeight;
		const below = { l: c.x - w / 2, t: c.y + 34, r: c.x + w / 2, b: c.y + 34 + h };
		const above = { l: c.x - w / 2, t: c.y - 40 - h, r: c.x + w / 2, b: c.y - 40 };
		const pick = cost(below, icons[i]) <= cost(above, icons[i]) ? below : above;
		css(c.name, { left: `${pick.l}px`, top: `${pick.t}px` });
		placed.push(pick);
	});
	for (const scene of ['now', 'next'] as const) {
		const mine: Box[] = [];
		cities.forEach((c, i) => {
			const s = c[scene];
			if (!s) return;
			const w = s.temp.offsetWidth;
			const right = { l: c.x + 38, t: c.y - 26, r: c.x + 38 + w, b: c.y + 22 };
			const left = { l: c.x - 38 - w, t: c.y - 26, r: c.x - 38, b: c.y + 22 };
			const others = [...placed, ...mine];
			const score = (bx: Box) =>
				others.filter((p) => p !== icons[i]).reduce((a, p) => a + overlap(bx, p), 0) +
				outside(bx) * 1000;
			const pick = score(right) <= score(left) ? right : left;
			// the clip holds the label with room for its rise
			css(s.clip, { left: `${pick.l - 8}px`, top: `${pick.t - 10}px`, width: `${w + 16}px` });
			mine.push(pick);
		});
	}
}

/** When a sweep that moves from `x0` to `x1` from `at` over `dur` on `ease` passes x. */
function passes(x: number, x0: number, x1: number, at: number, dur: number, ease: string) {
	const p = bezierPoints(ease);
	const want = (x - x0) / (x1 - x0);
	let [lo, hi] = [0, 1];
	for (let i = 0; i < 40; i++) {
		const mid = (lo + hi) / 2;
		if (cubicBezierAt(p, mid) < want) lo = mid;
		else hi = mid;
	}
	return at + ((lo + hi) / 2) * dur;
}

/** A window over the sweeps' region whose content stays put while the window moves. */
function sweepWindow(parent: HTMLElement) {
	const win = css(div('wx-win', parent), {
		left: `${REGION.x}px`,
		top: `${REGION.y}px`,
		width: `${REGION.width}px`,
		height: `${REGION.height}px`
	});
	const inner = css(div('wx-in', win), {
		left: `${-REGION.x}px`,
		top: `${-REGION.y}px`,
		width: '1920px',
		height: '1080px'
	});
	return { win, inner };
}

let gradients = 0;

export const weather: Template<WeatherSpec> = {
	version: 2,
	build({ root, sound }, spec) {
		const D = spec.duration ?? 15;
		const tl = timeline(root, D);
		root.classList.add('bulletin', 'weather');
		field(tl, root, { x: 1240, y: 1220, rx: 1500, ry: 560, a: 0.75 });

		// the scenes, on the beat: now, then tomorrow where there is room, then the next days
		const days = Math.min(...spec.cities.map((c) => c.days.length));
		const table = days >= 2;
		const tomorrow = table && D >= 13;
		const T1 = tomorrow ? onBeat(D * 0.32) : D;
		const T2 = table ? onBeat(D * (tomorrow ? 0.64 : 0.45)) : D;
		const first = spec.cities[0];

		// --- the map ---------------------------------------------------------------------------
		const map = czechiaMap(MAP);
		const { win: mapWin, inner: world } = sweepWindow(root);
		const defs = svg('svg', { width: 0, height: 0, class: 'wx-defs' }, world);
		css(defs as unknown as HTMLElement, { position: 'absolute' });
		const land = `wx-land-${++gradients}`;
		const g = svg(
			'linearGradient',
			{
				id: land,
				gradientUnits: 'userSpaceOnUse',
				x1: 0,
				y1: map.box.y,
				x2: 0,
				y2: map.box.y + map.box.height
			},
			svg('defs', {}, defs)
		);
		svg('stop', { offset: 0, 'stop-color': '#560610' }, g);
		svg('stop', { offset: 1, 'stop-color': '#260306' }, g);

		const layer = (cls: string, box: { x: number; y: number; width: number; height: number }) => {
			const pad = 6;
			const [x, y, w, h] = [box.x - pad, box.y - pad, box.width + 2 * pad, box.height + 2 * pad];
			const holder = css(div(cls, world), {
				left: `${x}px`,
				top: `${y}px`,
				width: `${w}px`,
				height: `${h}px`
			});
			const s = svg(
				'svg',
				{ width: w.toFixed(1), height: h.toFixed(1), viewBox: `${x} ${y} ${w} ${h}` },
				holder
			);
			css(s as unknown as HTMLElement, { display: 'block', overflow: 'visible' });
			return { holder, s };
		};
		// the kraje rise into place west to east, the borders drawn over them once they are in
		const kraje = [...map.kraje].sort(
			(a, b) => a.box.x + a.box.width / 2 - (b.box.x + b.box.width / 2)
		);
		kraje.forEach((k, i) => {
			const { holder, s } = layer('wx-kraj', k.box);
			svg('path', { d: k.path, fill: `url(#${land})` }, s);
			tl.add(
				holder,
				[
					{ opacity: 0, transform: 'translateY(26px)' },
					{ opacity: 1, transform: 'translateY(0px)' }
				],
				0.2 + i * 0.045,
				0.8,
				'glide'
			);
		});
		const lines = layer('wx-lines', map.box);
		svg('path', { d: map.inner, class: 'inner' }, lines.s);
		svg('path', { d: map.edge, class: 'edge' }, lines.s);
		tl.add(lines.holder, [{ opacity: 0 }, { opacity: 1 }], 0.2 + kraje.length * 0.045, 0.5, 'sine');

		// the warned kraje in their level's colour, hatched where only part of one is warned
		const warned = warnedKraje(spec.warnings ?? []);
		const tints: { holder: HTMLElement; k: KrajShape }[] = [];
		for (const k of kraje) {
			const w = warned.get(k.code);
			if (!w) continue;
			const { holder, s } = layer('wx-tint', k.box);
			const colour = LEVEL_COLOUR[w.level];
			let fill = colour;
			if (w.partial) {
				const id = `wx-hatch-${gradients}-${k.code}`;
				const pat = svg(
					'pattern',
					{
						id,
						patternUnits: 'userSpaceOnUse',
						width: 14,
						height: 14,
						patternTransform: 'rotate(45)'
					},
					svg('defs', {}, s)
				);
				svg('rect', { x: 0, y: 0, width: 4, height: 14, fill: colour }, pat);
				fill = `url(#${id})`;
			}
			// a kraj warned whole is outlined in the level's colour; one warned in part is hatched
			svg(
				'path',
				{
					d: k.path,
					fill,
					'fill-opacity': w.partial ? 0.5 : 0.34,
					...(w.partial ? {} : { stroke: colour })
				},
				s
			);
			tints.push({ holder, k });
		}

		// every city: its icon on its point, its temperature beside it, its name under it
		const cityEls: City[] = spec.cities.map((c) => {
			const [x, y] = map.point(c.lat, c.lon);
			const icon = (code: number, night: boolean) => {
				const w = css(div('wx-icon', world), {
					left: `${x - ICON / 2}px`,
					top: `${y - ICON / 2}px`,
					width: `${ICON}px`,
					height: `${ICON}px`
				});
				weatherIcon(w, code, { size: ICON, night });
				return w;
			};
			const temp = (hi: number, lo?: number) => {
				const clip = css(div('wx-win', world), { height: '62px' });
				const t = css(div('wx-temp tnum', clip), { left: '8px', top: '10px' });
				span('hi', t, degrees(hi));
				if (lo != null) span('lo', t, degrees(lo));
				return { clip, temp: t };
			};
			const name = div('wx-name', world, c.name);
			fitText(name, { width: 320, min: 16 });
			const now = { icon: icon(c.now.code, !c.now.day), ...temp(c.now.temperature) };
			const t1 = c.days[1];
			const next =
				tomorrow && t1 ? { icon: icon(t1.code, false), ...temp(t1.max, t1.min) } : undefined;
			return { x, y, name, now, next };
		});
		placeLabels(cityEls);
		const west = [...cityEls].sort((a, b) => a.x - b.x);
		west.forEach((c, i) => {
			const at = 0.75 + i * 0.045;
			tl.add(
				c.now.icon,
				[
					{ opacity: 0, transform: 'scale(0.82)' },
					{ opacity: 1, transform: 'scale(1)' }
				],
				at,
				0.55,
				'glide'
			);
			tl.add(
				c.now.temp,
				[{ transform: 'translateY(150%)' }, { transform: 'translateY(0)' }],
				at + 0.06,
				0.6,
				'glide'
			);
			tl.add(
				c.name,
				[
					{ opacity: 0, transform: 'translateY(8px)' },
					{ opacity: 1, transform: 'translateY(0px)' }
				],
				at + 0.1,
				0.5,
				'glide'
			);
		});
		const tintIn = 1.5;
		tints.forEach(({ holder }, i) =>
			tl.add(holder, [{ opacity: 0 }, { opacity: 1 }], tintIn + i * 0.06, 0.5, 'sine')
		);

		// --- the left column: the headers and the cards ----------------------------------------
		const hNow = head(root, 'Weather', ['Now', spec.asOf]);
		const hTom = tomorrow ? head(root, first.days[1].label, [first.days[1].weekday]) : null;
		const hDays = table ? head(root, 'Next days', ['Czechia']) : null;

		const cardClip = (top = 470) =>
			css(div('wx-win', root), { left: '96px', top: `${top}px`, width: '480px', height: '520px' });
		const card = (parent: HTMLElement, colour: string) => {
			const c = css(div('wx-card', parent), { left: '0px', top: '0px' });
			const bar = css(div('wx-card-bar', c), { background: colour });
			return { c, bar, body: div('wx-card-body', c) };
		};
		const rangeCard = (label: string | null, day: number, calm: boolean) => {
			const clip = cardClip();
			const { c, bar, body } = card(clip, BRAND.red);
			if (label) fitText(div('wx-range-label', body, label), { width: 420, min: 22 });
			for (const [k, pick] of [
				['Highs', (d: { max: number }) => d.max],
				['Lows', (d: { min: number }) => d.min]
			] as const) {
				const r = div('wx-range tnum', body);
				span('k', r, k);
				span('v', r, range(spec.cities.map((ct) => pick(ct.days[day]))));
			}
			if (calm) div('wx-calm', body, 'No weather warnings');
			return { clip, c, bar };
		};
		let nowCard: { clip: HTMLElement; c: HTMLElement; bar: HTMLElement };
		const warnings = spec.warnings ?? [];
		if (warnings.length) {
			const clip = cardClip();
			const c = css(div('wx-card', clip), { left: '0px', top: '0px' });
			const shown = warnings.slice(0, WARNINGS_SHOWN);
			const bars: HTMLElement[] = [];
			shown.forEach((w) => {
				const item = div(`wx-warning lv-${w.level}`, c);
				bars.push(css(div('wx-card-bar', item), { background: LEVEL_COLOUR[w.level] }));
				const body = div('wx-card-body', item);
				div('wx-level', body, LEVEL_NAME[w.level]);
				fitText(div('wx-event', body, w.event), { width: 442, min: 30, lines: 2, leading: 1.08 });
				fitText(div('wx-when tnum', body, warningTime(w)), { width: 442, min: 20 });
				fitText(div('wx-regions', body, regionsPhrase(w)), {
					width: 442,
					min: 18,
					lines: shown.length > 1 ? 2 : 4,
					leading: 1.36
				});
			});
			const more = warnings.length - shown.length;
			if (more > 0) div('wx-more', c, `+ ${more} more`);
			// two warnings stand higher, so the card ends above the credits
			css(clip, {
				top: `${Math.min(470, 944 - c.offsetHeight)}px`,
				height: `${c.offsetHeight + 24}px`
			});
			nowCard = { clip, c, bar: bars[0] };
			bars
				.slice(1)
				.forEach((b, i) =>
					tl.add(
						b,
						[{ transform: 'scaleY(0)' }, { transform: 'scaleY(1)' }],
						1.45 + 0.1 * (i + 1),
						0.5,
						'glide'
					)
				);
		} else nowCard = rangeCard('Today', 0, !!spec.warnings);
		const cardIn = (c: { c: HTMLElement; bar: HTMLElement }, at: number) => {
			tl.add(
				c.c,
				[{ transform: 'translateX(-105%)' }, { transform: 'translateX(0)' }],
				at,
				0.75,
				'glide'
			);
			tl.add(
				c.bar,
				[{ transform: 'scaleY(0)' }, { transform: 'scaleY(1)' }],
				at + 0.1,
				0.5,
				'glide'
			);
		};
		const cardOut = (c: { c: HTMLElement }, at: number) =>
			tl.add(
				c.c,
				[{ transform: 'translateX(0)' }, { transform: 'translateX(-105%)' }],
				at,
				0.4,
				'leave'
			);
		cardIn(nowCard, 1.35);

		// --- the credits and the corner ---------------------------------------------------------
		const mapCredits = [spec.credits.forecast];
		if (spec.warnings) mapCredits.push(spec.credits.warnings);
		mapCredits.push(spec.credits.borders);
		const credMap = credits(root, mapCredits);
		tl.add(credMap, [{ opacity: 0 }, { opacity: 1 }], 1.2, 0.6, 'sine');
		const corner = mark(tl, root, 0.3);
		headIn(tl, hNow, 0.15);

		// --- tomorrow: a front crosses the country, every city turning as it passes -------------
		const out: Element[] = [corner];
		let lastHead = hNow;
		let lastCard: { c: HTMLElement } = nowCard;
		if (tomorrow && hTom) {
			headOut(tl, hNow, T1);
			headIn(tl, hTom, T1 + 0.3);
			cardOut(nowCard, T1);
			const tomCard = rangeCard(null, 1, false);
			cardIn(tomCard, T1 + 0.4);
			const x0 = MAP.x - 40;
			const x1 = 1930;
			const sweep = 1.5;
			const front = css(div('wx-front', root), {
				left: `${x0}px`,
				top: `${MAP.y - 50}px`,
				width: '6px',
				height: `${MAP.height + 100}px`
			});
			tl.add(
				front,
				[{ transform: 'translateX(0px)' }, { transform: `translateX(${x1 - x0}px)` }],
				T1 + 0.1,
				sweep,
				'move'
			);
			tl.add(front, [{ opacity: 0 }, { opacity: 1 }], T1 + 0.1, 0.12, 'linear');
			tl.add(front, [{ opacity: 1 }, { opacity: 0 }], T1 + 0.1 + sweep - 0.12, 0.12, 'linear');
			for (const c of cityEls) {
				if (!c.next) continue;
				const at = passes(c.x, x0, x1, T1 + 0.1, sweep, 'move');
				tl.add(c.now.icon, [{ opacity: 1 }, { opacity: 0 }], at - 0.05, 0.2, 'sine');
				tl.add(
					c.now.temp,
					[{ transform: 'translateY(0)' }, { transform: 'translateY(-150%)' }],
					at - 0.05,
					0.28,
					'leave'
				);
				tl.add(
					c.next.icon,
					[
						{ opacity: 0, transform: 'scale(0.82)' },
						{ opacity: 1, transform: 'scale(1)' }
					],
					at + 0.05,
					0.5,
					'glide'
				);
				tl.add(
					c.next.temp,
					[{ transform: 'translateY(150%)' }, { transform: 'translateY(0)' }],
					at + 0.1,
					0.55,
					'glide'
				);
			}
			// the warnings are today's: each warned kraj clears as the front reaches its middle
			for (const { holder, k } of tints) {
				const at = passes(k.box.x + k.box.width / 2, x0, x1, T1 + 0.1, sweep, 'move');
				tl.add(holder, [{ opacity: 1 }, { opacity: 0 }], at - 0.1, 0.35, 'sine');
			}
			lastHead = hTom;
			lastCard = tomCard;
		}

		// --- the next days: a red bar sweeps the map away and leaves the table ------------------
		if (table && hDays) {
			headOut(tl, lastHead, T2);
			headIn(tl, hDays, T2 + 0.3);
			cardOut(lastCard, T2);
			const { win: tWin, inner: tIn } = sweepWindow(root);
			const tbl = div('wx-table', tIn);
			const cols = Math.min(days, 5);
			const colW = 192;
			const nameW = 1224 - cols * colW;
			const thead = div('wx-thead', tbl);
			css(div(null, thead), { width: `${nameW}px` });
			first.days.slice(0, cols).forEach((d, i) => {
				const h = css(div(i ? null : 'today', thead, i ? d.short : d.label), {
					width: `${colW}px`,
					paddingLeft: '8px'
				});
				fitText(h, { width: colW - 12, min: 16 });
			});
			div('wx-rule', tbl);
			const rows = Math.min(spec.cities.length, 14);
			const rowH = Math.min(58, Math.floor(706 / rows));
			const iconSize = Math.round(rowH * 0.8);
			for (const c of spec.cities.slice(0, rows)) {
				const r = css(div('wx-row', tbl), { height: `${rowH}px` });
				const n = css(div('city', r, c.name), { width: `${nameW}px` });
				fitText(n, { width: nameW - 16, min: 16 });
				c.days.slice(0, cols).forEach((d) => {
					const cell = css(div('wx-cell', r), { width: `${colW}px` });
					weatherIcon(cell, d.code, { size: iconSize });
					span('hi tnum', cell, degrees(d.max));
					span('lo tnum', cell, degrees(d.min));
				});
			}
			// a short table stands on the map's middle rather than under the header
			tbl.style.top = `${Math.max(186, Math.round(MAP.y + MAP.height / 2 - tbl.offsetHeight / 2))}px`;
			const credDays = credits(root, [spec.credits.forecast]);
			tl.add(credMap, [{ opacity: 1 }, { opacity: 0 }], T2 + 0.2, 0.35, 'sine');
			tl.add(credDays, [{ opacity: 0 }, { opacity: 1 }], T2 + 0.75, 0.5, 'sine');

			// the bar's right edge runs from the region's left edge until the bar has left it:
			// the map shows right of the bar, the table left of it
			const at = T2 + 0.15;
			const dur = 1.3;
			const travel = REGION.width + WIPE;
			const wipeWin = sweepWindow(root).win;
			const bar = css(div('wx-front', wipeWin), {
				left: `${-WIPE}px`,
				top: '0px',
				width: `${WIPE}px`,
				height: `${REGION.height}px`
			});
			const slide = (e: Element, from: number, to: number) =>
				tl.add(
					e,
					[{ transform: `translateX(${from}px)` }, { transform: `translateX(${to}px)` }],
					at,
					dur,
					'move'
				);
			slide(bar, 0, travel);
			slide(mapWin, 0, travel);
			slide(world, 0, -travel);
			slide(tWin, -travel, 0);
			slide(tIn, travel, 0);
			out.push(tWin, credDays);
			lastHead = hDays;
		} else out.push(mapWin, credMap, lastCard.c);
		out.push(lastHead.root);
		exit(tl, out);

		tl.cue('now', 0);
		if (tomorrow) tl.cue('tomorrow', T1);
		if (table) tl.cue('next days', T2);

		// --- the sound: the pad on D, glass at each scene, air under the sweeps -----------------
		tl.sound(
			0,
			(when, b, skip) => {
				const w = when - skip;
				const h = bulletinHouse(sound, b, w, D);
				// the last chord holds into the fade
				const end = D - 1.2;
				padSweep(h, w, [
					[0, 700],
					[1.6, 2600],
					[Math.max(2, T1 - 0.4), 2200],
					[Math.min(end, T1 + 1.2), 3400],
					[Math.max(Math.min(end, T1 + 1.3), T2 - 0.4), 2400],
					[Math.min(end, T2 + 1.2), 3800],
					[end + 0.01, 1400]
				]);
				const chords: [number, number, string[]][] = [
					[0, T1, ['D3', 'A3', 'E4', 'F#4']],
					[T1, T2, ['B2', 'F#3', 'D4', 'E4']],
					[T2, end, ['G2', 'D3', 'B3', 'A4']]
				];
				chords.forEach(([a, z, notes], i) => {
					if (a >= end) return;
					pad(h, w + Math.max(0, a - 0.1), notes, Math.max(0.3, Math.min(z, end) - a - 0.5), {
						attack: i ? 0.7 : 1.2,
						release: 1.4,
						level: 0.9,
						seed: 120 + i
					});
				});
				sub(h, w + 0.2, 'D2', Math.max(0.5, T1 - 1), { gain: 0.35, attack: 1.2, release: 1 });
			},
			D
		);
		const motif = (at: number, notes: string[], pans: number[], seed: number) =>
			notes.forEach((n, i) =>
				tl.sound(at + i * (BEAT / 2), (w, b) =>
					glass(house(sound, b), w, n, {
						decay: i === notes.length - 1 ? 2.2 : 1.6,
						gain: 0.85,
						pan: pans[i],
						verb: 0.65,
						seed: seed + i
					})
				)
			);
		motif(0.4, ['A4', 'D5', 'E5'], [-0.3, 0, 0.3], 600);
		tl.sound(0.4, (w, b) =>
			piano(house(sound, b), w, 'D2', { vel: 0.4, dur: 2.4, verb: 0.6, gain: 0.6, seed: 610 })
		);
		if (tomorrow) {
			const crossing = (f: number) =>
				passes(MAP.x + f * MAP.width, MAP.x - 40, 1930, T1 + 0.1, 1.5, 'move');
			['B4', 'E5', 'F#5'].forEach((n, i) =>
				tl.sound(crossing(0.2 + 0.3 * i), (w, b) =>
					glass(house(sound, b), w, n, {
						decay: i === 2 ? 2.2 : 1.6,
						gain: 0.8,
						pan: -0.5 + 0.5 * i,
						verb: 0.65,
						seed: 620 + i
					})
				)
			);
			tl.sound(T1, (w, b) =>
				air(house(sound, b), w, 1.8, {
					peakAt: 0.8,
					band: [500, 2600],
					pans: [-0.7, 0.7],
					gain: 0.55,
					seed: 630
				})
			);
			tl.sound(T1, (w, b) =>
				piano(house(sound, b), w, 'B1', { vel: 0.35, dur: 2.2, verb: 0.6, gain: 0.55, seed: 631 })
			);
		}
		if (table) {
			tl.sound(T2, (w, b) =>
				air(house(sound, b), w, 1.7, {
					peakAt: 0.75,
					band: [400, 2200],
					pans: [-0.8, 0.8],
					gain: 0.6,
					seed: 640
				})
			);
			motif(T2 + 0.4, ['A4', 'D5', 'A5'], [-0.25, 0.05, 0.3], 650);
			tl.sound(T2 + 0.4, (w, b) => {
				const h = house(sound, b);
				piano(h, w, 'G1', { vel: 0.35, dur: 2.2, verb: 0.6, gain: 0.55, seed: 660 });
				shimmer(h, w + 0.8, ['D6', 'F#6', 'A6'], 0.8, {
					gain: 0.5,
					attack: 0.6,
					release: 1.2,
					seed: 661
				});
			});
		}
		return tl;
	}
};
