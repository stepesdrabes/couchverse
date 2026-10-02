import { currentLang } from '$lib/i18n/locale.svelte';

export class ApiError extends Error {
	constructor(
		public status: number,
		public code: string,
		message: string
	) {
		super(message);
	}
}

interface RequestOptions {
	method?: string;
	/** sent as JSON, except a FormData (multipart) and bytes (application/octet-stream) */
	body?: unknown;
	signal?: AbortSignal;
	/** skip the global 401 → login redirect (e.g. the initial /auth/me probe) */
	skipAuthRedirect?: boolean;
	/** omit the display-language (?lang=) query param */
	skipLang?: boolean;
}

/** the options a generated operation passes through to `api()` */
export type CallOptions = Omit<RequestOptions, 'method' | 'body'>;

function encodeBody(body: unknown): { body?: BodyInit; contentType?: string } {
	if (body === undefined) return {};
	// the browser writes the multipart boundary into the Content-Type itself
	if (body instanceof FormData) return { body };
	if (body instanceof Blob || body instanceof ArrayBuffer || ArrayBuffer.isView(body)) {
		return { body: body as BodyInit, contentType: 'application/octet-stream' };
	}
	return { body: JSON.stringify(body), contentType: 'application/json' };
}

// registered by the session module to avoid a circular import
let unauthorizedHandler: (() => void) | null = null;
export function onUnauthorized(handler: () => void) {
	unauthorizedHandler = handler;
}

/** build a query string, skipping empty/undefined params */
export function qs(params: Record<string, string | number | undefined>) {
	const search = new URLSearchParams();
	for (const [key, value] of Object.entries(params)) {
		if (value !== undefined && value !== '') search.set(key, String(value));
	}
	const s = search.toString();
	return s ? `?${s}` : '';
}

export async function api<T>(path: string, opts: RequestOptions = {}): Promise<T> {
	// the display language rides on every request; the backend honours it only on
	// public catalog reads (admin/auth ignore it), so sending it globally is safe
	const sep = path.includes('?') ? '&' : '?';
	const url = opts.skipLang ? `/api/v1${path}` : `/api/v1${path}${sep}lang=${currentLang()}`;
	const { body, contentType } = encodeBody(opts.body);
	const res = await fetch(url, {
		method: opts.method ?? 'GET',
		headers: contentType ? { 'Content-Type': contentType } : undefined,
		body,
		credentials: 'same-origin',
		signal: opts.signal
	});

	if (!res.ok) {
		let code = 'unknown';
		let message = res.statusText;
		try {
			const data = await res.json();
			code = data.error?.code ?? code;
			message = data.error?.message ?? message;
		} catch {
			// non-JSON error body
		}
		if (res.status === 401 && !opts.skipAuthRedirect) {
			unauthorizedHandler?.();
		}
		throw new ApiError(res.status, code, message);
	}

	if (res.status === 204) return undefined as T;
	return res.json();
}
