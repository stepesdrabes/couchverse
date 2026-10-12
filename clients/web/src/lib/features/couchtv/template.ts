// The weekly template as the editor holds it, and the checks the server makes on it
// (couchtv.CouchTvTemplate.validate), so the editor can point at a problem before Save.
import type {
	CouchTvConfig,
	CouchTvRotation,
	CouchTvSlot,
	CouchTvSlotDaysItem,
	CouchTvSlotFill,
	CouchTvSlotPick
} from '$lib/generated/api';
import { clockMinutes } from './time';

export type Day = CouchTvSlotDaysItem;

/** The week from Monday, as the template names its days. */
export const DAYS: Day[] = ['mon', 'tue', 'wed', 'thu', 'fri', 'sat', 'sun'];
export const FILLS: CouchTvSlotFill[] = [
	'strip',
	'marathon',
	'reruns',
	'movie',
	'matinee',
	'premiere',
	'saga',
	'encore'
];
export const PICKS: CouchTvSlotPick[] = ['any', 'comedy', 'animation', 'drama', 'family'];

export const ROTATION_KEYS: (keyof CouchTvRotation)[] = [
	'primeRepeatMinDays',
	'primeRepeatFactor',
	'movieGapDays',
	'rerunGapDays',
	'premiereWindowDays',
	'fillerMinMinutes'
];

/** Fills that air series (their pinned titles are series); the rest air movies. */
export const airsSeries = (fill: CouchTvSlotFill) =>
	fill === 'strip' || fill === 'marathon' || fill === 'reruns';

/** Fills that air a number of episodes a day; a strip or marathon also pins one series. */
export const inOrder = (fill: CouchTvSlotFill) => fill === 'strip' || fill === 'marathon';

/** A slot as the editor holds it: every part present, the count as its input's text. */
export interface SlotDraft {
	key: string;
	days: Day[];
	start: string;
	anchor: boolean;
	fill: CouchTvSlotFill;
	pick: CouchTvSlotPick;
	count: string;
	titles: string[];
}

export interface ConfigDraft {
	timeZone: string;
	breakMinSeconds: string;
	rotation: Record<keyof CouchTvRotation, string>;
	slots: SlotDraft[];
}

export function draftOf(config: CouchTvConfig): ConfigDraft {
	return {
		timeZone: config.timeZone,
		breakMinSeconds: String(config.breakMinSeconds),
		rotation: Object.fromEntries(
			ROTATION_KEYS.map((k) => [k, String(config.rotation[k])])
		) as Record<keyof CouchTvRotation, string>,
		slots: config.template.slots.map((s) => ({
			key: s.key,
			days: [...s.days],
			start: s.start ?? '',
			anchor: s.anchor ?? false,
			fill: s.fill,
			pick: s.pick ?? 'any',
			count: s.count ? String(s.count) : '',
			titles: [...(s.titles ?? [])]
		}))
	};
}

/** The config to save: the draft over what the server had (the guest link stays as it was). */
export function configOf(draft: ConfigDraft, base: CouchTvConfig): CouchTvConfig {
	const slots: CouchTvSlot[] = draft.slots.map((s) => {
		const slot: CouchTvSlot = {
			key: s.key.trim(),
			days: DAYS.filter((d) => s.days.includes(d)),
			fill: s.fill
		};
		if (s.start) slot.start = s.start;
		if (s.anchor) slot.anchor = true;
		if (s.pick !== 'any') slot.pick = s.pick;
		if (inOrder(s.fill) && s.count) slot.count = Number(s.count);
		if (s.titles.length) slot.titles = inOrder(s.fill) ? s.titles.slice(0, 1) : [...s.titles];
		return slot;
	});
	return {
		...base,
		timeZone: draft.timeZone,
		breakMinSeconds: Number(draft.breakMinSeconds),
		rotation: Object.fromEntries(
			ROTATION_KEYS.map((k) => [k, Number(draft.rotation[k])])
		) as unknown as CouchTvRotation,
		template: { slots }
	};
}

/** A key no slot has yet, after the fill it starts with. */
export function freshKey(slots: SlotDraft[], fill: CouchTvSlotFill): string {
	const taken = new Set(slots.map((s) => s.key));
	for (let n = 1; ; n++) {
		const key = n === 1 ? fill : `${fill}-${n}`;
		if (!taken.has(key)) return key;
	}
}

/** The slots that air on a day, in template order (its broadcast day). */
export function slotsOn(slots: SlotDraft[], day: Day): SlotDraft[] {
	return slots.filter((s) => s.days.includes(day));
}

/** What the server would reject, under the code it answers with (`problem_<code>`). */
export type ProblemCode =
	| 'invalid_slot'
	| 'anchor_needs_time'
	| 'day_without_slots'
	| 'day_needs_anchor'
	| 'slot_out_of_order'
	| 'reruns_without_end'
	| 'invalid_pinned_title';

export interface TemplateProblem {
	code: ProblemCode;
	/** the slot it is about, by its index in the template */
	slot?: number;
	day?: Day;
}

const KEY = /^[a-z0-9][a-z0-9-]*$/;

/** Every problem the server's validation would find, slot by slot, then day by day. */
export function problems(slots: SlotDraft[]): TemplateProblem[] {
	const out: TemplateProblem[] = [];
	const seen = new Set<string>();
	slots.forEach((s, slot) => {
		const key = s.key.trim();
		const count = s.count === '' ? 0 : Number(s.count);
		if (
			!KEY.test(key) ||
			key.length > 40 ||
			seen.has(key) ||
			s.days.length === 0 ||
			(s.start !== '' && Number.isNaN(clockMinutes(s.start))) ||
			(inOrder(s.fill) && (!Number.isInteger(count) || count < 0 || count > 20))
		) {
			out.push({ code: 'invalid_slot', slot });
		}
		seen.add(key);
		if (s.anchor && s.start === '') out.push({ code: 'anchor_needs_time', slot });
	});
	for (const day of DAYS) {
		const today = slots.map((s, i) => ({ s, i })).filter(({ s }) => s.days.includes(day));
		if (today.length === 0) {
			out.push({ code: 'day_without_slots', day });
			continue;
		}
		if (!today[0].s.anchor) {
			out.push({ code: 'day_needs_anchor', day, slot: today[0].i });
			continue;
		}
		// times run on through the night, from the day's first anchor
		const first = clockMinutes(today[0].s.start);
		let last = -1;
		today.forEach(({ s, i }, n) => {
			if (s.start !== '') {
				const at = (clockMinutes(s.start) - first + 24 * 60) % (24 * 60);
				if (at <= last) out.push({ code: 'slot_out_of_order', day, slot: i });
				last = Math.max(last, at);
			}
			if (
				s.fill === 'reruns' &&
				n + 1 < today.length &&
				!today.slice(n + 1).some(({ s: next }) => next.start !== '')
			) {
				out.push({ code: 'reruns_without_end', day, slot: i });
			}
		});
	}
	return out;
}
