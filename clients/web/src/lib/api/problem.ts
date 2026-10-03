import { ApiError } from './client';
import type { Problem } from '$lib/generated/core';
import * as m from '$lib/paraglide/messages';

const messages = m as unknown as Record<string, unknown>;

/**
 * What to tell the user about a failed call: the shared `problem_<code>` string for an error
 * code the server documents (from a web API call's `ApiError` or a core view's `Problem`), a
 * network message when the server was out of reach, else `fallback`. The server's own text is
 * English and never shown.
 */
export function problemMessage(err: unknown, fallback: string): string {
	if (err instanceof ApiError || isProblem(err)) {
		const key = `problem_${err.code}`;
		const message = key in messages ? messages[key] : undefined;
		if (typeof message === 'function') return (message as () => string)();
	} else if (err instanceof TypeError) {
		// fetch rejects with a TypeError when no response came back
		return m.problem_offline();
	}
	return fallback;
}

function isProblem(err: unknown): err is Problem {
	return typeof err === 'object' && err !== null && typeof (err as Problem).code === 'string';
}
