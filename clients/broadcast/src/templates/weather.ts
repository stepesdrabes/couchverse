// The weather (placeholder until the bulletin design lands): Czechia's outline with every city's
// temperature now, the first city's next days, ČHMÚ's warnings and the credits. Everything
// settles in the first seconds and then holds still, so most frames render as one capture.

import { css, div, span, svg } from '../core/dom';
import { fitText } from '../core/fit';
import { czechiaMap } from '../core/geo';
import type { BulletinTime, WeatherSpec, WeatherWarning } from '../spec';
import { bulletinBackdrop, bulletinHead, bulletinOut, bulletinSound, credit } from './bulletin';
import { timeline, type Template } from './shared';

/** The words for a WMO code (Open-Meteo's table), by day or by night. */
export function condition(code: number, day = true): string {
	if (code === 0) return day ? 'Sunny' : 'Clear';
	if (code === 1) return 'Mainly clear';
	if (code === 2) return 'Partly cloudy';
	if (code === 3) return 'Overcast';
	if (code === 45 || code === 48) return 'Fog';
	if (code >= 51 && code <= 55) return 'Drizzle';
	if (code === 56 || code === 57) return 'Freezing drizzle';
	if (code === 61) return 'Light rain';
	if (code === 63) return 'Rain';
	if (code === 65) return 'Heavy rain';
	if (code === 66 || code === 67) return 'Freezing rain';
	if (code >= 71 && code <= 77) return code === 75 ? 'Heavy snow' : 'Snow';
	if (code >= 80 && code <= 82) return code === 82 ? 'Heavy showers' : 'Showers';
	if (code === 85 || code === 86) return 'Snow showers';
	if (code >= 95) return code === 95 ? 'Thunderstorms' : 'Thunderstorms with hail';
	return 'Unsettled';
}

/** `14°`, with a true minus sign below zero. */
export const degrees = (t: number) => `${t < 0 ? '−' : ''}${Math.abs(t)}°`;

const when = (t: BulletinTime) => (t.day ? `${t.day} ${t.time}` : t.time);

/** `Now until Wednesday 02:00`, `From 16:00 until further notice`. */
export function warningTime(w: WeatherWarning): string {
	const until = w.until ? `until ${when(w.until)}` : 'until further notice';
	return w.active ? `Now ${until}` : `From ${when(w.from)} ${until}`;
}

// the side column holds the days and this many warnings under them
const WARNINGS_SHOWN = 2;

export const weather: Template<WeatherSpec> = {
	version: 1,
	build({ root, sound }, spec) {
		const D = spec.duration ?? 15;
		const tl = timeline(root, D);
		root.classList.add('bulletin', 'weather');
		bulletinBackdrop(tl, root);
		bulletinHead(tl, root, 'Weather', 'Czechia', spec.asOf);

		// the map: the outline and every city at its place with its temperature now
		const map = czechiaMap({ x: 128, y: 300, width: 900, height: 520 });
		const art = svg(
			'svg',
			{ class: 'wx-map', viewBox: '0 0 1920 1080', width: 1920, height: 1080 },
			root
		);
		svg('path', { d: map.path, class: 'wx-border' }, art);
		const pins = spec.cities.map((c) => {
			const [x, y] = map.point(c.lat, c.lon);
			const pin = css(div('wx-pin', root), { left: `${x.toFixed(1)}px`, top: `${y.toFixed(1)}px` });
			// the easternmost cities are labelled to the west, clear of the side column
			if (x > map.box.x + 0.88 * map.box.width) pin.classList.add('west');
			div('wx-dot', pin);
			const label = div('wx-label', pin);
			span('wx-temp tnum', label, degrees(c.now.temperature));
			span('wx-city', label, c.name);
			return pin;
		});
		tl.add(art, [{ opacity: 0 }, { opacity: 1 }], 0.3, 0.9, 'sine');
		pins.forEach((p, i) =>
			tl.add(
				p,
				[
					{ opacity: 0, transform: 'scale(0.6)' },
					{ opacity: 1, transform: 'scale(1)' }
				],
				0.7 + i * 0.05,
				0.5,
				'pop'
			)
		);

		// the first city's days
		const first = spec.cities[0];
		const side = div('wx-side', root);
		const days = div('wx-days', side);
		fitText(div('eyebrow wx-days-city', days, first.name), { width: 700, min: 18 });
		const rows = first.days.map((d) => {
			const row = div('wx-day', days);
			div('wx-day-label', row, d.label);
			div('wx-day-text', row, condition(d.code));
			div('wx-day-rain tnum', row, d.precipitation != null ? `${d.precipitation}%` : '');
			div('wx-day-temps tnum', row, `${degrees(d.min)} / ${degrees(d.max)}`);
			return row;
		});
		rows.forEach((r, i) =>
			tl.add(
				r,
				[
					{ opacity: 0, transform: 'translateX(40px)' },
					{ opacity: 1, transform: 'translateX(0)' }
				],
				0.9 + i * 0.1,
				0.7,
				'enter'
			)
		);

		// the warnings, when ČHMÚ could be read
		const credits = [spec.credits.forecast];
		if (spec.warnings) {
			credits.push(spec.credits.warnings);
			const box = div('wx-warnings', side);
			div(
				'eyebrow wx-warnings-head',
				box,
				spec.warnings.length ? 'Weather warnings' : 'No weather warnings'
			);
			for (const w of spec.warnings.slice(0, WARNINGS_SHOWN)) {
				const item = div(`wx-warning ${w.level}`, box);
				const what = div('wx-warning-event', item, w.event);
				fitText(what, { width: 640, min: 22 });
				div('wx-warning-time', item, warningTime(w));
				const where = div('wx-warning-regions', item, w.regions.map((r) => r.name).join(', '));
				fitText(where, { width: 640, min: 18, lines: 2, leading: 1.2 });
			}
			const more = spec.warnings.length - WARNINGS_SHOWN;
			if (more > 0)
				div(
					'wx-warning-more',
					box,
					more === 1 ? 'and 1 more warning' : `and ${more} more warnings`
				);
			tl.add(
				box,
				[
					{ opacity: 0, transform: 'translateY(20px)' },
					{ opacity: 1, transform: 'translateY(0)' }
				],
				1.5,
				0.7,
				'enter'
			);
		}
		credits.push(spec.credits.borders);
		credit(tl, root, credits.join('  ·  '), 1.2);
		bulletinOut(tl, root);
		bulletinSound(tl, sound, [0.7, 0.9, 1.5]);
		return tl;
	}
};
