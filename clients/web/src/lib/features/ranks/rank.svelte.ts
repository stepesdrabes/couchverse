import { core } from '$lib/core';
import type { AchievementCard, RankBadge, RankView } from '$lib/generated/core';
import { RANK } from './api';

/**
 * The viewer's rank badge and the unlocks waiting to be celebrated, as the core holds them.
 * Two places show a celebration: the player's overlay while one is mounted (it lives inside
 * the fullscreen subtree), and toasts everywhere else.
 */
class Rank {
	/** How many in-player overlays are mounted; the toasts stand down while any is. */
	overlays = $state(0);

	#taking: Promise<void> | null = null;

	/** Keeps the view current while the calling component is mounted; call during its setup. */
	watch() {
		$effect(() => core.watch(RANK));
	}

	get #view() {
		return core.view<RankView>(RANK);
	}

	/** Absent until the first check, or with rankings off. */
	get badge(): RankBadge | undefined {
		return this.#view?.rank;
	}

	/** Bumped on a genuine level-up, so the nav ring can pulse once. */
	get levelUps(): number {
		return this.#view?.levelUps ?? 0;
	}

	get celebration(): AchievementCard | undefined {
		return this.#view?.celebration;
	}

	/**
	 * Takes the unlock waiting to be celebrated: the caller shows it from now on, and the core
	 * moves on to the next. Undefined while another take is under way.
	 */
	async take(): Promise<AchievementCard | undefined> {
		const next = this.celebration;
		if (!next || this.#taking) return undefined;
		this.#taking = core.send({ type: 'celebrationDismissed' });
		try {
			await this.#taking;
		} finally {
			this.#taking = null;
		}
		return next;
	}
}

export const rank = new Rank();
