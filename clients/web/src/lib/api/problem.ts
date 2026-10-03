import { ApiError } from './client';
import * as m from '$lib/paraglide/messages';

const messages = m as unknown as Record<string, unknown>;

/**
 * What to tell the user about a failed call: the shared `problem_<code>` string for an error
 * code the server documents, a network message when the server was out of reach, else
 * `fallback`. The server's own text is English and never shown.
 */
export function problemMessage(err: unknown, fallback: string): string {
	if (err instanceof ApiError) {
		const key = `problem_${err.code}`;
		const message = key in messages ? messages[key] : undefined;
		if (typeof message === 'function') return (message as () => string)();
	} else if (err instanceof TypeError) {
		// fetch rejects with a TypeError when no response came back
		return m.problem_offline();
	}
	return fallback;
}
