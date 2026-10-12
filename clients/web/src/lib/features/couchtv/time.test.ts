import { describe, expect, it } from 'vitest';
import {
	addDays,
	clockMinutes,
	clockOf,
	dateIn,
	formatLength,
	parseLength,
	wallTime,
	weekday,
	zonedTime
} from './time';

const PRAGUE = 'Europe/Prague';

describe('channel time', () => {
	it('reads the wall clock of an instant in the channel zone', () => {
		// 18:00 UTC is 20:00 in Prague in summer time
		expect(wallTime(Date.parse('2026-10-12T18:00:00Z'), PRAGUE)).toMatchObject({
			hour: 20,
			minute: 0,
			day: 12
		});
		expect(clockOf(Date.parse('2026-10-12T18:15:00Z'), PRAGUE)).toBe('20:15');
		expect(dateIn(Date.parse('2026-10-12T22:30:00Z'), PRAGUE)).toBe('2026-10-13');
	});

	it('finds the instant of a local time, across the clock changes', () => {
		expect(zonedTime('2026-10-12', 20 * 60 + 15, PRAGUE)).toBe(Date.parse('2026-10-12T18:15:00Z'));
		// the autumn change (25 October 2026): 06:00 is in winter time
		expect(zonedTime('2026-10-25', 6 * 60, PRAGUE)).toBe(Date.parse('2026-10-25T05:00:00Z'));
		expect(zonedTime('2026-10-24', 6 * 60, PRAGUE)).toBe(Date.parse('2026-10-24T04:00:00Z'));
		// the spring one skips 02:00-03:00; a skipped time lands past the gap
		expect(zonedTime('2026-03-29', 2 * 60 + 30, PRAGUE)).toBe(Date.parse('2026-03-29T01:30:00Z'));
	});

	it('counts dates and weekdays', () => {
		expect(addDays('2026-10-31', 1)).toBe('2026-11-01');
		expect(addDays('2026-01-01', -1)).toBe('2025-12-31');
		expect(weekday('2026-10-12')).toBe(1);
		expect(clockMinutes('20:15')).toBe(20 * 60 + 15);
		expect(clockMinutes('24:00')).toBeNaN();
	});

	it('writes and reads lengths', () => {
		expect(formatLength(83_400)).toBe('1:23');
		expect(formatLength(3_723_000)).toBe('1:02:03');
		expect(parseLength('1:23')).toBe(83_000);
		expect(parseLength('1:02:03.5')).toBe(3_723_500);
		expect(parseLength('95')).toBe(95_000);
		expect(parseLength('1:75')).toBeNaN();
		expect(parseLength('soon')).toBeNaN();
	});
});
