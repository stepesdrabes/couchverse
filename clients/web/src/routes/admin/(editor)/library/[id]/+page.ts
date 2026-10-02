import * as libraryApi from '$lib/features/library/api';

// Non-blocking: the editor swaps in immediately behind a skeleton; a save's
// invalidateAll revalidates in place (StreamedView keeps the last value, no flash).
export function load({ params }) {
	return { id: params.id, fresh: libraryApi.getTitle(params.id) };
}
