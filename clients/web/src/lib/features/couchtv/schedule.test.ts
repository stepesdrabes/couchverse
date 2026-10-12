import { describe, expect, it } from 'vitest';
import type { CouchTvAiring } from '$lib/generated/api';
import {
	afterTime,
	atClock,
	axisStart,
	columnDate,
	columns,
	neighbours,
	nowLine,
	place,
	readiness
} from './schedule';

const PRAGUE = 'Europe/Prague';

function airing(
	id: string,
	kind: 'program' | 'break',
	starts: string,
	ends: string,
	extra: Partial<CouchTvAiring> = {}
) {
	return {
		id,
		kind,
		startsAt: starts,
		endsAt: ends,
		locked: false,
		ready: kind === 'break',
		source: 'auto',
		state: 'planned',
		creditsAtMs: null,
		...extra
	} as CouchTvAiring;
}

describe('the EPG grid', () => {
	it('starts the columns where the earliest broadcast day opens', () => {
		const every = ['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'] as const;
		expect(
			axisStart([{ key: 'a', days: [...every], start: '06:00', anchor: true, fill: 'reruns' }])
		).toBe(360);
		expect(
			axisStart([
				{ key: 'sat', days: ['sat'], start: '07:30', anchor: true, fill: 'reruns' },
				{
					key: 'rest',
					days: ['mon', 'tue', 'wed', 'thu', 'fri', 'sun'],
					start: '06:30',
					anchor: true,
					fill: 'reruns'
				}
			])
		).toBe(390);
		expect(axisStart([])).toBe(360);
	});

	it('makes a column of 24 hours of wall time, the clock change included', () => {
		const [sat, sun] = columns('2026-10-24', 2, 360, PRAGUE);
		expect(sat.start).toBe(Date.parse('2026-10-24T04:00:00Z'));
		expect(sat.end).toBe(sun.start);
		// the night the clocks go back has an hour more
		expect((sat.end - sat.start) / 3_600_000).toBe(25);
	});

	it('places airings in a column, cut at its edges', () => {
		const [col] = columns('2026-10-12', 1, 360, PRAGUE);
		const rows = [
			airing('a', 'program', '2026-10-12T03:30:00Z', '2026-10-12T04:30:00Z'),
			airing('b', 'break', '2026-10-12T04:30:00Z', '2026-10-12T04:35:00Z'),
			airing('c', 'program', '2026-10-13T03:50:00Z', '2026-10-13T04:20:00Z')
		];
		const programs = place(col, rows, 'program');
		expect(programs.map((p) => [p.airing.id, p.top, p.height, p.cutStart, p.cutEnd])).toEqual([
			['a', 0, 30, true, false],
			['c', 23 * 60 + 50, 10, false, true]
		]);
		expect(place(col, rows, 'break').map((p) => [p.top, p.height])).toEqual([[30, 5]]);
		expect(nowLine(col, Date.parse('2026-10-12T05:00:00Z'))).toBe(60);
		expect(nowLine(col, Date.parse('2026-10-11T05:00:00Z'))).toBeNull();
	});

	it('reads clock times on a column, the night after belonging to it', () => {
		expect(columnDate(Date.parse('2026-10-12T23:30:00Z'), 360, PRAGUE)).toBe('2026-10-12');
		expect(columnDate(Date.parse('2026-10-13T04:30:00Z'), 360, PRAGUE)).toBe('2026-10-13');
		expect(atClock('2026-10-12', '20:15', 360, PRAGUE)).toBe(Date.parse('2026-10-12T18:15:00Z'));
		expect(atClock('2026-10-12', '01:00', 360, PRAGUE)).toBe(Date.parse('2026-10-12T23:00:00Z'));
		expect(atClock('2026-10-12', 'soon', 360, PRAGUE)).toBeNaN();
	});

	it('puts an insert after a program on the next flow mark past the shortest break', () => {
		const a = airing('a', 'program', '2026-10-12T16:00:00Z', '2026-10-12T16:21:50Z');
		expect(afterTime(a, 23)).toBe(Date.parse('2026-10-12T16:25:00Z'));
		const b = airing('b', 'program', '2026-10-12T16:00:00Z', '2026-10-12T16:24:40Z');
		expect(afterTime(b, 23)).toBe(Date.parse('2026-10-12T16:30:00Z'));
	});

	it('counts what is ready and finds the neighbours of a program', () => {
		const rows = [
			airing('p1', 'program', '2026-10-12T10:00:00Z', '2026-10-12T10:20:00Z', { ready: true }),
			airing('b1', 'break', '2026-10-12T10:20:00Z', '2026-10-12T10:25:00Z', { render: 'ready' }),
			airing('p2', 'program', '2026-10-12T10:25:00Z', '2026-10-12T10:50:00Z'),
			airing('b2', 'break', '2026-10-12T10:50:00Z', '2026-10-12T10:55:00Z', { render: 'failed' }),
			airing('p3', 'program', '2026-10-12T10:55:00Z', '2026-10-12T11:20:00Z')
		];
		expect(
			readiness(rows, Date.parse('2026-10-12T10:00:00Z'), Date.parse('2026-10-12T11:00:00Z'))
		).toEqual({
			programs: 3,
			prepared: 1,
			breaks: 2,
			rendered: 1,
			failed: 1
		});
		const n = neighbours(rows, 'p2');
		expect([n.previous?.id, n.breakBefore?.id, n.breakAfter?.id, n.next?.id]).toEqual([
			'p1',
			'b1',
			'b2',
			'p3'
		]);
	});
});
