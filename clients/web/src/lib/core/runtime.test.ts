import { readFileSync } from 'node:fs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
	AppPhase,
	AuthMode,
	BrowseSort,
	HttpFailureKind,
	LoadStatus,
	Platform,
	TitleKind
} from '$lib/generated/core';
import type {
	BrowseView,
	CoreConfig,
	EffectOutput,
	GenresView,
	HttpRequest,
	NoticesView,
	StoreRequest,
	Surface,
	TimerRequest
} from '$lib/generated/core';
import { palette } from '$lib/theme';
import { browserExecutor } from './executor';
import { CoreRuntime, surfaceKey, type Bridge, type Executor, type Spawn } from './runtime.svelte';
import { spawner } from './wasm';

const compiled = WebAssembly.compile(
	readFileSync(new URL('./pkg/couchverse_core_bg.wasm', import.meta.url))
);
const spawnReal = spawner(compiled);

const WEB: CoreConfig = {
	platform: Platform.Web,
	authMode: AuthMode.Cookie,
	deviceName: '',
	locale: 'en',
	origin: 'http://tv.home'
};

interface Held {
	request: HttpRequest;
	answer: (output: EffectOutput) => void;
}

interface Timer {
	request: TimerRequest;
	fire: () => void;
	cancelled: boolean;
}

/** A scripted shell: answers HTTP from a route table (or holds it), stores in a Map. */
class FakeShell implements Executor {
	routes = new Map<string, { status: number; body: unknown }>();
	requests: HttpRequest[] = [];
	held: Held[] = [];
	holding = false;
	stored = new Map<string, string>();
	timers: Timer[] = [];

	route(method: string, url: string, status: number, body: unknown) {
		this.routes.set(`${method} ${url}`, { status, body });
	}

	async http(request: HttpRequest): Promise<EffectOutput> {
		this.requests.push(request);
		if (this.holding) return new Promise((answer) => this.held.push({ request, answer }));
		return this.answer(request);
	}

	answer(request: HttpRequest): EffectOutput {
		const route = this.routes.get(`${request.method} ${request.url}`);
		if (!route) {
			return { type: 'httpFailed', content: { kind: HttpFailureKind.Other, message: 'no route' } };
		}
		return { type: 'http', content: { status: route.status, body: JSON.stringify(route.body) } };
	}

	/** Answers every held request from the route table. */
	release() {
		this.holding = false;
		for (const { request, answer } of this.held.splice(0)) answer(this.answer(request));
	}

	store({ key, op }: StoreRequest): EffectOutput {
		if (op.type === 'read') {
			const value = this.stored.get(key);
			return { type: 'stored', content: value === undefined ? {} : { value } };
		}
		if (op.type === 'write') this.stored.set(key, op.content);
		else this.stored.delete(key);
		return { type: 'storeDone' };
	}

	upload = browserExecutor.upload;
	secureStore = browserExecutor.secureStore;

	timer(request: TimerRequest, fire: () => void) {
		const timer: Timer = { request, fire, cancelled: false };
		this.timers.push(timer);
		return () => (timer.cancelled = true);
	}

	sent(method: string, url: string) {
		return this.requests.filter((r) => r.method === method && r.url === url);
	}
}

function user(username: string, admin = false) {
	return {
		id: 1,
		username,
		displayName: username.toUpperCase(),
		role: admin ? 'admin' : 'member',
		disabled: false,
		avatarId: 'av-1',
		bannerId: null,
		bio: '# Hi',
		createdAt: '2026-01-01T00:00:00Z'
	};
}

function signedIn(shell: FakeShell, language = 'en') {
	shell.route('GET', '/api/v1/auth/me', 200, user('admin', true));
	shell.route('GET', '/api/v1/features', 200, {
		couchEnabled: true,
		rankingsEnabled: false,
		downloadsEnabled: true
	});
	shell.route('GET', '/api/v1/me/preferences', 200, { language });
	shell.route('GET', '/api/v1/server', 200, {
		id: 'srv',
		name: 'Home',
		version: '1.0.0',
		apiLevel: 1,
		accent: '#3a6ea5'
	});
	shell.route('PUT', '/api/v1/me/preferences', 200, {});
	shell.route('POST', '/api/v1/auth/logout', 204, null);
}

function runtime(shell: FakeShell, spawn: Spawn = spawnReal, config = WEB) {
	let now = 1000;
	return new CoreRuntime(config, spawn, shell, () => (now += 10));
}

describe('the core runtime', () => {
	it('loads the web session over the cookie before start settles', async () => {
		const shell = new FakeShell();
		signedIn(shell);
		const core = runtime(shell);
		expect(core.started).toBe(false);

		await core.start();

		expect(core.app.phase).toBe(AppPhase.Ready);
		expect(core.session.status).toBe(LoadStatus.Loaded);
		expect(core.session.user).toMatchObject({ username: 'admin', admin: true, bio: '# Hi' });
		expect(core.session.features).toEqual({ couch: true, rankings: false });
		// the web still derives scoped accents itself, so the two derivations must agree
		expect(core.session.accent).toEqual(palette('#3a6ea5'));
		// origin-relative, no bearer: the browser's cookie authenticates
		const me = shell.sent('GET', '/api/v1/auth/me')[0];
		expect(me.headers.some((h) => h.name === 'Authorization')).toBe(false);
	});

	it('signs a visitor without a session in, keeping the server accent', async () => {
		const shell = new FakeShell();
		signedIn(shell);
		shell.route('GET', '/api/v1/auth/me', 401, { error: { code: 'unauthorized', message: '' } });
		const core = runtime(shell);

		await core.start();

		expect(core.app.phase).toBe(AppPhase.SignIn);
		expect(core.session.user).toBeUndefined();
		expect(core.session.accent.accent).toBe('#3a6ea5');
	});

	it('settles a send once the effects it led to have finished', async () => {
		const shell = new FakeShell();
		signedIn(shell);
		const core = runtime(shell);
		await core.start();

		await core.send({ type: 'displayLanguageChanged', content: { code: 'cs' } });

		expect(core.session.language).toBe('cs');
		const [save] = shell.sent('PUT', '/api/v1/me/preferences');
		expect(JSON.parse(save.body ?? '')).toEqual({ language: 'cs' });
	});

	it('keeps the parts of a view that did not change', async () => {
		const shell = new FakeShell();
		signedIn(shell);
		const core = runtime(shell);
		await core.start();
		const before = core.session;

		await core.send({ type: 'sessionChanged' });

		// the status went stale and back, but the user, flags and accent are the same objects
		expect(shell.sent('GET', '/api/v1/auth/me')).toHaveLength(2);
		expect(core.session).toEqual(before);
		expect(core.session.user).toBe(before.user);
		expect(core.session.features).toBe(before.features);

		shell.route('GET', '/api/v1/features', 200, {
			couchEnabled: false,
			rankingsEnabled: false,
			downloadsEnabled: true
		});
		await core.send({ type: 'sessionChanged' });
		expect(core.session.features.couch).toBe(false);
		expect(core.session.user).toBe(before.user);
		expect(core.session.accent).toBe(before.accent);
	});

	it('signs out on the server and drops the user', async () => {
		const shell = new FakeShell();
		signedIn(shell);
		const core = runtime(shell);
		await core.start();

		await core.send({ type: 'signOutRequested', content: { accountId: 'web' } });

		expect(shell.sent('POST', '/api/v1/auth/logout')).toHaveLength(1);
		expect(core.app.phase).toBe(AppPhase.SignIn);
		expect(core.session.user).toBeUndefined();
	});

	it('renders markdown safely', async () => {
		const shell = new FakeShell();
		signedIn(shell);
		const core = runtime(shell);
		await core.start();

		const doc = core.markdown('Hi **there** <b>x</b> [x](javascript:alert(1))');
		const [paragraph] = doc.blocks;
		expect(paragraph.type).toBe('paragraph');
		expect(JSON.stringify(doc)).not.toContain('javascript:');
		expect(JSON.stringify(doc)).toContain('<b>x</b>');
	});

	it('runs timers and stores for a native config', async () => {
		const shell = new FakeShell();
		const server = 'https://media.example.com';
		shell.stored.set(
			'servers',
			JSON.stringify([
				{
					id: 'srv',
					url: server,
					name: 'Home',
					version: '1.0.0',
					apiLevel: 1,
					accent: '#3a6ea5',
					insecure: false
				}
			])
		);
		shell.route('POST', `${server}/api/v1/auth/pairings`, 201, {
			deviceCode: 'dc',
			userCode: 'WDJB-MJHT',
			verifyPath: '/pair?code=WDJB-MJHT',
			expiresIn: 600,
			interval: 5
		});
		shell.route('POST', `${server}/api/v1/auth/pairings/poll`, 200, { status: 'pending' });
		const core = runtime(shell, spawnReal, {
			platform: Platform.Tvos,
			authMode: AuthMode.Bearer,
			deviceName: 'Living Room',
			locale: 'en'
		});
		await core.start();
		expect(core.app.phase).toBe(AppPhase.SignIn);

		await core.send({ type: 'pairingStarted', content: { serverId: 'srv' } });
		const poll = shell.timers.find((t) => t.request.repeat);
		expect(poll?.request.afterMs).toBe(5000);

		poll?.fire();
		await vi.waitFor(() =>
			expect(shell.sent('POST', `${server}/api/v1/auth/pairings/poll`)).toHaveLength(1)
		);

		await core.send({ type: 'pairingCancelled' });
		expect(shell.timers.every((t) => t.cancelled)).toBe(true);
	});
});

describe('screens', () => {
	const GENRES: Surface = { type: 'genres' };
	const drama = (label: string) => [{ id: 1, name: 'Drama', label }];
	const card = (id: string) => ({
		titleId: id,
		slug: `${id}-slug`,
		name: `Name ${id}`,
		kind: 'movie',
		year: 2024,
		posterId: `p-${id}`,
		posterVer: 3,
		posterAccent: null,
		backdropId: null,
		backdropVer: null,
		backdropAccent: null
	});

	async function signedInCore(shell: FakeShell, clock = { now: 1000 }) {
		signedIn(shell);
		const core = new CoreRuntime(WEB, spawnReal, shell, () => clock.now);
		await core.start();
		return core;
	}

	it('keeps an open screen current and the parts that did not change', async () => {
		const shell = new FakeShell();
		shell.route('GET', '/api/v1/genres?lang=en', 200, drama('Drama'));
		const core = await signedInCore(shell);
		// before it opens, a page reads what the core holds
		expect(core.view<GenresView>(GENRES)?.status).toBe(LoadStatus.Idle);

		const close = core.open(GENRES);
		await vi.waitFor(() => expect(core.view<GenresView>(GENRES)?.status).toBe(LoadStatus.Loaded));
		const loaded = core.view<GenresView>(GENRES);
		expect(loaded?.genres).toEqual([{ name: 'Drama', label: 'Drama' }]);

		// the status goes stale and back, but the same answer keeps the same genres
		core.revalidate(GENRES);
		await vi.waitFor(() => expect(shell.sent('GET', '/api/v1/genres?lang=en')).toHaveLength(2));
		await core.send({ type: 'noticeDismissed', content: { id: 0 } });
		expect(core.view<GenresView>(GENRES)).toEqual(loaded);
		expect(core.view<GenresView>(GENRES)?.genres).toBe(loaded?.genres);

		// a changed label replaces only what changed
		shell.route('GET', '/api/v1/genres?lang=en', 200, drama('Drama (new)'));
		core.revalidate(GENRES);
		await vi.waitFor(() =>
			expect(core.view<GenresView>(GENRES)?.genres[0].label).toBe('Drama (new)')
		);
		close();
		close();
		// closing twice is closing once; the view is still readable, no longer kept
		expect(core.view<GenresView>(GENRES)?.genres[0].label).toBe('Drama (new)');
	});

	it('matches the surfaces the core renders whatever their field order', async () => {
		expect(
			surfaceKey({ type: 'browse', content: { sort: BrowseSort.Name, kind: TitleKind.Movie } })
		).toBe(
			surfaceKey({ type: 'browse', content: { kind: TitleKind.Movie, sort: BrowseSort.Name } })
		);

		const shell = new FakeShell();
		shell.route('GET', '/api/v1/titles?lang=en&kind=movie&sort=name&page=1', 200, {
			items: [card('a')],
			total: 1
		});
		const core = await signedInCore(shell);
		const listing: Surface = {
			type: 'browse',
			content: { sort: BrowseSort.Name, kind: TitleKind.Movie }
		};
		core.open(listing);
		await vi.waitFor(() =>
			expect(core.view<BrowseView>(listing)?.cards.map((c) => c.titleId)).toEqual(['a'])
		);
		expect(core.view<BrowseView>(listing)?.cards[0].poster?.url).toBe(
			'/api/v1/artwork/p-a?size=w342&v=3'
		);
	});

	it('revalidates a screen once for its load and its opening', async () => {
		const shell = new FakeShell();
		shell.route('GET', '/api/v1/genres?lang=en', 200, drama('Drama'));
		const clock = { now: 1000 };
		const core = await signedInCore(shell, clock);
		const asked = () => shell.sent('GET', '/api/v1/genres?lang=en').length;

		core.revalidate(GENRES);
		const close = core.open(GENRES, { revalidate: true });
		await vi.waitFor(() => expect(core.view<GenresView>(GENRES)?.status).toBe(LoadStatus.Loaded));
		expect(asked()).toBe(1);
		close();

		// coming back later catches up, though the core still holds it as fresh
		clock.now += 6_000;
		core.open(GENRES, { revalidate: true });
		await vi.waitFor(() => expect(asked()).toBe(2));
		// a plain open trusts the core's freshness
		core.open(GENRES);
		await core.send({ type: 'noticeDismissed', content: { id: 0 } });
		expect(asked()).toBe(2);
	});

	it('shows a My List change at once and turns a refusal into a notice', async () => {
		const shell = new FakeShell();
		const core = await signedInCore(shell);
		const notices: Surface = { type: 'notices' };
		const stop = core.watch(notices);

		await core.send({ type: 'watchlistChanged', content: { titleId: 't1', listed: true } });
		const [notice] = core.view<NoticesView>(notices)?.notices ?? [];
		expect(notice.code).toBe('watchlist_failed');

		await core.send({ type: 'noticeDismissed', content: { id: notice.id } });
		expect(core.view<NoticesView>(notices)?.notices).toEqual([]);
		stop();
	});
});

describe('a trapped core', () => {
	/** The real core behind a switch that makes its next call abort like a Rust panic. */
	function trapping() {
		const spawned: Bridge[] = [];
		const trap = { next: false, source: '' };
		const spawn: Spawn = async (config) => {
			const real = await spawnReal(config);
			const abort = () => {
				trap.next = false;
				throw new WebAssembly.RuntimeError('unreachable');
			};
			const bridge: Bridge = {
				send: (m) => (trap.next ? abort() : real.send(m)),
				resolve: (r) => real.resolve(r),
				view: (s) => (trap.source && s.includes(trap.source) ? abort() : real.view(s))
			};
			spawned.push(bridge);
			return bridge;
		};
		return { spawn, spawned, trap };
	}

	beforeEach(() => {
		vi.spyOn(console, 'error').mockImplementation(() => {});
	});

	afterEach(() => {
		vi.restoreAllMocks();
	});

	it('is replaced and the session loads again, with the views kept meanwhile', async () => {
		const shell = new FakeShell();
		signedIn(shell);
		const { spawn, spawned, trap } = trapping();
		const core = runtime(shell, spawn);
		await core.start();
		const before = core.session;

		trap.next = true;
		shell.holding = true;
		const sent = core.send({ type: 'sessionChanged' });
		await vi.waitFor(() => expect(shell.held).toHaveLength(4));

		expect(spawned).toHaveLength(2);
		expect(core.session).toBe(before);
		expect(core.app.phase).toBe(AppPhase.Ready);

		shell.release();
		await sent;
		expect(core.session.user?.username).toBe('admin');
		expect(core.app.phase).toBe(AppPhase.Ready);
	});

	it('opens the screens the pages still show on the new core', async () => {
		const shell = new FakeShell();
		signedIn(shell);
		shell.route('GET', '/api/v1/genres?lang=en', 200, [{ id: 1, name: 'Drama', label: 'Drama' }]);
		const { spawn, spawned, trap } = trapping();
		const core = runtime(shell, spawn);
		await core.start();
		const genres: Surface = { type: 'genres' };
		core.open(genres);
		await vi.waitFor(() => expect(core.view<GenresView>(genres)?.status).toBe(LoadStatus.Loaded));

		trap.next = true;
		await core.send({ type: 'sessionChanged' });

		expect(spawned).toHaveLength(2);
		// the new core loaded it for the open page, which kept showing it meanwhile
		expect(shell.sent('GET', '/api/v1/genres?lang=en')).toHaveLength(2);
		expect(core.view<GenresView>(genres)?.genres[0].label).toBe('Drama');
	});

	it('shows markdown that trapped it as plain text from then on', async () => {
		const shell = new FakeShell();
		signedIn(shell);
		const { spawn, spawned, trap } = trapping();
		const core = runtime(shell, spawn);
		await core.start();

		trap.source = 'boom';
		expect(core.markdown('**boom**')).toEqual({
			blocks: [{ type: 'paragraph', content: [{ type: 'text', content: '**boom**' }] }]
		});
		await core.send({ type: 'sessionChanged' });

		expect(spawned).toHaveLength(2);
		expect(core.markdown('**boom**').blocks[0]).toMatchObject({ type: 'paragraph' });
		expect(core.markdown('**fine**').blocks[0]).toEqual({
			type: 'paragraph',
			content: [{ type: 'strong', content: [{ type: 'text', content: 'fine' }] }]
		});
	});
});

describe('the browser executor', () => {
	it('namespaces stored keys and never keeps a secret', () => {
		const items = new Map<string, string>();
		vi.stubGlobal('localStorage', {
			getItem: (k: string) => items.get(k) ?? null,
			setItem: (k: string, v: string) => items.set(k, v),
			removeItem: (k: string) => items.delete(k)
		});

		const write = { key: 'servers', op: { type: 'write' as const, content: '[]' } };
		expect(browserExecutor.store(write)).toEqual({ type: 'storeDone' });
		expect([...items.keys()]).toEqual(['cv.core.servers']);
		expect(browserExecutor.store({ key: 'servers', op: { type: 'read' } })).toEqual({
			type: 'stored',
			content: { value: '[]' }
		});

		const secret = { key: 'token.a', op: { type: 'write' as const, content: 'tok' } };
		expect(browserExecutor.secureStore(secret).type).toBe('storeFailed');
		expect(browserExecutor.secureStore({ key: 'token.a', op: { type: 'read' } })).toEqual({
			type: 'stored',
			content: {}
		});
		expect(items.size).toBe(1);
		vi.unstubAllGlobals();
	});
});
