import { core } from '$lib/core';

/** Admin-toggleable feature flags, as the core's session reads them. */
export const features = {
	get couchEnabled() {
		return core.session.features.couch;
	},
	get rankingsEnabled() {
		return core.session.features.rankings;
	}
};
