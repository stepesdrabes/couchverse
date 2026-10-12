// The EPG grid's geometry: one column per date, 24 hours from the hour the broadcast
// day starts, with programs and breaks placed by their minutes into the column.
import type { CouchTvAiring, CouchTvSlot } from '$lib/generated/api';
import { addDays, clockMinutes, dateIn, wallTime, zonedTime } from './time';

const MINUTE = 60_000;
export const DAY_MINUTES = 24 * 60;
/** The flow grid programs start on (couchtv.flowStep). */
export const FLOW_STEP_MS = 5 * MINUTE;

/** A date's column: [start, end) in ms. */
export interface DayColumn {
	date: string;
	start: number;
	end: number;
}

/** A row of the schedule placed in a column, in minutes from its top. */
export interface Placed {
	airing: CouchTvAiring;
	top: number;
	height: number;
	/** it began in the column before, or runs on into the next */
	cutStart: boolean;
	cutEnd: boolean;
}

const dayNames = ['sun', 'mon', 'tue', 'wed', 'thu', 'fri', 'sat'];

/**
 * Minutes after midnight where the columns start: the earliest time any weekday's broadcast
 * day opens (its first slot's start), 06:00 when the template has none.
 */
export function axisStart(slots: CouchTvSlot[]): number {
	let earliest = Infinity;
	for (const day of dayNames) {
		const first = slots.find((s) => s.days.includes(day as CouchTvSlot['days'][number]));
		const at = first?.start ? clockMinutes(first.start) : NaN;
		if (!Number.isNaN(at)) earliest = Math.min(earliest, at);
	}
	return Number.isFinite(earliest) ? earliest : 6 * 60;
}

/** Columns for days dates from first, each 24 hours of wall time from the axis start. */
export function columns(first: string, days: number, axis: number, timeZone: string): DayColumn[] {
	return Array.from({ length: days }, (_, i) => {
		const date = addDays(first, i);
		return {
			date,
			start: zonedTime(date, axis, timeZone),
			end: zonedTime(addDays(date, 1), axis, timeZone)
		};
	});
}

/** The airings of a kind that show in a column, placed. */
export function place(
	column: DayColumn,
	airings: CouchTvAiring[],
	kind: CouchTvAiring['kind']
): Placed[] {
	const out: Placed[] = [];
	for (const a of airings) {
		if (a.kind !== kind) continue;
		const s = Date.parse(a.startsAt);
		const e = Date.parse(a.endsAt);
		if (e <= column.start || s >= column.end) continue;
		const from = Math.max(s, column.start);
		const to = Math.min(e, column.end);
		out.push({
			airing: a,
			top: (from - column.start) / MINUTE,
			height: (to - from) / MINUTE,
			cutStart: s < column.start,
			cutEnd: e > column.end
		});
	}
	return out;
}

/** Where now is in a column, in minutes from its top; null when it is not in it. */
export function nowLine(column: DayColumn, now: number): number | null {
	return now >= column.start && now < column.end ? (now - column.start) / MINUTE : null;
}

/** When a program inserted after another would start: the flow's next mark after its end and the shortest break. */
export function afterTime(airing: CouchTvAiring, breakMinSeconds: number): number {
	const t = Date.parse(airing.endsAt) + breakMinSeconds * 1000;
	return Math.ceil(t / FLOW_STEP_MS) * FLOW_STEP_MS;
}

/** The date of the column an instant falls in: before the axis start it is the night of the day before. */
export function columnDate(ms: number, axis: number, timeZone: string): string {
	const w = wallTime(ms, timeZone);
	const date = dateIn(ms, timeZone);
	return w.hour * 60 + w.minute < axis ? addDays(date, -1) : date;
}

/** The instant a column's date shows a clock time, a time before the axis start being the night after. */
export function atClock(date: string, hhmm: string, axis: number, timeZone: string): number {
	const minutes = clockMinutes(hhmm);
	if (Number.isNaN(minutes)) return NaN;
	return zonedTime(minutes < axis ? addDays(date, 1) : date, minutes, timeZone);
}

export interface Readiness {
	programs: number;
	prepared: number;
	breaks: number;
	rendered: number;
	failed: number;
}

/** How ready what airs from now to until is: prepared programs, rendered breaks. */
export function readiness(airings: CouchTvAiring[], now: number, until: number): Readiness {
	const r: Readiness = { programs: 0, prepared: 0, breaks: 0, rendered: 0, failed: 0 };
	for (const a of airings) {
		const s = Date.parse(a.startsAt);
		if (s < now || s >= until) continue;
		if (a.kind === 'program') {
			r.programs++;
			if (a.ready) r.prepared++;
		} else {
			r.breaks++;
			if (a.render === 'ready') r.rendered++;
			if (a.render === 'failed') r.failed++;
		}
	}
	return r;
}

/** The program before and after a program, and the breaks between them. */
export function neighbours(airings: CouchTvAiring[], id: string) {
	const i = airings.findIndex((a) => a.id === id);
	const before = airings.slice(0, Math.max(i, 0)).reverse();
	const after = i < 0 ? [] : airings.slice(i + 1);
	return {
		breakBefore: before[0]?.kind === 'break' ? before[0] : undefined,
		breakAfter: after[0]?.kind === 'break' ? after[0] : undefined,
		previous: before.find((a) => a.kind === 'program'),
		next: after.find((a) => a.kind === 'program')
	};
}
