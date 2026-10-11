// English airing phrases (D13), as the server is expected to format them. The admin preview
// uses this to build specs; the server keeps its own copy of the same rules.

import type { Airing, ClockTime, Weekday } from './spec';

export const DAY_NAMES = [
	'Monday',
	'Tuesday',
	'Wednesday',
	'Thursday',
	'Friday',
	'Saturday',
	'Sunday'
];

export type AiringKind = 'tonight' | 'tomorrow' | 'day' | 'premiere' | 'weekdays' | 'nightly';

export function airing(kind: AiringKind, time: ClockTime, day: Weekday = 5): Airing {
	switch (kind) {
		case 'tonight':
			return { eyebrow: 'Tonight', time };
		case 'tomorrow':
			return { eyebrow: 'Tomorrow', time };
		case 'day':
			return { eyebrow: 'This week', day: DAY_NAMES[day], time };
		case 'premiere':
			return { eyebrow: 'Premiere', day: DAY_NAMES[day], time };
		case 'weekdays':
			return { eyebrow: 'Every weekday', time };
		case 'nightly':
			return { eyebrow: 'Every night', time };
	}
}
