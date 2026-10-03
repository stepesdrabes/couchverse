import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import {
	test as base,
	expect,
	type BrowserContext,
	type Locator,
	type Page
} from '@playwright/test';

export { expect };

/** every seeded account shares the admin's password */
export const PASSWORD = 'admin';

/** nora is the seeded member; otto and vera are added by e2e/setup.sql for specs that write */
export type User = 'admin' | 'nora' | 'otto' | 'vera';
export const USERS: User[] = ['admin', 'nora', 'otto', 'vera'];

/** the signed-in cookie jar auth.setup.ts saves for a user */
export const authFile = (user: User) =>
	fileURLToPath(new URL(`.auth/${user}.json`, import.meta.url));

export const movie = {
	id: '00000000-0000-4000-8000-000000000101',
	slug: 'glass-harbor-2025',
	name: 'Glass Harbor',
	nameCs: 'Skleněný přístav',
	overview: 'A lighthouse keeper finds a door in the sea.',
	overviewCs: 'Strážce majáku najde dveře v moři.',
	/** its logo artwork per display language */
	logos: {
		en: '00000000-0000-4000-8000-000000000407',
		cs: '00000000-0000-4000-8000-000000000408'
	}
};

export const series = {
	id: '00000000-0000-4000-8000-000000000102',
	slug: 'static-bloom-2024',
	name: 'Static Bloom'
};

export const draft = { id: '00000000-0000-4000-8000-000000000103', name: 'Unfinished' };

type Lang = 'en' | 'cs';
const catalogs = Object.fromEntries(
	(['en', 'cs'] as const).map((lang) => [
		lang,
		JSON.parse(
			readFileSync(new URL(`../../../contract/i18n/${lang}.json`, import.meta.url), 'utf8')
		) as Record<string, unknown>
	])
) as Record<Lang, Record<string, unknown>>;

/** a UI string from the shared message catalogs, so copy edits do not break the suite */
export function t(key: string, params: Record<string, string | number> = {}, lang: Lang = 'en') {
	const message = catalogs[lang][key];
	if (typeof message !== 'string') throw new Error(`no plain message "${key}" in ${lang}`);
	return message.replace(/\{(\w+)\}/g, (_, name: string) => String(params[name]));
}

/** a home/browse row, found by its heading */
export const row = (page: Page, label: string): Locator =>
	page.locator('section').filter({ has: page.getByRole('heading', { name: label, exact: true }) });

/** the player's video element's current position */
export const videoTime = (page: Page) =>
	page.locator('video').evaluate((v: HTMLVideoElement) => v.currentTime);

export const videoPaused = (page: Page) =>
	page.locator('video').evaluate((v: HTMLVideoElement) => v.paused);

/** how long a player may take to fetch, buffer and decode before its clock moves on a busy machine */
export const MEDIA_TIMEOUT = 20_000;

const EXPECTED = [
	// a signed-out page's boot: the core reads the session and flags, the web its
	// preferences, and 401 is the expected answer to each
	/status of 401 .*\/api\/v1\/(auth\/me|features|me\/preferences)\b/,
	// the host OS reporting a network change (VPN, Wi-Fi) aborts requests in flight
	/net::ERR_NETWORK_CHANGED/
];

class ErrorWatch {
	readonly seen: string[] = [];
	private readonly allowed = [...EXPECTED];

	/** collect uncaught exceptions and console errors from every page of a context */
	watch(context: BrowserContext) {
		context.on('weberror', (e) => this.seen.push(`uncaught: ${e.error().stack ?? e.error()}`));
		context.on('console', (msg) => {
			if (msg.type() !== 'error') return;
			const line = `console: ${msg.text()} @ ${msg.location().url}`;
			if (!this.allowed.some((re) => re.test(line))) this.seen.push(line);
		});
	}

	/** tolerate an error a test provokes on purpose */
	allow(pattern: RegExp) {
		this.allowed.push(pattern);
	}
}

/** every test fails on an uncaught page error or a console error it did not expect */
export const test = base.extend<{ errors: ErrorWatch }>({
	errors: [
		async ({ context }, use) => {
			const errors = new ErrorWatch();
			errors.watch(context);
			await use(errors);
			expect(errors.seen, 'uncaught page errors or console errors').toEqual([]);
		},
		{ auto: true }
	]
});
