// Wall-clock maths in the channel's time zone (the schedule is stored in UTC and the
// template's times are local to the channel, whatever zone the admin's browser is in).
// Dates are civil dates as `YYYY-MM-DD`.

export interface WallTime {
	year: number;
	month: number;
	day: number;
	hour: number;
	minute: number;
	second: number;
}

const formatters = new Map<string, Intl.DateTimeFormat>();

function parts(timeZone: string) {
	let f = formatters.get(timeZone);
	if (!f) {
		f = new Intl.DateTimeFormat('en-US', {
			timeZone,
			hourCycle: 'h23',
			year: 'numeric',
			month: 'numeric',
			day: 'numeric',
			hour: 'numeric',
			minute: 'numeric',
			second: 'numeric'
		});
		formatters.set(timeZone, f);
	}
	return f;
}

/** The wall-clock time an instant (ms) shows in a time zone. */
export function wallTime(ms: number, timeZone: string): WallTime {
	const out: Record<string, number> = {};
	for (const p of parts(timeZone).formatToParts(new Date(ms))) {
		if (p.type !== 'literal') out[p.type] = Number(p.value);
	}
	return {
		year: out.year,
		month: out.month,
		day: out.day,
		hour: out.hour,
		minute: out.minute,
		second: out.second
	};
}

// how far the zone's clock is ahead of UTC at an instant
function offset(ms: number, timeZone: string): number {
	const w = wallTime(ms, timeZone);
	const asUtc = Date.UTC(w.year, w.month - 1, w.day, w.hour, w.minute, w.second);
	return asUtc - Math.floor(ms / 1000) * 1000;
}

/** The instant (ms) a date shows a clock time in a time zone; a time the clock skips lands past the gap. */
export function zonedTime(date: string, minutes: number, timeZone: string): number {
	const [y, m, d] = date.split('-').map(Number);
	const guess = Date.UTC(y, m - 1, d, 0, minutes);
	const first = guess - offset(guess, timeZone);
	// the offset at the answer can differ from the one at the guess across a clock change
	return guess - offset(first, timeZone);
}

const pad = (n: number) => String(n).padStart(2, '0');

/** The civil date an instant falls on in a time zone. */
export function dateIn(ms: number, timeZone: string): string {
	const w = wallTime(ms, timeZone);
	return `${w.year}-${pad(w.month)}-${pad(w.day)}`;
}

/** A date some days from another. */
export function addDays(date: string, days: number): string {
	const [y, m, d] = date.split('-').map(Number);
	const t = new Date(Date.UTC(y, m - 1, d + days));
	return `${t.getUTCFullYear()}-${pad(t.getUTCMonth() + 1)}-${pad(t.getUTCDate())}`;
}

/** The weekday of a date, 0 for Sunday. */
export function weekday(date: string): number {
	const [y, m, d] = date.split('-').map(Number);
	return new Date(Date.UTC(y, m - 1, d)).getUTCDay();
}

/** Minutes after midnight of an `HH:MM` time; NaN when it is not one. */
export function clockMinutes(hhmm: string): number {
	const m = /^([01]\d|2[0-3]):([0-5]\d)$/.exec(hhmm);
	return m ? Number(m[1]) * 60 + Number(m[2]) : NaN;
}

/** `HH:MM` of an instant in a time zone, for time inputs. */
export function clockOf(ms: number, timeZone: string): string {
	const w = wallTime(ms, timeZone);
	return `${pad(w.hour)}:${pad(w.minute)}`;
}

/** An instant's time of day in a time zone, on the 24-hour clock the template uses. */
export function formatClock(ms: number, timeZone: string, lang: string): string {
	return new Intl.DateTimeFormat(lang, {
		timeZone,
		hour: '2-digit',
		minute: '2-digit',
		hourCycle: 'h23'
	}).format(ms);
}

/** A date as a short weekday and day, in the display language. */
export function formatDay(
	date: string,
	lang: string,
	weekdayStyle: 'short' | 'long' = 'short'
): string {
	const [y, m, d] = date.split('-').map(Number);
	return new Intl.DateTimeFormat(lang, {
		timeZone: 'UTC',
		weekday: weekdayStyle,
		day: 'numeric',
		month: 'numeric'
	}).format(Date.UTC(y, m - 1, d));
}

/** A length of time as h:mm:ss or m:ss. */
export function formatLength(ms: number): string {
	const total = Math.max(0, Math.round(ms / 1000));
	const h = Math.floor(total / 3600);
	const m = Math.floor((total % 3600) / 60);
	const s = total % 60;
	return h > 0 ? `${h}:${pad(m)}:${pad(s)}` : `${m}:${pad(s)}`;
}

/** Parses m:ss, h:mm:ss or plain seconds (with an optional fraction) into ms; NaN otherwise. */
export function parseLength(text: string): number {
	const t = text.trim();
	if (!/^\d+(:\d{1,2}){0,2}(\.\d{1,3})?$/.test(t)) return NaN;
	const [whole, fraction = ''] = t.split('.');
	const units = whole.split(':').map(Number);
	if (units.slice(1).some((u) => u > 59)) return NaN;
	const seconds = units.reduce((acc, u) => acc * 60 + u, 0);
	return seconds * 1000 + Number(fraction.padEnd(3, '0'));
}
