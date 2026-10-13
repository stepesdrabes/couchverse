import { expect, test } from '@playwright/test';
import { COUCH, LOGO, LOGO_GAP, POPCORN, SILHOUETTE } from '../../src/brand/glyph';
import { PARTS } from '../../src/brand/wordmark';
import type { ClipSpec, PromoSpec, SqueezeSpec, UpNextSpec } from '../../src/spec';
import { FIXTURES, TITLES, type FixtureName } from '../fixtures/specs';
import { load, openRender, pageErrors, seek } from './render';

// chosen frames of every template (seconds into the clip)
const GOLDEN: [FixtureName, number[], ('picture' | 'matte')?][] = [
	// the full idents: early, mid-way and the held lockup
	['ident-journey', [1.2, 3.2, 9]],
	['ident-cinematic', [1, 3.6, 9]],
	['ident-morph', [1.6, 4.6, 9.5]],
	['ident-assemble', [1.6, 4.4, 9]],
	// the couch alone, the popcorn falling (motion blurred), the landing's rebound, the lockup
	['ident-short', [0, 1.04, 1.28, 4.48]],
	['sting-2', [0.2, 0.84, 1.96]],
	['sting-3', [0.24, 1.2, 2.96]],
	['sting-5', [0.16, 1.4, 2.48, 4.96]],
	// up next over scenes; a logo in place of the name, the house's score
	['upnext', [2.5, 7.6]],
	['upnext-logo', [0.6, 3]],
	// promos: a tagline, the title (a logo or the name), the panels closing, the end card
	['promo-cinematic', [2.5, 13.4, 15.6, 19.5]],
	['promo-grit-premiere', [4.4, 8, 16]],
	['promo-bright', [1.5, 4.4, 10.6, 16]],
	['promo-premiere', [12.4, 20.6]],
	['evergreen', [2.5]],
	// the panel sliding in, the week lit, the tag on the clean field
	['strip-bright', [0.5, 7, 13.5]],
	['strip-dark', [7, 13.5]],
	// the rush, the swipe on the hit, the card
	['short', [0.6, 1.2, 3.5]],
	['short-logo', [3.5]],
	['lineup', [1, 6]],
	// the hold, the push into the card, its last frame (up next's first)
	['squeeze-expand', [5, 10.4, 11.48]],
	['squeeze-key', [5]],
	['squeeze-key', [2.6], 'matte'],
	['squeeze-frames', [1, 5]],
	['upnext-continue', [0, 2.5]],
	['standby', [2]],
	// the longest names and lines: fitted, broken onto lines, never off the frame
	['promo-cinematic-long', [3.4, 7, 13.5, 19.5]],
	['promo-grit-long', [1.2, 4.4, 8, 16]],
	['promo-bright-long', [2.4, 4.6, 10.8, 16]],
	['evergreen-long', [3]],
	// no artwork and no scenes: the painted dusk, in grit's letterbox and behind the end card
	['promo-zabky', [3, 8, 16]],
	['upnext-long', [3]],
	['strip-long', [8]],
	['strip-long-bright', [6]],
	['short-long', [4.2]],
	['lineup-long', [4.4, 7]],
	['squeeze-long', [5]],
	// the bulletins' scenes: now, tomorrow and the next days; a story mid-bulletin
	['weather', [3, 8, 13]],
	['weather-quiet', [3]],
	['weather-unread', [3]],
	['news', [3, 9]],
	['news-three', [3]],
	// every default city and a warning of each level; the longest headlines
	['weather-long', [3, 8]],
	['news-long', [3]]
];

test.beforeEach(async ({ page }) => openRender(page));
test.afterEach(({ page }) => expect(pageErrors(page)).toEqual([]));

for (const [name, times, layer] of GOLDEN) {
	test(`${name}${layer ? ` (${layer})` : ''}`, async ({ page }) => {
		const info = await load(page, FIXTURES[name]);
		expect(info.missing).toEqual([]);
		expect(Math.abs(info.duration * 25 - Math.round(info.duration * 25))).toBeLessThan(1e-6);
		if (layer) await page.evaluate((l) => window.couchtvRender.setLayer(l), layer);
		for (const t of times) {
			await seek(page, t);
			await expect(page).toHaveScreenshot(`${name}${layer ? `-${layer}` : ''}-${t.toFixed(2)}.png`);
		}
	});
}

test('text stays inside the title-safe width and on the frame', async ({ page }) => {
	for (const [name, times, layer] of GOLDEN) {
		if (layer) continue;
		await load(page, FIXTURES[name]);
		for (const t of times) {
			await seek(page, t);
			const outside = await page.evaluate(() => {
				const stage = document.getElementById('stage')?.shadowRoot?.querySelector('.stage');
				const piece = stage?.querySelector('.piece');
				if (!stage || !piece) return ['no piece'];
				const out: string[] = [];
				const range = document.createRange();
				const walker = document.createTreeWalker(piece, NodeFilter.SHOW_TEXT);
				for (let n = walker.nextNode(); n; n = walker.nextNode()) {
					const el = n.parentElement;
					// the corner bug sits in the action-safe corner by design, and the squeeze's panel
					// is the program's own picture (the preview's credits roll), which may leave it
					if (!el || !n.textContent?.trim() || el.closest('.bug, .program')) continue;
					if (getComputedStyle(el).visibility === 'hidden') continue;
					let opacity = 1;
					const clips: DOMRect[] = [];
					for (let a: Element | null = el; a && a !== piece; a = a.parentElement) {
						const s = getComputedStyle(a);
						opacity *= parseFloat(s.opacity);
						if (s.overflow !== 'visible') clips.push(a.getBoundingClientRect());
					}
					// only what reads: the tail of a fade does not
					if (opacity < 0.5) continue;
					range.selectNodeContents(n);
					for (const r of range.getClientRects()) {
						const box = clips.reduce(
							(b, c) => ({
								l: Math.max(b.l, c.left),
								r: Math.min(b.r, c.right),
								t: Math.max(b.t, c.top),
								b: Math.min(b.b, c.bottom)
							}),
							{ l: r.left, r: r.right, t: r.top, b: r.bottom }
						);
						if (box.r - box.l < 1 || box.b - box.t < 1) continue;
						if (box.l < 95.5 || box.r > 1824.5 || box.t < 0 || box.b > 1080)
							out.push(`${n.textContent} [${Math.round(box.l)}..${Math.round(box.r)}]`);
					}
				}
				return out;
			});
			expect(outside, `${name} at ${t}`).toEqual([]);
		}
	}
});

test('the standby loop ends on its first frame', async ({ page }) => {
	const info = await load(page, FIXTURES.standby);
	await seek(page, 0);
	const first = await page.screenshot();
	await seek(page, info.duration);
	expect((await page.screenshot()).equals(first)).toBe(true);
});

test('versions name every template as load reports it', async ({ page }) => {
	const versions = await page.evaluate(() => window.couchtvRender.versions());
	for (const name of ['ident-short', 'upnext', 'promo-bright', 'lineup', 'standby'] as const) {
		const spec = FIXTURES[name];
		expect((await load(page, spec)).version, name).toBe(versions[spec.template]);
	}
});

test('a frame is a function of t alone', async ({ page }) => {
	for (const [name, t] of [
		['promo-cinematic', 6.2],
		['lineup', 4.4],
		['squeeze-frames', 0.6],
		['strip-dark', 9.3]
	] as const) {
		await load(page, FIXTURES[name]);
		await seek(page, t);
		const first = await page.screenshot();
		await seek(page, t + 2.3);
		await seek(page, 0);
		await seek(page, t);
		expect((await page.screenshot()).equals(first), name).toBe(true);
	}
});

test("the renderer's step shows the frame seek does and skips held ones", async ({ page }) => {
	const info = await load(page, FIXTURES['ident-short']);
	const frames = Math.round(info.duration * 25);
	const held = await page.evaluate((n) => {
		for (let i = 1; i < n; i++)
			if (window.couchtvRender.isStatic((i - 1) / 25, i / 25)) return i / 25;
		return null;
	}, frames);
	// the lockup holds before the short ident ends
	expect(held).not.toBeNull();
	if (held === null) return;
	expect(await page.evaluate((t) => window.couchtvRender.advance(t, t - 0.04), held)).toBe(false);
	for (const t of [1.04, held]) {
		await seek(page, t);
		const seeked = await page.screenshot();
		await seek(page, 0);
		expect(await page.evaluate((x) => window.couchtvRender.advance(x, -1), t)).toBe(true);
		expect((await page.screenshot()).equals(seeked), `t=${t}`).toBe(true);
	}
});

// a full ident while it moves (a render averages sub-frames) and the frame its lockup holds
// from; cinematic's film grain moves to the end
const HELD: [FixtureName, number, number | null][] = [
	['ident-journey', 3.2, 197],
	['ident-cinematic', 3.6, null],
	['ident-morph', 4.6, 224],
	['ident-assemble', 4.4, 201]
];

for (const [name, moving, held] of HELD) {
	test(`${name}: the renderer's step shows the frame seek does and skips the hold`, async ({
		page
	}) => {
		const info = await load(page, FIXTURES[name]);
		expect(info.duration).toBeCloseTo(10.52, 6);
		const frames = Math.round(info.duration * 25);
		// the first frame from which every frame is the one before it
		const hold = await page.evaluate((n) => {
			let from: number | null = null;
			for (let i = n - 1; i > 0; i--) {
				if (!window.couchtvRender.isStatic((i - 1) / 25, i / 25)) break;
				from = i;
			}
			return from;
		}, frames);
		expect(hold).toBe(held);
		for (const t of held === null ? [moving] : [moving, held / 25]) {
			await seek(page, t);
			const seeked = await page.screenshot();
			await seek(page, 0);
			expect(await page.evaluate((x) => window.couchtvRender.advance(x, -1), t)).toBe(true);
			expect((await page.screenshot()).equals(seeked), `t=${t}`).toBe(true);
		}
		if (held !== null)
			expect(
				await page.evaluate((t) => window.couchtvRender.advance(t, t - 0.04), (held + 1) / 25)
			).toBe(false);
	});
}

test('a promo body does not depend on the airing', async ({ page }) => {
	for (const mood of ['promo-cinematic', 'promo-grit-premiere', 'promo-bright'] as const) {
		const base = FIXTURES[mood] as PromoSpec;
		const shots: Buffer[] = [];
		for (const airing of [
			base.airing,
			{ eyebrow: 'Every Saturday', day: 'Saturday', time: '23:45' }
		]) {
			const info = await load(page, { ...base, part: 'body', airing } satisfies ClipSpec);
			await seek(page, info.duration - 0.04);
			shots.push(await page.screenshot());
		}
		expect(shots[0].equals(shots[1]), mood).toBe(true);
	}
});

test('the end card follows the body on the next frame', async ({ page }) => {
	const base = FIXTURES['promo-cinematic'] as PromoSpec;
	const full = await load(page, { ...base, part: 'full' });
	const body = await load(page, { ...base, part: 'body' });
	const end = await load(page, { ...base, part: 'end' });
	expect(body.duration + end.duration).toBeCloseTo(full.duration, 6);
	expect(end.duration).toBeCloseTo(4, 6);
	await seek(page, 0);
	const endFirst = await page.screenshot();
	await load(page, { ...base, part: 'full' });
	await seek(page, body.duration);
	expect((await page.screenshot()).equals(endFirst)).toBe(true);
});

test('the key rectangle follows the program panel', async ({ page }) => {
	await load(page, FIXTURES['squeeze-key']);
	for (const t of [0.5, 2.4, 2.9, 5, 9.6, 10.0]) {
		await seek(page, t);
		const { rect, box } = await page.evaluate((x) => {
			const panel = document.getElementById('stage')?.shadowRoot?.querySelector('.program');
			const r = panel?.getBoundingClientRect();
			return {
				rect: window.couchtvRender.keyRect(x),
				box: r ? { x: r.x, y: r.y, width: r.width, height: r.height } : null
			};
		}, t);
		expect(rect, `t=${t}`).not.toBeNull();
		if (!rect || !box) continue;
		for (const k of ['x', 'y', 'width', 'height'] as const)
			expect(Math.abs(rect[k] - box[k]), `${k} at ${t}`).toBeLessThan(1.5);
	}
	expect(await page.evaluate(() => window.couchtvRender.keyRect(10.56))).toBeNull();
});

test("the squeeze hands over on the short ident's first frame", async ({ page }) => {
	const squeeze: SqueezeSpec = {
		...(FIXTURES['squeeze-expand'] as SqueezeSpec),
		handover: 'ident'
	};
	const info = await load(page, squeeze);
	expect(info.duration).toBeCloseTo(10.6, 6);
	await seek(page, info.duration - 0.04);
	const last = await page.screenshot();
	await load(page, FIXTURES['ident-short']);
	await seek(page, 0);
	expect((await page.screenshot()).equals(last)).toBe(true);
});

test("the expanding squeeze's last frame is up next's first", async ({ page }) => {
	// over the backdrop: scenes would move on by the frame between them
	const squeeze = FIXTURES['squeeze-expand'] as SqueezeSpec;
	const title = TITLES.sunny;
	const info = await load(page, { ...squeeze, next: { ...squeeze.next, title } });
	expect(info.duration).toBeCloseTo(11.52, 6);
	// both against one golden: a static transform and up next's push rasterize a level apart
	await seek(page, info.duration - 0.04);
	await expect(page).toHaveScreenshot('expand-handover.png');
	await load(page, { ...(FIXTURES['upnext-continue'] as UpNextSpec), title });
	await seek(page, 0);
	await expect(page).toHaveScreenshot('expand-handover.png');
});

test('the corner bug', async ({ browser }) => {
	const page = await browser.newPage({
		viewport: { width: 1920, height: 1080 },
		deviceScaleFactor: 1
	});
	await openRender(page);
	await load(page, FIXTURES.upnext);
	await seek(page, 2.5);
	await expect(page).toHaveScreenshot('bug.png', {
		clip: { x: 1340, y: 40, width: 520, height: 88 }
	});
	expect(pageErrors(page)).toEqual([]);
	await page.close();
});

test('the couch and the popcorn with its knockout are the logo', async ({ page }) => {
	const diff = await page.evaluate(
		({ couch, popcorn, silhouette, logo, gap }) => {
			const scale = 2;
			const draw = (fn: (g: CanvasRenderingContext2D) => void) => {
				const c = document.createElement('canvas');
				c.width = 1046 * scale;
				c.height = 745 * scale;
				const g = c.getContext('2d', { willReadFrequently: true }) as CanvasRenderingContext2D;
				g.fillStyle = '#000';
				g.fillRect(0, 0, c.width, c.height);
				g.scale(scale, scale);
				g.fillStyle = '#fff';
				fn(g);
				return g.getImageData(0, 0, c.width, c.height).data;
			};
			const whole = draw((g) => g.fill(new Path2D(logo)));
			const parts = draw((g) => {
				g.fill(new Path2D(couch));
				const halo = new Path2D(silhouette);
				g.fillStyle = g.strokeStyle = '#000';
				g.lineWidth = gap * 2;
				g.lineJoin = 'round';
				g.fill(halo);
				g.stroke(halo);
				g.fillStyle = '#fff';
				g.fill(new Path2D(popcorn));
			});
			let off = 0;
			for (let i = 0; i < whole.length; i += 4) if (Math.abs(whole[i] - parts[i]) > 128) off++;
			return off;
		},
		{ couch: COUCH, popcorn: POPCORN, silhouette: SILHOUETTE, logo: LOGO, gap: LOGO_GAP }
	);
	// edges antialias alike, so no pixel is more than half a step apart
	expect(diff).toBe(0);
});

test("the lockup's letters are the bundled face's own outlines", async ({ page }) => {
	await load(page, FIXTURES['ident-short']);
	const out = await page.evaluate(
		({ parts }) => {
			const size = 400;
			const draw = (fn: (g: CanvasRenderingContext2D) => void) => {
				const c = document.createElement('canvas');
				c.width = 1800;
				c.height = 520;
				const g = c.getContext('2d', { willReadFrequently: true }) as CanvasRenderingContext2D;
				g.fillStyle = '#000';
				g.fillRect(0, 0, c.width, c.height);
				g.fillStyle = '#fff';
				fn(g);
				return g.getImageData(0, 0, c.width, c.height).data;
			};
			const text = draw((g) => {
				g.font = `800 ${size}px "Couch TV Sans"`;
				g.letterSpacing = `${-0.035 * size}px`;
				let x = 40;
				for (const p of parts) {
					g.fillText(p.text, x, 420);
					x += g.measureText(p.text).width;
				}
			});
			const paths = draw((g) => {
				for (const p of parts) {
					const k = size / 1000;
					g.setTransform(k, 0, 0, k, 40 + p.x * k, 420);
					g.fill(new Path2D(p.d));
				}
			});
			let ink = 0;
			let off = 0;
			for (let i = 0; i < text.length; i += 4) {
				if (text[i] > 128) ink++;
				if (Math.abs(text[i] - paths[i]) > 128) off++;
			}
			return { ink, off };
		},
		{ parts: PARTS.map((p) => ({ text: p.text, x: p.x, d: p.d })) }
	);
	expect(out.ink).toBeGreaterThan(50000);
	expect(out.off / out.ink).toBeLessThan(0.005);
});
