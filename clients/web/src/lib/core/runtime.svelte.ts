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
		try {
			this.#bridge = await this.#spawn(JSON.stringify(this.#config));
			await this.#dispatch({ type: 'appStarted' });
		} finally {
			this.#restarting = false;
		}
		if (!this.#bridge) return;
		this.#render([{ type: 'app' }, { type: 'session' }]);
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
			}
		}
	}
}

/**
 * `next`, keeping every part of `prev` that did not change, so a `$derived` or `$effect` that
 * reads only those parts does not run again.
 */
function reuse<T>(prev: T, next: T): T {
	if (JSON.stringify(prev) === JSON.stringify(next)) return prev;
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
