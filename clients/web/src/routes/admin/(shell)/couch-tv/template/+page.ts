import { adminGetCouchTv } from '$lib/features/couchtv/api';

// Non-blocking: the editor swaps in behind a skeleton (StreamedView), and a save adopts
// the answer in place.
export function load() {
	return { fresh: adminGetCouchTv() };
}
