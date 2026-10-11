// Drives the render page the way the sidecar does: load a spec, seek, capture.

import { resolve } from 'node:path';
import type { Page } from '@playwright/test';
import type { LoadResult } from '../../src/render';
import type { Spec } from '../../src/spec';

const MEDIA = resolve(import.meta.dirname, '../media');

const errors = new WeakMap<Page, string[]>();

/** Errors the page threw or logged (a spec asserts there are none). */
export const pageErrors = (page: Page) => errors.get(page) ?? [];

/** Open the render page with test/media served from disk. */
export async function openRender(page: Page) {
	const seen: string[] = [];
	errors.set(page, seen);
	page.on('pageerror', (err) => seen.push(err.message));
	page.on('console', (m) => {
		if (m.type() === 'error') seen.push(m.text());
	});
	await page.route('**/test/media/**', (route) => {
		const path = new URL(route.request().url()).pathname.replace(/^\/test\/media\//, '');
		return route.fulfill({ path: resolve(MEDIA, decodeURIComponent(path)) });
	});
	await page.goto('/render.html');
	await page.waitForFunction(() => !!window.couchtvRender);
}

export function load(page: Page, spec: Spec): Promise<LoadResult> {
	return page.evaluate((s) => window.couchtvRender.load(s), spec);
}

export function seek(page: Page, t: number) {
	return page.evaluate((x) => window.couchtvRender.seek(x), t);
}

/** Two renders of the loaded spec's sound, compared in the page. */
export function renderTwice(page: Page) {
	return page.evaluate(async () => {
		const decode = (b64: string) => {
			const bytes = Uint8Array.from(atob(b64), (c) => c.charCodeAt(0));
			return new Float32Array(bytes.buffer.slice(44));
		};
		const a = decode(await window.couchtvRender.renderAudioBase64());
		const report = window.couchtvRender.audioReport();
		const b = decode(await window.couchtvRender.renderAudioBase64());
		let maxDiff = 0;
		for (let i = 0; i < a.length; i++) maxDiff = Math.max(maxDiff, Math.abs(a[i] - b[i]));
		// how far the loop's end is from its start, against the typical step between samples
		const frames = a.length / 2;
		let step = 0;
		for (let i = 2; i < a.length; i += 2) step += Math.abs(a[i] - a[i - 2]);
		const seam = Math.abs(a[0] - a[a.length - 2]);
		return {
			frames,
			maxDiff,
			report,
			seam,
			meanStep: step / frames,
			length: a.length === b.length
		};
	});
}
