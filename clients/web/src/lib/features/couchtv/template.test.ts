import { describe, expect, it } from 'vitest';
import type { CouchTvConfig } from '$lib/generated/api';
import { configOf, draftOf, freshKey, problems, type SlotDraft } from './template';

const all = ['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'] as const;

function config(): CouchTvConfig {
	return {
		timeZone: 'Europe/Prague',
		breakMinSeconds: 23,
		guestLink: { enabled: true },
		bulletins: { enabled: true, cities: [{ name: 'Praha', lat: 50.088, lon: 14.4208 }] },
		rotation: {
			primeRepeatMinDays: 5,
			primeRepeatFactor: 0.8,
			movieGapDays: 2,
			rerunGapDays: 14,
			premiereWindowDays: 28,
			fillerMinMinutes: 5
		},
		template: {
			slots: [
				{
					key: 'morning',
					days: [...all],
					start: '06:00',
					anchor: true,
					fill: 'reruns',
					pick: 'comedy'
				},
				{
					key: 'sitcom-strip',
					days: [...all],
					start: '18:00',
					anchor: true,
					fill: 'strip',
					count: 2
				},
				{ key: 'prime-movie', days: [...all], start: '20:15', anchor: true, fill: 'movie' },
				{ key: 'overnight', days: [...all], start: '01:00', fill: 'reruns' }
			]
		}
	};
}

const slotsWith = (change: (slots: SlotDraft[]) => void) => {
	const draft = draftOf(config());
	change(draft.slots);
	return problems(draft.slots);
};

describe('template drafts', () => {
	it('round-trips a config, keeping the guest link', () => {
		const base = config();
		const back = configOf(draftOf(base), base);
		expect(back).toEqual(base);
	});

	it('drops what a fill does not use', () => {
		const base = config();
		const draft = draftOf(base);
		draft.slots[2].count = '3';
		draft.slots[1].titles = ['a', 'b'];
		const out = configOf(draft, base).template.slots;
		expect(out[2].count).toBeUndefined();
		expect(out[1].titles).toEqual(['a']);
	});

	it('names new slots after their fill', () => {
		const slots = draftOf(config()).slots;
		expect(freshKey(slots, 'saga')).toBe('saga');
		expect(freshKey(slots, 'reruns')).toBe('reruns');
		slots.push({ ...slots[0], key: 'reruns' });
		expect(freshKey(slots, 'reruns')).toBe('reruns-2');
	});
});

describe('template problems', () => {
	it('finds none in a good template', () => {
		expect(problems(draftOf(config()).slots)).toEqual([]);
	});

	it('finds what the server rejects', () => {
		expect(slotsWith((s) => (s[1].key = 'morning'))).toContainEqual({
			code: 'invalid_slot',
			slot: 1
		});
		expect(slotsWith((s) => (s[1].key = 'Strip!'))).toContainEqual({
			code: 'invalid_slot',
			slot: 1
		});
		expect(slotsWith((s) => (s[1].count = '21'))).toContainEqual({ code: 'invalid_slot', slot: 1 });
		expect(slotsWith((s) => (s[2].start = ''))).toContainEqual({
			code: 'anchor_needs_time',
			slot: 2
		});
		expect(slotsWith((s) => (s[0].anchor = false))).toContainEqual({
			code: 'day_needs_anchor',
			day: 'mon',
			slot: 0
		});
		expect(slotsWith((s) => (s[2].start = '17:00'))).toContainEqual({
			code: 'slot_out_of_order',
			day: 'mon',
			slot: 2
		});
		expect(
			slotsWith((s) => {
				for (const slot of s) slot.days = slot.days.filter((d) => d !== 'wed');
			})
		).toContainEqual({ code: 'day_without_slots', day: 'wed' });
		expect(
			slotsWith((s) => {
				s[3].start = '';
				s.push({ ...s[2], key: 'late', start: '', anchor: false });
			})
		).toContainEqual({ code: 'reruns_without_end', day: 'mon', slot: 3 });
	});

	it('reads times through the night from the first anchor', () => {
		// 01:00 comes after 20:15: it is the night after the day
		expect(slotsWith(() => {})).toEqual([]);
		expect(slotsWith((s) => (s[3].start = '05:59'))).toEqual([]);
		expect(slotsWith((s) => (s[3].start = '06:00'))).toContainEqual({
			code: 'slot_out_of_order',
			day: 'mon',
			slot: 3
		});
	});
});
