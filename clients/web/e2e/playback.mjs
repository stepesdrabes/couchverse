// Plays every movie of a running server in Chromium the way a viewer does: the
// page measures its device profile, posts it, and the player must start; then it
// switches subtitles, audio languages and every quality the menu offers, and
// finally follows a couch session anonymously. Prints one line per check and
// exits non-zero on a failure.
// usage: SERVER=http://localhost:8080 CV_USER=admin CV_PASSWORD=admin [CHANNEL=chrome] node e2e/playback.mjs
import { chromium } from 'playwright';

const server = process.env.SERVER ?? 'http://localhost:8080';
const credentials = {
	username: process.env.CV_USER ?? 'admin',
	password: process.env.CV_PASSWORD ?? 'admin'
};
// Playwright's own Chromium by default; CHANNEL=chrome uses the installed Google Chrome
const channel = process.env.CHANNEL || undefined;

const browser = await chromium.launch({
	channel,
	args: ['--autoplay-policy=no-user-gesture-required']
});
const context = await browser.newContext({
	baseURL: server,
	viewport: { width: 1280, height: 720 }
});
const login = await context.request.post('/api/v1/auth/login', { data: credentials });
if (!login.ok()) throw new Error(`login: ${login.status()}`);

let failures = 0;
let profileShown = false;
function report(ok, name, detail) {
	if (!ok) failures++;
	console.log(`${ok ? 'ok  ' : 'FAIL'} ${name}${detail ? `: ${detail}` : ''}`);
}

/** waits until the video has played `seconds` more than now */
async function advances(page, seconds = 1.5, timeout = 40_000) {
	const start = await page.evaluate(() => document.querySelector('video')?.currentTime ?? 0);
	await page.waitForFunction(
		([from, by]) => {
			const v = document.querySelector('video');
			return !!v && !v.error && v.currentTime > from + by;
		},
		[start, seconds],
		{ timeout }
	);
}

/** opens one of the player's menus and picks an option in it */
async function choose(page, menu, option) {
	// a menu stays open after a pick; close it so the trigger opens the next one
	await page.keyboard.press('Escape');
	await page.mouse.move(640, 400);
	await page.mouse.move(650, 410);
	await page.getByRole('button', { name: menu, exact: true }).click();
	await page
		.locator('[data-popover-content]')
		.getByRole('button', { name: option, exact: true })
		.click();
}

async function check(title) {
	const page = await context.newPage();
	const requests = [];
	page.on('request', (r) => requests.push(r.url()));
	const errors = [];
	// ingested samples stay drafts, whose title page (preloaded by the player) is a 404
	page.on('pageerror', (e) => e.message !== 'resource not found' && errors.push(e.message));
	const name = title.name;
	try {
		const payload = page.waitForResponse(
			(r) => r.url().includes('/api/v1/playback/movie/') && r.request().method() === 'POST'
		);
		await page.goto(`/watch/movie/${title.id}`);
		const response = await payload;
		if (!profileShown) {
			console.log(`profile ${response.request().postData()}`);
			profileShown = true;
		}
		const info = await response.json();
		if (info.mode === 'unsupported') {
			report(false, name, 'unsupported');
			return;
		}
		await advances(page);
		report(true, name, `${info.tier} via ${info.mode}`);

		if (info.subtitles.length > 0) {
			const sub = info.subtitles[0];
			await choose(page, 'Subtitles', sub.label);
			// the samples have cues at 1-4 s, 5.5-7.5 s and 20-23 s
			await page.evaluate(() => {
				const v = document.querySelector('video');
				if (v) v.currentTime = 5.6;
			});
			await page.waitForFunction(
				() => !!document.querySelector('.subtitle-overlay')?.textContent?.trim(),
				null,
				{ timeout: 20_000 }
			);
			report(true, `${name} subtitles`, sub.label);
		}

		if ((info.audio?.length ?? 0) > 1) {
			const other = info.audio.find((a) => !a.default) ?? info.audio[1];
			const seen = requests.length;
			await choose(page, 'Audio', other.label);
			await advances(page);
			const loaded = requests.slice(seen).filter((u) => /audio-\d+-|\/stream|\.m3u8/.test(u));
			report(loaded.length > 0, `${name} audio`, `${other.label} (${loaded.length} new requests)`);
		}

		// the menu appears once there is a choice: Original, and Auto with the ladder rungs
		const qualities = [];
		if (info.originalUrl) qualities.push('Original');
		if (info.hlsUrl && (info.variants?.length ?? 0) > 0) {
			qualities.push('Auto', ...info.variants.map((v) => `${v.height}p`));
		}
		for (const quality of qualities.length > 1 ? qualities : []) {
			await choose(page, 'Quality', quality);
			await advances(page);
			report(true, `${name} quality`, quality);
		}
		report(errors.length === 0, `${name} page errors`, errors.join('; '));
	} catch (err) {
		report(false, name, String(err).split('\n')[0]);
	} finally {
		await page.close();
	}
}

const library = await (
	await context.request.get('/api/v1/admin/library?type=movie&limit=200')
).json();
for (const title of library.items) await check(title);

// an anonymous couch follower plays what the host plays; the host speaks the
// couch protocol from a page of the signed-in context, which holds its couch cookie
const first = library.items.find((t) => t.name === 'Static Bloom') ?? library.items[0];
const couch = await (
	await context.request.post('/api/v1/couch', { data: { kind: 'movie', id: first.id } })
).json();
const host = await context.newPage();
await host.goto('/');
await host.evaluate(
	({ token, titleId }) => {
		const ws = new WebSocket(`${location.origin.replace('http', 'ws')}/api/v1/couch/${token}/ws`);
		const started = Date.now();
		const play = () =>
			ws.send(
				JSON.stringify({
					type: 'host_state',
					data: {
						media: { kind: 'movie', titleId },
						playing: true,
						positionSeconds: (Date.now() - started) / 1000
					}
				})
			);
		ws.onopen = () => setInterval(play, 2000);
	},
	{ token: couch.shareToken, titleId: first.id }
);
const guest = await browser.newContext({ baseURL: server });
const page = await guest.newPage();
try {
	await page.goto(`/couch/${couch.shareToken}`);
	await page.getByRole('button', { name: 'Start watching' }).click();
	await advances(page);
	report(true, `couch follower (${first.name})`);
} catch (err) {
	report(false, 'couch follower', String(err).split('\n')[0]);
}
await host.close();
await context.request.post(`/api/v1/couch/${couch.shareToken}/end`);

await browser.close();
console.log(failures ? `${failures} failures` : 'all passed');
process.exit(failures ? 1 : 0);
