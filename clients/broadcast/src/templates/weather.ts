// The weather (DESIGN.md 2.10, sharp), half a minute long: Czechia as a deep red country of
// kraje on the night, every configured city's sky and temperature where the city is, ČHMÚ's
// warnings filling the kraje they cover. Scenes, each still once it has entered: now (the cities
// and the warmest and coldest of them), the warnings (the cities give way to the kraje, one
// warning at a time, or "No weather warnings"), today and tomorrow (the cities turn to the day's
// high and low, tomorrow's as a red front crosses the country west to east) and the next days
// (a red bar sweeps the map away into a table), then the channel's sign-off. English chrome; city
// names as configured. The music is the house pad stepping through one progression in D, glass
// at every change of scene, air under the sweeps and the sonic logo's own chord to close, all of
// it over before the clip ends.

import {
	air,
	glass,
	house,
	pad,
	padSweep,
	piano,
	shimmer,
	sub,
	type PadOptions
} from '../audio/voices';
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
	list,
	mark,
	SAFE_RIGHT,
	share,
	signOff
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

/** What the day's chances of precipitation come to: the highest, or that there is none. */
export function precipitation(chances: (number | undefined)[]) {
	const known = chances.filter((p): p is number => p != null);
	if (!known.length) return '';
	const most = Math.max(...known);
	return most === 0 ? 'No precipitation expected' : `Precipitation chance up to ${most}%`;
}

const when = (t: { day?: string; time: string }) => (t.day ? `${t.day} ${t.time}` : t.time);

/** `Now until Wednesday 02:00`, `From 16:00 until further notice`. */
export function warningTime(w: WeatherWarning): string {
	const until = w.until ? `until ${when(w.until)}` : 'until further notice';
	return w.active ? `Now ${until}` : `From ${when(w.from)} ${until}`;
}

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

// the warnings the scene has room for; the rest are counted
export const WARNINGS_SHOWN = 3;

/** The scenes' starts, seconds. */
export interface Schedule {
	/** The warnings scene (or the quiet one), where the bulletin has one. */
	warnings?: number;
	today: number;
	tomorrow?: number;
	days?: number;
	/** When each warning shown comes up within its scene. */
	each: number[];
	/** Where the sign-off's popcorn lands. */
	tag: number;
}

/**
 * When each scene starts, on the beat, filling the bulletin: now, the warnings (`shown` of them,
 * 0 for a quiet day, null when ČHMÚ could not be read: no scene), today, and where the forecast
 * has a second day tomorrow and the next days, which hold the sign-off in their last seconds.
 * Each scene wants the beats its content needs and gives or takes some of them when the
 * warnings or the days change what there is to read.
 */
export function schedule(D: number, shown: number | null, days: number): Schedule {
	const table = days >= 2;
	// the scenes in order, each with the beats it wants and may take
	const scenes: [string, { w: number; min: number; max: number }][] = [
		['now', { w: 9, min: 6, max: 12 }]
	];
	if (shown != null)
		scenes.push([
			'warnings',
			shown
				? { w: 3 + 4 * shown, min: 3 + 3 * shown, max: 3 + 6 * shown }
				: { w: 5, min: 4, max: 7 }
		]);
	scenes.push(['today', table ? { w: 8, min: 5, max: 11 } : { w: 12, min: 9, max: 16 }]);
	if (table)
		scenes.push(['tomorrow', { w: 8, min: 5, max: 11 }], ['days', { w: 14, min: 10, max: 18 }]);
	const beats = share(
		scenes.map(([, p]) => p),
		Math.round(D / BEAT)
	);
	const start = new Map<string, number>();
	let t = 0;
	scenes.forEach(([id], i) => {
		start.set(id, Math.round(t * BEAT * 1000) / 1000);
		t += beats[i];
	});
	const out: Schedule = {
		warnings: start.get('warnings'),
		today: start.get('today')!,
		tomorrow: start.get('tomorrow'),
		days: start.get('days'),
		each: [],
		tag: D - 5.2
	};
	if (out.warnings != null && shown) {
		const len = out.today - out.warnings;
		for (let k = 0; k < shown; k++) out.each.push(out.warnings + 0.3 + (k * (len - 0.3)) / shown);
	}
	return out;
}

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

/** A city's sky and temperature for one scene: the icon's wrapper and the temperature's clip. */
interface Face {
	icon: HTMLElement;
	temp: HTMLElement;
	clip: HTMLElement;
}

const FACES = ['now', 'today', 'tomorrow'] as const;

interface City {
	x: number;
	y: number;
	name: HTMLElement;
	now: Face;
	today: Face;
	tomorrow?: Face;
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
	for (const scene of FACES) {
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

/**
 * Where a kraj's name goes: the point nearest the kraj's middle with the whole name inside the
 * kraj (probed on the kraj's own path), or null where the kraj is too small to hold it.
 */
function labelSpot(
	path: SVGPathElement,
	k: KrajShape,
	label: HTMLElement
): [number, number] | null {
	const hw = label.offsetWidth / 2 + 10;
	const hh = label.offsetHeight / 2 + 6;
	const probes = [
		[0, 0],
		[-hw, 0],
		[hw, 0],
		[-hw, -hh],
		[hw, -hh],
		[-hw, hh],
		[hw, hh]
	];
	const [cx, cy] = [k.box.x + k.box.width / 2, k.box.y + k.box.height / 2];
	let best: [number, number] | null = null;
	let near = Infinity;
	for (let y = k.box.y; y <= k.box.y + k.box.height; y += 14)
		for (let x = k.box.x; x <= k.box.x + k.box.width; x += 14) {
			const d = (x - cx) ** 2 + (y - cy) ** 2;
			if (
				d >= near ||
				!probes.every(([dx, dy]) => path.isPointInFill(new DOMPoint(x + dx, y + dy)))
			)
				continue;
			best = [x, y];
			near = d;
		}
	return best;
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
	version: 4,
	build({ root, sound }, spec) {
		const D = spec.duration ?? 38.4;
		const tl = timeline(root, D);
		root.classList.add('bulletin', 'weather');
		field(tl, root, { x: 1240, y: 1220, rx: 1500, ry: 560, a: 0.75 });

		// the scenes: where there are warnings they are the second, a quiet day has a short one
		const days = Math.min(...spec.cities.map((c) => c.days.length));
		const table = days >= 2;
		const warnings = spec.warnings ?? [];
		const shown = warnings.slice(0, WARNINGS_SHOWN);
		const sched = schedule(D, spec.warnings ? shown.length : null, days);
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

		// each warning's kraje in its level's colour, hatched where only part of one is warned,
		// each named where the kraj has room for its name
		const tints = shown.map((w, wi) => {
			const out: { holder: HTMLElement; k: KrajShape }[] = [];
			const colour = LEVEL_COLOUR[w.level];
			for (const r of w.regions) {
				const k = map.kraje.find((x) => x.code === r.code);
				if (!k) continue;
				const { holder, s } = layer('wx-tint', k.box);
				let fill = colour;
				if (r.partial) {
					const id = `wx-hatch-${gradients}-${wi}-${k.code}`;
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
				const path = svg(
					'path',
					{
						d: k.path,
						fill,
						'fill-opacity': r.partial ? 0.5 : 0.34,
						...(r.partial ? {} : { stroke: colour })
					},
					s
				);
				const name = div('wx-kname', holder, r.name.replace(/ Region$/, ''));
				const spot = labelSpot(path, k, name);
				if (spot)
					css(name, {
						left: `${spot[0] - holder.offsetLeft - name.offsetWidth / 2}px`,
						top: `${spot[1] - holder.offsetTop - name.offsetHeight / 2}px`
					});
				else name.remove();
				out.push({ holder, k });
			}
			return out;
		});

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
			const face = (code: number, night: boolean, hi: number, lo?: number): Face => ({
				icon: icon(code, night),
				...temp(hi, lo)
			});
			const [d0, d1] = c.days;
			return {
				x,
				y,
				name,
				now: face(c.now.code, !c.now.day, c.now.temperature),
				today: face(d0.code, false, d0.max, d0.min),
				tomorrow: table && d1 ? face(d1.code, false, d1.max, d1.min) : undefined
			};
		});
		placeLabels(cityEls);
		const west = [...cityEls].sort((a, b) => a.x - b.x);

		const faceIn = (f: Face, at: number) => {
			tl.add(
				f.icon,
				[
					{ opacity: 0, transform: 'scale(0.82)' },
					{ opacity: 1, transform: 'scale(1)' }
				],
				at,
				0.55,
				'glide'
			);
			tl.add(
				f.temp,
				[{ transform: 'translateY(150%)' }, { transform: 'translateY(0)' }],
				at + 0.06,
				0.6,
				'glide'
			);
		};
		const faceOut = (f: Face, at: number) => {
			tl.add(f.icon, [{ opacity: 1 }, { opacity: 0 }], at, 0.2, 'sine');
			tl.add(
				f.temp,
				[{ transform: 'translateY(0)' }, { transform: 'translateY(-150%)' }],
				at,
				0.28,
				'leave'
			);
		};
		const nameIn = (c: City, at: number) =>
			tl.add(
				c.name,
				[
					{ opacity: 0, transform: 'translateY(8px)' },
					{ opacity: 1, transform: 'translateY(0px)' }
				],
				at,
				0.5,
				'glide'
			);
		west.forEach((c, i) => {
			const at = 0.75 + i * 0.045;
			faceIn(c.now, at);
			nameIn(c, at + 0.1);
		});

		// --- the left column: the headers and the cards ----------------------------------------
		const hNow = head(root, 'Weather', ['Now', spec.asOf]);
		const hWarn = spec.warnings
			? head(root, 'Warnings', [
					'Czechia',
					...(warnings.length
						? [`${warnings.length} warning${warnings.length > 1 ? 's' : ''}`]
						: [])
				])
			: null;
		const hToday = head(root, first.days[0].label, [first.days[0].weekday]);
		const hTom = table ? head(root, first.days[1].label, [first.days[1].weekday]) : null;
		const hDays = table ? head(root, 'Next days', ['Czechia']) : null;

		const cardClip = (top = 470) =>
			css(div('wx-win', root), { left: '96px', top: `${top}px`, width: '480px', height: '520px' });
		const card = (parent: HTMLElement, colour: string) => {
			const c = css(div('wx-card', parent), { left: '0px', top: '0px' });
			const bar = css(div('wx-card-bar', c), { background: colour });
			return { c, bar, body: div('wx-card-body', c) };
		};
		type Card = { c: HTMLElement; bar: HTMLElement };
		const cardIn = (c: Card, at: number) => {
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
		const cardOut = (c: Card, at: number) =>
			tl.add(
				c.c,
				[{ transform: 'translateX(0)' }, { transform: 'translateX(-105%)' }],
				at,
				0.4,
				'leave'
			);

		// now: the warmest and the coldest of the cities
		const extremes = () => {
			const by = spec.cities.map((c) => ({ name: c.name, t: c.now.temperature }));
			const hot = by.reduce((a, b) => (b.t > a.t ? b : a));
			const cold = by.reduce((a, b) => (b.t < a.t ? b : a));
			const clip = cardClip();
			const k = card(clip, BRAND.red);
			const rows =
				hot.t === cold.t
					? [['Everywhere', hot, false] as const]
					: [['Warmest', hot, true] as const, ['Coldest', cold, true] as const];
			for (const [label, who, named] of rows) {
				const r = div('wx-extreme', k.body);
				div('k', r, label);
				const v = div('v tnum', r);
				span('t', v, degrees(who.t));
				if (named) fitText(span('n', v, who.name), { width: 320, min: 18 });
			}
			return k;
		};
		const nowCard = spec.cities.length > 1 ? extremes() : null;

		// the warnings, one card each; a quiet day's card says so
		const warningCards = shown.map((w, k) => {
			const clip = cardClip();
			const c = css(div('wx-card', clip), { left: '0px', top: '0px' });
			const item = div(`wx-warning lv-${w.level}`, c);
			const bar = css(div('wx-card-bar', item), { background: LEVEL_COLOUR[w.level] });
			const body = div('wx-card-body', item);
			const level = div('wx-level', body);
			span(null, level, LEVEL_NAME[w.level]);
			if (warnings.length > 1) span('of', level, `${k + 1} of ${warnings.length}`);
			fitText(div('wx-event', body, w.event), { width: 442, min: 30, lines: 2, leading: 1.08 });
			fitText(div('wx-when tnum', body, warningTime(w)), { width: 442, min: 20 });
			fitText(div('wx-regions', body, regionsPhrase(w)), {
				width: 442,
				min: 18,
				lines: 5,
				leading: 1.36
			});
			const more = warnings.length - shown.length;
			if (more > 0 && k === shown.length - 1) div('wx-more', c, `+ ${more} more`);
			css(clip, {
				top: `${Math.min(470, 944 - c.offsetHeight)}px`,
				height: `${c.offsetHeight + 24}px`
			});
			return { c, bar };
		});
		const quietCard = (() => {
			if (!spec.warnings || warnings.length) return null;
			const k = card(cardClip(), 'var(--paper-40)');
			div('wx-quiet', k.body, 'No weather warnings');
			div('wx-calm', k.body, 'None in force now or later today');
			return k;
		})();

		// today and tomorrow: the day's highs and lows over the cities, and its chance of rain
		const rangeCard = (day: number) => {
			const k = card(cardClip(), BRAND.red);
			for (const [label, pick] of [
				['Highs', (d: { max: number }) => d.max],
				['Lows', (d: { min: number }) => d.min]
			] as const) {
				const r = div('wx-range tnum', k.body);
				span('k', r, label);
				span('v', r, range(spec.cities.map((ct) => pick(ct.days[day]))));
			}
			const rain = precipitation(spec.cities.map((ct) => ct.days[day].precipitation));
			if (rain) fitText(div('wx-calm', k.body, rain), { width: 442, min: 18 });
			return k;
		};
		const todayCard = rangeCard(0);
		const tomCard = table ? rangeCard(1) : null;

		// --- the credits and the corner ---------------------------------------------------------
		const mapCredits = [spec.credits.forecast, spec.credits.borders];
		const credMap = credits(root, mapCredits);
		tl.add(credMap, [{ opacity: 0 }, { opacity: 1 }], 1.2, 0.6, 'sine');
		const corner = mark(tl, root, 0.3);
		headIn(tl, hNow, 0.15);
		if (nowCard) cardIn(nowCard, 1.35);
		tl.cue('now', 0);

		// every scene change swaps the header, the card and the credit; `swap` does the first two
		let onHead = hNow;
		let onCard: Card | null = nowCard;
		const swap = (to: typeof hNow, next: Card | null, at: number) => {
			headOut(tl, onHead, at);
			headIn(tl, to, at + 0.3);
			if (onCard) cardOut(onCard, at);
			if (next) cardIn(next, at + 0.4);
			onHead = to;
			onCard = next;
		};
		const fade = (el: Element, from: number, to: number, at: number, dur = 0.35) =>
			tl.add(el, [{ opacity: from }, { opacity: to }], at, dur, 'sine');

		// --- the warnings: the cities give way to the kraje, one warning at a time --------------
		const T = sched;
		let credWarn: HTMLElement | null = null;
		if (hWarn && T.warnings != null) {
			const tw = T.warnings;
			tl.cue(shown.length ? 'warnings' : 'no warnings', tw);
			swap(hWarn, quietCard, tw);
			credWarn = credits(root, [
				spec.credits.forecast,
				spec.credits.warnings,
				spec.credits.borders
			]);
			fade(credMap, 1, 0, tw + 0.1);
			fade(credWarn, 0, 1, tw + 0.6, 0.5);
			shown.forEach((_, k) => {
				const at = T.each[k];
				if (k) {
					cardOut(warningCards[k - 1], at);
					for (const { holder } of tints[k - 1]) fade(holder, 1, 0, at, 0.4);
				}
				cardIn(warningCards[k], at + (k ? 0.35 : 0.5));
				tints[k].forEach(({ holder }, j) => fade(holder, 0, 1, at + 0.3 + j * 0.06, 0.5));
				onCard = warningCards[k];
			});
			// where there are warnings the cities leave the map to the kraje
			if (shown.length)
				west.forEach((c, i) => {
					const at = tw + 0.05 + i * 0.04;
					faceOut(c.now, at);
					fade(c.name, 1, 0, at + 0.05, 0.3);
				});
		}

		// --- today: the cities turn to the day's high and low -----------------------------------
		const hidden = shown.length > 0;
		tl.cue('today', T.today);
		swap(hToday, todayCard, T.today);
		if (credWarn) {
			fade(credWarn, 1, 0, T.today);
			fade(credMap, 0, 1, T.today + 0.5, 0.5);
		}
		if (hidden) {
			for (const { holder } of tints[shown.length - 1]) fade(holder, 1, 0, T.today, 0.4);
			west.forEach((c, i) => {
				const at = T.today + 0.45 + i * 0.045;
				faceIn(c.today, at);
				nameIn(c, at + 0.1);
			});
		} else
			west.forEach((c, i) => {
				const at = T.today + 0.1 + i * 0.045;
				faceOut(c.now, at);
				faceIn(c.today, at + 0.12);
			});

		// --- tomorrow: a front crosses the country, every city turning as it passes -------------
		const last: Element[] = [];
		if (hTom && tomCard && T.tomorrow != null) {
			const tm = T.tomorrow;
			tl.cue('tomorrow', tm);
			swap(hTom, tomCard, tm);
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
				tm + 0.1,
				sweep,
				'move'
			);
			tl.add(front, [{ opacity: 0 }, { opacity: 1 }], tm + 0.1, 0.12, 'linear');
			tl.add(front, [{ opacity: 1 }, { opacity: 0 }], tm + 0.1 + sweep - 0.12, 0.12, 'linear');
			for (const c of cityEls) {
				if (!c.tomorrow) continue;
				const at = passes(c.x, x0, x1, tm + 0.1, sweep, 'move');
				faceOut(c.today, at - 0.05);
				faceIn(c.tomorrow, at + 0.05);
			}
		}

		// --- the next days: a red bar sweeps the map away and leaves the table ------------------
		if (hDays && T.days != null) {
			const td = T.days;
			tl.cue('next days', td);
			swap(hDays, null, td);
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
			// the rows end above the sign-off in the foot's corner
			const rowH = Math.min(58, Math.floor(660 / rows));
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
			fade(credMap, 1, 0, td + 0.2);
			fade(credDays, 0, 1, td + 0.75, 0.5);

			// the bar's right edge runs from the region's left edge until the bar has left it:
			// the map shows right of the bar, the table left of it
			const at = td + 0.15;
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
			last.push(tWin, credDays);
		} else last.push(mapWin, credMap, ...(onCard ? [onCard.c] : []));

		// the sign-off: the tag lands in the foot's corner as the last scene holds
		const tag = signOff(tl, corner, T.tag, sound);
		exit(tl, [...last, onHead.root, tag]);

		// --- the sound: the pad steps through D, B minor, G, A and E minor and home -------------
		const end = D - 1.2;
		const quiet = shown.length === 0;
		const starts: { at: number; chord: string[]; root: string }[] = [
			{ at: 0, chord: ['D3', 'A3', 'E4', 'F#4'], root: 'D2' }
		];
		if (T.warnings != null)
			starts.push({ at: T.warnings, chord: ['B2', 'F#3', 'D4', 'E4'], root: 'B1' });
		starts.push({ at: T.today, chord: ['G2', 'D3', 'B3', 'A4'], root: 'G1' });
		if (T.tomorrow != null && T.days != null)
			starts.push(
				{ at: T.tomorrow, chord: ['A2', 'E3', 'B3', 'C#4'], root: 'A1' },
				{ at: T.days, chord: ['E3', 'B3', 'D4', 'F#4'], root: 'E2' }
			);
		// the mix and its fade, and the pads' filter, dipping at every change of scene and
		// opening again over the next
		tl.sound(
			0,
			(when, b, skip) => {
				const w = when - skip;
				bulletinHouse(sound, b, w, D);
				const sweep: [number, number][] = [
					[0, 700],
					[1.6, 2600]
				];
				for (const { at } of starts.slice(1)) sweep.push([at - 0.4, 2200], [at + 1.2, 3400]);
				sweep.push([T.tag - 0.4, 2300], [T.tag + 1.2, 3400], [end + 0.01, 1400]);
				padSweep(house(sound, b), w, sweep);
			},
			D
		);
		// a chord is built when its turn comes, not up front: every voice built costs the render
		const chord = (from: number, notes: string[], dur: number, o: PadOptions) =>
			tl.sound(
				from,
				(when, b, skip) => pad(house(sound, b), when - skip, notes, dur, o),
				o.attack! + dur + o.release! * 2.4
			);
		starts.forEach(({ at, chord: notes }, i) => {
			const to = starts[i + 1]?.at ?? T.tag;
			const from = i ? at - 0.4 : 0.1;
			chord(from, notes, Math.max(0.3, to - from - 0.8), {
				attack: i ? 0.8 : 1.2,
				release: 1.4,
				level: 0.9,
				seed: 120 + i
			});
		});
		// home: the chord the sonic logo ends on, held into the fade
		chord(T.tag - 0.5, ['D3', 'A3', 'E4', 'F#4'], end - T.tag - 0.3, {
			attack: 0.9,
			release: 1.4,
			level: 0.9,
			seed: 130
		});
		tl.sound(0.2, (w, b) =>
			sub(house(sound, b), w, 'D2', Math.max(0.5, starts[1].at - 1), {
				gain: 0.35,
				attack: 1.2,
				release: 1
			})
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
		// a few felt notes rising through a scene's hold, never twice alike
		const rise = (at: number, notes: string[], seed: number) =>
			notes.forEach((n, i) =>
				tl.sound(at + i * BEAT, (w, b) =>
					piano(house(sound, b), w, n, {
						vel: 0.3,
						dur: 1.1,
						pan: -0.2 + 0.2 * i,
						verb: 0.55,
						gain: 0.55,
						seed: seed + i
					})
				)
			);
		const low = (at: number, note: string, seed: number) =>
			tl.sound(at, (w, b) =>
				piano(house(sound, b), w, note, { vel: 0.4, dur: 2.4, verb: 0.6, gain: 0.6, seed })
			);
		const swell = (at: number, dur: number, band: [number, number], seed: number, gain = 0.55) =>
			tl.sound(at, (w, b) =>
				air(house(sound, b), w, dur, {
					peakAt: dur * 0.45,
					band,
					pans: [-0.75, 0.75],
					gain,
					seed
				})
			);
		motif(0.4, ['A4', 'D5', 'E5'], [-0.3, 0, 0.3], 600);
		low(0.4, 'D2', 610);
		rise(T.warnings != null ? T.warnings - 3 : T.today - 3, ['A3', 'D4', 'F#4'], 615);
		if (T.warnings != null) {
			const tw = T.warnings;
			swell(tw, 1.8, [500, 2200], 625, quiet ? 0.4 : 0.55);
			motif(tw + 0.5, quiet ? ['A4', 'D5', 'F#5'] : ['F#4', 'B4', 'D5'], [-0.2, 0.1, 0.3], 626);
			low(tw + 0.5, 'B1', 630);
			// each warning past the first is told by a lone note of glass
			T.each.slice(1).forEach((at, k) =>
				tl.sound(at + 0.3, (w, b) =>
					glass(house(sound, b), w, ['D5', 'B4'][k % 2], {
						decay: 2,
						gain: 0.7,
						pan: 0.1 - 0.2 * k,
						verb: 0.65,
						seed: 635 + k
					})
				)
			);
		}
		swell(T.today, 1.8, [450, 2400], 640);
		motif(T.today + 0.5, ['B4', 'D5', 'A5'], [-0.25, 0.05, 0.3], 641);
		low(T.today + 0.5, 'G1', 645);
		rise(T.today + 2.6, ['B3', 'D4', 'A4'], 646);
		if (T.tomorrow != null) {
			const tm = T.tomorrow;
			const crossing = (f: number) =>
				passes(MAP.x + f * MAP.width, MAP.x - 40, 1930, tm + 0.1, 1.5, 'move');
			['B4', 'E5', 'F#5'].forEach((n, i) =>
				tl.sound(crossing(0.2 + 0.3 * i), (w, b) =>
					glass(house(sound, b), w, n, {
						decay: i === 2 ? 2.2 : 1.6,
						gain: 0.8,
						pan: -0.5 + 0.5 * i,
						verb: 0.65,
						seed: 650 + i
					})
				)
			);
			swell(tm, 1.8, [500, 2600], 655);
			low(tm, 'B1', 656);
			rise(tm + 2.8, ['E4', 'B4', 'C#5'], 657);
		}
		if (T.days != null) {
			const td = T.days;
			swell(td, 1.7, [400, 2200], 660, 0.6);
			motif(td + 0.4, ['A4', 'D5', 'A5'], [-0.25, 0.05, 0.3], 661);
			tl.sound(td + 0.4, (w, b) => {
				const h = house(sound, b);
				piano(h, w, 'E2', { vel: 0.35, dur: 2.2, verb: 0.6, gain: 0.55, seed: 665 });
				shimmer(h, w + 0.8, ['D6', 'F#6', 'A6'], 0.8, {
					gain: 0.5,
					attack: 0.6,
					release: 1.2,
					seed: 666
				});
			});
			rise(td + 3, ['D4', 'F#4', 'A4'], 667);
		}
		return tl;
	}
};
