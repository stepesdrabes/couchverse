import { SvelteMap } from 'svelte/reactivity';

/**
 * Reactive stale-while-revalidate cache. `get` is a reactive read, so a page can
 * derive from the cache and re-render silently once a background `revalidate`
 * lands - a revisit paints instantly from memory instead of waiting on the network,
 * which is the whole point on slow hardware. A small LRU cap keeps a long browsing
 * session from growing unbounded.
 */
class SwrCache<T> {
	#map = new SvelteMap<string, T>();
	#order: string[] = [];
	#max: number;
	// the newest fetch per key: bookkeeping that nothing renders
	// eslint-disable-next-line svelte/prefer-svelte-reactivity
	#latest = new Map<string, Promise<T>>();

	constructor(max: number) {
		this.#max = max;
	}

	get(key: string): T | undefined {
		return this.#map.get(key);
	}

	/** store a value; fetches of the key already in flight carry an older answer and are dropped */
	set(key: string, value: T) {
		this.#latest.delete(key);
		if (this.#map.has(key)) {
			this.#order.splice(this.#order.indexOf(key), 1);
		} else if (this.#order.length >= this.#max) {
			const evicted = this.#order.shift();
			if (evicted !== undefined) this.#map.delete(evicted);
		}
		this.#order.push(key);
		this.#map.set(key, value);
	}

	/** fetch fresh, store it, return it - a page's `load` returns this promise */
	revalidate(key: string, fetcher: () => Promise<T>): Promise<T> {
		const fresh = fetcher().then((value) => {
			// Only the newest fetch stores its answer, and only if nothing was written since
			// it started: a hover preload and the click's own load overlap, and a page writes
			// its own change (a My List toggle) while a refetch is in flight. An older answer
			// landing last would undo it.
			if (this.#latest.get(key) === fresh) this.set(key, value);
			return value;
		});
		this.#latest.set(key, fresh);
		// The page handles a failure once it mounts, but a fast 404 can reject before then
		// and a page whose layout redirected (a signed-out deep link) never mounts at all.
		fresh.catch(() => {});
		return fresh;
	}

	clear() {
		this.#map.clear();
		this.#order = [];
		this.#latest.clear();
	}
}

const registry: SwrCache<unknown>[] = [];

export function createSwrCache<T>(max = 50): SwrCache<T> {
	const cache = new SwrCache<T>(max);
	registry.push(cache as SwrCache<unknown>);
	return cache;
}

/** drop every cache - called on logout / 401 so the next user starts clean */
export function resetAllCaches() {
	for (const cache of registry) cache.clear();
}

export type { SwrCache };
