import * as api from './api';
import type { Achievement, RankProgress } from './types';

/** at most one check per this window, mirroring the server-side throttle */
const CHECK_INTERVAL_MS = 5 * 60 * 1000;

/**
 * The single source of the viewer's own rank plus the queue of achievements
 * waiting to be celebrated. One store, two consumers: the nav badge reads
 * `summary`, and whichever celebration surface is mounted drains `queue`.
 */
class RankStore {
	summary = $state<RankProgress | null>(null);
	/** unlocks waiting to be shown, oldest first */
	queue = $state<Achievement[]>([]);
	/** bumped only on a genuine level-up, so the nav ring can pulse once */
	levelUps = $state(0);
	/**
	 * How many in-player overlays are mounted. The app-wide watcher stands down
	 * while one is, so a celebration never fires behind a fullscreen player.
	 * Mirrors the couch store's player-mount counter.
	 */
	overlayMounts = $state(0);

	#lastCheck = 0;

	/** adopt a rank without treating the first value as a level-up */
	seed(next: RankProgress | null) {
		const first = this.summary === null;
		if (!first && next && next.tier.level > this.summary!.tier.level) this.levelUps++;
		this.summary = next;
	}

	/** ask the server whether anything new was earned; cheap and self-throttled */
	async check(force = false) {
		if (!force && Date.now() - this.#lastCheck < CHECK_INTERVAL_MS) return;
		this.#lastCheck = Date.now();
		try {
			const result = await api.checkAchievements();
			// a throttled call skipped the snapshot, so its rank is absent and the
			// one already on screen stays
			if (result.rank) this.seed(result.rank);
			if (result.unlocked.length > 0) this.queue = [...this.queue, ...result.unlocked];
		} catch {
			// progression must never interrupt watching
		}
	}

	/** take the next unlock to celebrate, or undefined when the queue is empty */
	shift(): Achievement | undefined {
		if (this.queue.length === 0) return undefined;
		const [next, ...rest] = this.queue;
		this.queue = rest;
		return next;
	}

	reset() {
		this.summary = null;
		this.queue = [];
		this.levelUps = 0;
		this.#lastCheck = 0;
	}
}

export const rank = new RankStore();
