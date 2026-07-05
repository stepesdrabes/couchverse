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

	constructor(max: number) {
		this.#max = max;
	}

	get(key: string): T | undefined {
		return this.#map.get(key);
	}

	set(key: string, value: T) {
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
	async revalidate(key: string, fetcher: () => Promise<T>): Promise<T> {
		const value = await fetcher();
		this.set(key, value);
		return value;
	}

	clear() {
		this.#map.clear();
		this.#order = [];
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
