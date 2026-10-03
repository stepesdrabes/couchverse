import { SvelteMap } from 'svelte/reactivity';
import { AppPhase } from '$lib/generated/core';
import type {
	AppView,
	CoreConfig,
	Event as CoreEvent,
	EffectOutput,
	EffectRequest,
	HttpRequest,
	MarkdownDoc,
	SessionView,
	StoreRequest,
	Surface,
	TimerRequest,
	UploadRequest
} from '$lib/generated/core';

/** One core instance (the wasm `CoreBridge`): JSON strings in, JSON strings out. */
export interface Bridge {
	send(message: string): string;
	resolve(resolution: string): string;
	view(surface: string): string;
}

/**
 * How long a revalidation covers opening the same screen with `revalidate`. Long enough for a
 * hover preload a moment before the click, short enough that coming back catches up.
 */
const REVISIT_MS = 5_000;
const MAX_PEEKED = 8;

/** Creates an independent core instance from a `CoreConfig` JSON. */
export type Spawn = (config: string) => Promise<Bridge>;

/** Performs the effects the core asks for: the browser's in the app, a scripted one in tests. */
export interface Executor {
	http(request: HttpRequest): Promise<EffectOutput>;
	/** Posts the file behind `request.file` as a multipart form; answers like `http`. */
	upload(request: UploadRequest): Promise<EffectOutput>;
	store(request: StoreRequest): EffectOutput;
	secureStore(request: StoreRequest): EffectOutput;
	/** Calls `fire` after `afterMs` (every `afterMs` when repeating); returns the cancel. */
	timer(request: TimerRequest, fire: () => void): () => void;
}

/**
 * The shell side of the core (plan 7.1): owns the bridge, stamps every message with the
 * monotonic clock, performs the effects the core asks for and keeps the view models the web
 * renders in `$state.raw`, re-reading only the surfaces a `render` effect names.
 */
export class CoreRuntime {
	#app = $state.raw<AppView>({ phase: AppPhase.Starting });
	#session = $state.raw<SessionView>();
	// bumped when a new core replaces a trapped one, so markdown derived from the old one re-runs
	#generation = $state(0);
	// the views of the surfaces something watches, by `surfaceKey`
	#views = new SvelteMap<string, unknown>();

	#config: CoreConfig;
	#spawn: Spawn;
	#executor: Executor;
	#now: () => number;

	// bookkeeping below, never rendered
	#bridge: Bridge | undefined;
	#ready: Promise<void> | undefined;
	// eslint-disable-next-line svelte/prefer-svelte-reactivity
	#timers = new Map<number, () => void>();
	#restarting = false;
	// markdown sources that trapped the core are shown as plain text from then on
	// eslint-disable-next-line svelte/prefer-svelte-reactivity
	#poisoned = new Set<string>();
	// how many watchers each watched surface has, and how many of them opened it as a screen
	// eslint-disable-next-line svelte/prefer-svelte-reactivity
	#watched = new Map<string, { surface: Surface; watchers: number; screens: number }>();
	// a surface read before anything watched it, so watching it keeps the same objects
	// eslint-disable-next-line svelte/prefer-svelte-reactivity
	#peeked = new Map<string, unknown>();
	// when each surface was last revalidated
	// eslint-disable-next-line svelte/prefer-svelte-reactivity
	#revalidated = new Map<string, number>();

	constructor(
		config: CoreConfig,
		spawn: Spawn,
		executor: Executor,
		now = () => Math.floor(performance.now())
	) {
		this.#config = config;
		this.#spawn = spawn;
		this.#executor = executor;
		this.#now = now;
	}

	/** Where the app is: `starting` until the session is known, then `ready` or `signIn`. */
	get app(): AppView {
		return this.#app;
	}

	/** The signed-in session: user, feature flags, display language and accent. */
	get session(): SessionView {
		if (!this.#session) throw new Error('the core has not started');
		return this.#session;
	}

	get started(): boolean {
		return this.#session !== undefined;
	}

	/** Creates the core and loads the session; settles once that first load has. */
	async start(): Promise<void> {
		this.#ready ??= this.#boot();
		await this.#settled();
	}

	/** Sends `event`; settles once every effect it led to has finished (timers aside). */
	async send(event: CoreEvent): Promise<void> {
		await this.start();
		await this.#dispatch(event);
		await this.#settled();
	}

	/** Waits out the start and any restart after a trap. */
	async #settled() {
		let ready;
		do {
			ready = this.#ready;
			await ready;
		} while (ready !== this.#ready);
	}

	/** User-authored markdown as the core's safe document tree. */
	markdown(source: string): MarkdownDoc {
		void this.#generation;
		const bridge = this.#bridge;
		if (!bridge || this.#poisoned.has(source)) return plainText(source);
		try {
			return JSON.parse(bridge.view(JSON.stringify({ type: 'markdown', content: source })));
		} catch (err) {
			if (!(err instanceof WebAssembly.RuntimeError)) throw err;
			this.#poisoned.add(source);
			this.#replace(bridge, err);
			return plainText(source);
		}
	}

	/**
	 * A surface's view model, kept current while something watches it (`open`, `watch`).
	 * Before that it is read as the core has it, so a page paints what is cached on its first
	 * render. Undefined only while no core is running.
	 */
	view<T>(surface: Surface): T | undefined {
		void this.#generation;
		const key = surfaceKey(surface);
		if (this.#views.has(key)) return this.#views.get(key) as T;
		const view = this.#read<T>(surface);
		if (view === undefined) return view;
		this.#peeked.delete(key);
		this.#peeked.set(key, view);
		// only a page about to open needs its read; a closed page's last one can go
		for (const oldest of this.#peeked.keys()) {
			if (this.#peeked.size <= MAX_PEEKED) break;
			this.#peeked.delete(oldest);
		}
		return view;
	}

	/**
	 * Opens a screen: the core loads `surface` and keeps it fresh (`ScreenOpened`), and its view
	 * stays current until the returned close (`ScreenClosed`). Pages open what they show from
	 * an `$effect`, so leaving the page closes it.
	 */
	open(surface: Surface, { revalidate = false } = {}): () => void {
		const stop = this.#watch(surface, true);
		// with `revalidate`, a visit catches up with changes made elsewhere even when the core
		// holds the surface as fresh; a page whose load just asked for that (or a hover that
		// preloaded it) does not ask twice
		const asked = this.#revalidated.get(surfaceKey(surface));
		if (revalidate && (asked === undefined || this.#now() - asked > REVISIT_MS)) {
			this.revalidate(surface);
		}
		void this.send({ type: 'screenOpened', content: surface });
		return () => {
			if (stop()) void this.send({ type: 'screenClosed', content: surface });
		};
	}

	/** Has the core load `surface` unless it holds it as fresh, without opening it (a preload). */
	prefetch(surface: Surface) {
		void this.send({ type: 'screenOpened', content: surface });
		void this.send({ type: 'screenClosed', content: surface });
	}

	/** Keeps the view of a surface that is not a screen (the notices) current until the stop. */
	watch(surface: Surface): () => void {
		const stop = this.#watch(surface, false);
		return () => void stop();
	}

	/**
	 * Has the core load `surface` again even when it is fresh (`RefreshRequested`), as a page's
	 * load revalidates what it is about to show; whatever is cached stays up meanwhile.
	 */
	revalidate(surface: Surface) {
		const now = this.#now();
		for (const [key, at] of this.#revalidated) {
			if (now - at > REVISIT_MS) this.#revalidated.delete(key);
		}
		this.#revalidated.set(surfaceKey(surface), now);
		void this.send({ type: 'refreshRequested', content: surface });
	}

	/** Starts watching `surface`; the returned stop says whether this call was still watching. */
	#watch(surface: Surface, screen: boolean): () => boolean {
		const key = surfaceKey(surface);
		let entry = this.#watched.get(key);
		if (!entry) {
			entry = { surface, watchers: 0, screens: 0 };
			this.#watched.set(key, entry);
			const view = this.#read(surface);
			if (view !== undefined) this.#views.set(key, reuse(this.#peeked.get(key), view));
		}
		this.#peeked.delete(key);
		entry.watchers++;
		if (screen) entry.screens++;
		const watching = entry;
		let stopped = false;
		return () => {
			if (stopped) return false;
			stopped = true;
			watching.watchers--;
			if (screen) watching.screens--;
			if (watching.watchers === 0) {
				this.#watched.delete(key);
				this.#views.delete(key);
			}
			return true;
		};
	}

	#read<T>(surface: Surface): T | undefined {
		const bridge = this.#bridge;
		if (!bridge) return undefined;
		try {
			return JSON.parse(bridge.view(JSON.stringify(surface)));
		} catch (err) {
			if (!(err instanceof WebAssembly.RuntimeError)) throw err;
			this.#replace(bridge, err);
			return undefined;
		}
	}

	async #boot() {
		this.#bridge = await this.#spawn(JSON.stringify(this.#config));
		this.#render([{ type: 'app' }, { type: 'session' }]);
		await this.#dispatch({ type: 'appStarted' });
	}

	/**
	 * A trap (a Rust panic aborts in wasm) leaves the instance unusable, so a new core takes
	 * over and loads the session again. The views on screen stay until it has; the effects
	 * still in flight belong to the old core and are dropped.
	 */
	#replace(trapped: Bridge, err: unknown) {
		if (this.#bridge !== trapped) return;
		this.#bridge = undefined;
		for (const cancel of this.#timers.values()) cancel();
		this.#timers.clear();
		if (this.#restarting) {
			console.error('the core trapped again while restarting; giving up', err);
			return;
		}
		console.error('the core trapped; starting a new one', err);
		this.#ready = this.#restart();
	}

	async #restart() {
		this.#restarting = true;
		const watched = [...this.#watched.values()];
		try {
			this.#bridge = await this.#spawn(JSON.stringify(this.#config));
			await this.#dispatch({ type: 'appStarted' });
			// the new core has none of the screens open that the pages still show
			const screens = watched.flatMap((w) => Array<Surface>(w.screens).fill(w.surface));
			await Promise.all(screens.map((s) => this.#dispatch({ type: 'screenOpened', content: s })));
		} finally {
			this.#restarting = false;
		}
		if (!this.#bridge) return;
		this.#render([{ type: 'app' }, { type: 'session' }, ...watched.map((w) => w.surface)]);
		this.#generation++;
	}

	async #dispatch(event: CoreEvent) {
		const bridge = this.#bridge;
		if (!bridge) return;
		const message = JSON.stringify({ nowMs: this.#now(), event });
		await this.#call(bridge, () => bridge.send(message));
	}

	async #resolve(bridge: Bridge, id: number, output: EffectOutput) {
		// an answer for a core that has since been replaced
		if (bridge !== this.#bridge) return;
		const resolution = JSON.stringify({ nowMs: this.#now(), id, output });
		await this.#call(bridge, () => bridge.resolve(resolution));
	}

	/** Runs one bridge call and performs the effects it returns. */
	async #call(bridge: Bridge, call: () => string) {
		let effects: EffectRequest[];
		try {
			effects = JSON.parse(call());
		} catch (err) {
			if (!(err instanceof WebAssembly.RuntimeError)) throw err;
			this.#replace(bridge, err);
			return;
		}
		await Promise.all(effects.map((request) => this.#perform(bridge, request)));
	}

	async #perform(bridge: Bridge, { id, effect }: EffectRequest) {
		switch (effect.type) {
			case 'http':
				return this.#resolve(bridge, id, await this.#executor.http(effect.content));
			case 'upload':
				return this.#resolve(bridge, id, await this.#executor.upload(effect.content));
			case 'store':
				return this.#resolve(bridge, id, this.#executor.store(effect.content));
			case 'secureStore':
				return this.#resolve(bridge, id, this.#executor.secureStore(effect.content));
			case 'timer': {
				const repeat = effect.content.repeat ?? false;
				const cancel = this.#executor.timer(effect.content, () => {
					if (!repeat) this.#timers.delete(id);
					void this.#resolve(bridge, id, { type: 'timerFired' });
				});
				this.#timers.set(id, cancel);
				return;
			}
			case 'cancelTimer':
				this.#timers.get(effect.content.id)?.();
				this.#timers.delete(effect.content.id);
				return;
			case 'render':
				// a restarting core reloads quietly; its views are read once it has settled
				if (!this.#restarting) this.#render(effect.content.surfaces);
				return;
			case 'player':
			case 'socket':
				// nothing sends PlayRequested or joins a couch through the core until the web's
				// player and couch adopt it
				return;
			case 'download':
				// downloads are for the native apps; the web never asks for one
				return;
			default: {
				// a new effect needs an executor here before the web takes that core version
				const unhandled: never = effect;
				throw new Error(`unhandled core effect ${JSON.stringify(unhandled)}`);
			}
		}
	}

	#render(surfaces: Surface[]) {
		const bridge = this.#bridge;
		if (!bridge) return;
		for (const surface of surfaces) {
			if (surface.type === 'app') {
				this.#app = reuse(this.#app, JSON.parse(bridge.view(JSON.stringify(surface))));
			} else if (surface.type === 'session') {
				this.#session = reuse(this.#session, JSON.parse(bridge.view(JSON.stringify(surface))));
			} else {
				const key = surfaceKey(surface);
				if (!this.#watched.has(key)) continue;
				const view = this.#read(surface);
				if (view !== undefined) this.#views.set(key, reuse(this.#views.get(key), view));
			}
		}
	}
}

/**
 * A surface's identity. The core names surfaces in renders with its own field order, so object
 * keys are sorted (`undefined` fields drop out, as they do on the wire).
 */
export function surfaceKey(surface: Surface): string {
	return JSON.stringify(surface, (_, value) =>
		isRecord(value)
			? Object.fromEntries(Object.entries(value).sort(([a], [b]) => (a < b ? -1 : 1)))
			: value
	);
}

/**
 * `next`, keeping every part of `prev` that did not change (array items by position), so a
 * `$derived` or `$effect` that reads only those parts does not run again.
 */
function reuse<T>(prev: T, next: T): T {
	if (JSON.stringify(prev) === JSON.stringify(next)) return prev;
	if (Array.isArray(prev) && Array.isArray(next)) {
		return next.map((item, i) => reuse(prev[i], item)) as T;
	}
	if (!isRecord(prev) || !isRecord(next)) return next;
	const merged: Record<string, unknown> = {};
	for (const [key, value] of Object.entries(next)) merged[key] = reuse(prev[key], value);
	return merged as T;
}

function isRecord(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function plainText(source: string): MarkdownDoc {
	return {
		blocks: source.trim()
			? [{ type: 'paragraph', content: [{ type: 'text', content: source }] }]
			: []
	};
}
