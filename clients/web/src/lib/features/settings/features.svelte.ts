import { getFeatures } from './api';

/** Admin-toggleable feature flags, fetched once at app bootstrap. */
class Features {
	couchEnabled = $state(true);
	rankingsEnabled = $state(true);

	async init() {
		try {
			const flags = await getFeatures();
			this.couchEnabled = flags.couchEnabled;
			this.rankingsEnabled = flags.rankingsEnabled;
		} catch {
			// unauthenticated or offline: leave defaults, login flow re-inits
		}
	}
}

export const features = new Features();
