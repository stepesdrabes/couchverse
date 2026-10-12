import { describe, expect, it, vi } from 'vitest';
import {
	CouchTvConnection,
	CouchTvItemKind,
	CouchTvStatus,
	type CouchTvProgram,
	type CouchTvView
} from '$lib/generated/core';
import {
	clock,
	guideHours,
	guideOffset,
	guideRows,
	HOUR_REM,
	livePlayerView,
	pictureDate,
	progressAt
} from './channel';

// the helpers are pure; the events they sit beside need no running core here
vi.mock('$lib/core', () => ({ core: {} }));

const at = (iso: string) => Date.parse(iso);

function program(id: string, starts: string, ends: string): CouchTvProgram {
	return {
		id,
		kind: CouchTvItemKind.Program,
		startsAtMs: at(starts),
		endsAtMs: at(ends),
		title: id
	};
}

function channel(view: Partial<CouchTvView> = {}): CouchTvView {
	return {
		status: CouchTvStatus.OnAir,
		connection: CouchTvConnection.Open,
		later: [],
		programDateAtMs: 0,
		cues: { bug: true, nowCaption: false, nextCaption: false },
		viewers: 1,
		members: [],
		reactions: [],
		audio: [],
		subtitles: [],
		joining: false,
		...view
	};
}

describe('the live channel', () => {
	it('runs the picture clock on from its last report', () => {
		const view = channel({ programDateMs: 1_000_000, programDateAtMs: 5_000 });
		expect(pictureDate(view, 5_000)).toBe(1_000_000);
		expect(pictureDate(view, 8_500)).toBe(1_003_500);
		// a render stamped after the shell's clock read never runs it backwards
		expect(pictureDate(view, 4_000)).toBe(1_000_000);
		expect(pictureDate(channel(), 8_500)).toBeUndefined();
	});

	it('says how far a program has come', () => {
		const p = program('a', '2026-10-12T18:00:00Z', '2026-10-12T18:30:00Z');
		expect(progressAt(p, at('2026-10-12T18:15:00Z'))).toBe(0.5);
		expect(progressAt(p, at('2026-10-12T17:00:00Z'))).toBe(0);
		expect(progressAt(p, at('2026-10-12T19:00:00Z'))).toBe(1);
	});

	it('shows times on the channel clock', () => {
		const ms = new Date(2026, 9, 12, 20, 15).getTime();
		expect(clock(ms, 'en')).toBe('20:15');
		expect(clock(ms, 'cs')).toBe('20:15');
	});

	it('is the player chrome of what is on', () => {
		const view = channel({
			now: { ...program('Glass Harbor', '2026-10-12T18:00:00Z', '2026-10-12T19:00:00Z') },
			audio: [
				{ id: 'cs', lang: 'cs', selected: false },
				{ id: 'en', lang: 'en', selected: true }
			],
			subtitles: [
				{ id: 'cs', lang: 'cs', selected: false },
				{ id: 'en', lang: 'en', selected: false }
			]
		});
		const player = livePlayerView(view);
		expect(player).toMatchObject({ title: 'Glass Harbor', linear: true, audioSelected: 'en' });
		expect(player.audio.map((t) => t.label)).toEqual(['Čeština', 'English']);
		expect(player.subtitleSelected).toBeUndefined();
		// a break carries the channel's name
		const brk = { ...view.now!, kind: CouchTvItemKind.Break, title: undefined };
		expect(livePlayerView(channel({ now: brk })).title).toBe('Couch TV');
	});
});

describe('the guide', () => {
	const day = {
		date: '2026-10-12',
		startsAtMs: at('2026-10-12T04:00:00Z'),
		endsAtMs: at('2026-10-13T04:00:00Z'),
		programs: [
			// the night before runs into the day: it is cut at the day's start
			program('late', '2026-10-12T03:30:00Z', '2026-10-12T04:30:00Z'),
			program('prime', '2026-10-12T18:15:00Z', '2026-10-12T20:15:00Z')
		]
	};

	it('places programs on their day by time', () => {
		const [row] = guideRows([day]);
		expect(row.programs.map((p) => [p.program.id, p.left, p.width])).toEqual([
			['late', 0, HOUR_REM / 2],
			['prime', 14.25 * HOUR_REM, 2 * HOUR_REM]
		]);
	});

	it('puts the now-line on its day only', () => {
		expect(guideOffset(day, at('2026-10-12T05:00:00Z'))).toBe(HOUR_REM);
		expect(guideOffset(day, at('2026-10-13T05:00:00Z'))).toBeUndefined();
	});

	it('spans the longest day, a clock change included', () => {
		const autumn = { startsAtMs: at('2026-10-25T04:00:00Z'), endsAtMs: at('2026-10-26T05:00:00Z') };
		expect(guideHours([day])).toBe(24);
		expect(guideHours([day, autumn])).toBe(25);
	});
});
