import { HttpFailureKind } from '$lib/generated/core';
import type { EffectOutput, HttpRequest, StoreRequest } from '$lib/generated/core';
import type { Executor } from './runtime.svelte';

// generous for a Raspberry Pi busy transcoding, but a hung request must not hold the app
const HTTP_TIMEOUT_MS = 30_000;

// next to the app's own `cv.*` keys
const STORE_PREFIX = 'cv.core.';

async function http({ method, url, headers, body }: HttpRequest): Promise<EffectOutput> {
	const abort = new AbortController();
	const timeout = setTimeout(() => abort.abort(), HTTP_TIMEOUT_MS);
	try {
		// same-origin like every other API call: the session cookie rides along and the
		// browser's Origin header satisfies the server's CSRF check
		const response = await fetch(url, {
			method,
			headers: headers.map((h): [string, string] => [h.name, h.value]),
			body,
			credentials: 'same-origin',
			signal: abort.signal
		});
		return { type: 'http', content: { status: response.status, body: await response.text() } };
	} catch (err) {
		const kind = abort.signal.aborted
			? HttpFailureKind.Timeout
			: navigator.onLine
				? HttpFailureKind.Other
				: HttpFailureKind.Offline;
		return { type: 'httpFailed', content: { kind, message: String(err) } };
	} finally {
		clearTimeout(timeout);
	}
}

// The web uploads images through its own profile editor until it adopts the core's (plan
// Phase 8), so the core never holds a file handle from it.
async function upload(): Promise<EffectOutput> {
	return {
		type: 'httpFailed',
		content: { kind: HttpFailureKind.Other, message: 'the web has handed the core no files' }
	};
}

function store({ key, op }: StoreRequest): EffectOutput {
	try {
		switch (op.type) {
			case 'read': {
				const value = localStorage.getItem(STORE_PREFIX + key);
				return { type: 'stored', content: value === null ? {} : { value } };
			}
			case 'write':
				localStorage.setItem(STORE_PREFIX + key, op.content);
				return { type: 'storeDone' };
			case 'delete':
				localStorage.removeItem(STORE_PREFIX + key);
				return { type: 'storeDone' };
		}
	} catch (err) {
		// private browsing or a full quota
		return { type: 'storeFailed', content: { message: String(err) } };
	}
}

// The web runs on its httpOnly session cookie and keeps no secrets of its own; writing one to
// localStorage would hand it to any script on the page.
function secureStore({ op }: StoreRequest): EffectOutput {
	switch (op.type) {
		case 'read':
			return { type: 'stored', content: {} };
		case 'write':
			return { type: 'storeFailed', content: { message: 'the web keeps no secrets' } };
		case 'delete':
			return { type: 'storeDone' };
	}
}

/** The browser's effects: fetch, localStorage and timers. */
export const browserExecutor: Executor = {
	http,
	upload,
	store,
	secureStore,
	timer({ afterMs, repeat }, fire) {
		if (repeat) {
			const interval = setInterval(fire, afterMs);
			return () => clearInterval(interval);
		}
		const timeout = setTimeout(fire, afterMs);
		return () => clearTimeout(timeout);
	}
};
