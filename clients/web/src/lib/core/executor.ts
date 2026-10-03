import { HttpFailureKind } from '$lib/generated/core';
import type {
	EffectOutput,
	HttpRequest,
	SocketOpen,
	StoreRequest,
	UploadRequest
} from '$lib/generated/core';
import { takeFile } from './files';
import type { Executor, Socket } from './runtime.svelte';

// generous for a Raspberry Pi busy transcoding, but a hung request must not hold the app
const HTTP_TIMEOUT_MS = 30_000;
// a phone photo over a slow uplink
const UPLOAD_TIMEOUT_MS = 5 * 60_000;

// next to the app's own `cv.*` keys
const STORE_PREFIX = 'cv.core.';

const KEEPALIVE_MAX_BODY = 60_000;

async function send(
	{ method, url, headers }: HttpRequest,
	body: string | FormData | undefined,
	options: { timeoutMs: number; keepalive: boolean }
): Promise<EffectOutput> {
	const abort = new AbortController();
	const timeout = setTimeout(() => abort.abort(), options.timeoutMs);
	try {
		// same-origin like every other API call: the session cookie rides along and the
		// browser's Origin header satisfies the server's CSRF check
		const response = await fetch(url, {
			method,
			headers: headers.map((h): [string, string] => [h.name, h.value]),
			body,
			credentials: 'same-origin',
			signal: abort.signal,
			keepalive: options.keepalive
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

function http(request: HttpRequest): Promise<EffectOutput> {
	const body = request.body;
	return send(request, body, {
		timeoutMs: HTTP_TIMEOUT_MS,
		// what the player saves as the page goes away (progress, stopping a transcode) must
		// outlive it; the browser caps such requests at 64 KB of body
		keepalive: request.method !== 'GET' && (body?.length ?? 0) < KEEPALIVE_MAX_BODY
	});
}

/** The picked file as a form's one part, under its own name: the server types images by it. */
async function upload({ request, file, field }: UploadRequest): Promise<EffectOutput> {
	const picked = takeFile(file);
	if (!picked) {
		const message = `no picked file ${file}`;
		return { type: 'httpFailed', content: { kind: HttpFailureKind.Other, message } };
	}
	const form = new FormData();
	form.set(field, picked, picked.name);
	// the browser writes the multipart content type, boundary included
	const headers = request.headers.filter((h) => h.name.toLowerCase() !== 'content-type');
	return send({ ...request, headers }, form, { timeoutMs: UPLOAD_TIMEOUT_MS, keepalive: false });
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

// The core names a socket by path on the web; the page's origin picks ws or wss. Browsers
// cannot set headers on a WebSocket, and the web's cookies need none.
function socket({ url }: SocketOpen, emit: (output: EffectOutput) => void): Socket {
	const target = new URL(url, location.href);
	target.protocol = target.protocol === 'https:' ? 'wss:' : target.protocol.replace('http', 'ws');
	const ws = new WebSocket(target);
	let closed = false;
	ws.onopen = () => emit({ type: 'socketOpened' });
	ws.onmessage = (e) => {
		if (typeof e.data === 'string') emit({ type: 'socketText', content: { text: e.data } });
	};
	ws.onclose = (e) => {
		if (closed) return;
		closed = true;
		emit({ type: 'socketClosed', content: { code: e.code, reason: e.reason } });
	};
	return {
		send(text) {
			if (ws.readyState === WebSocket.OPEN) ws.send(text);
		},
		close() {
			closed = true;
			ws.close();
		}
	};
}

/** The browser's effects: fetch, localStorage, timers and WebSockets. */
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
	},
	socket
};
