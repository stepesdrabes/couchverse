import { error } from '@sveltejs/kit';
import { ApiError } from '$lib/api/client';
import { getCouchInfo } from '$lib/features/couch/api';

// Public route. We only PREVIEW the session here (no join, no participant); the
// actual join happens on the "Start watching" click so the video can autoplay
// off that user gesture. skipAuthRedirect keeps anonymous viewers off /login.
export async function load({ params }) {
	try {
		const info = await getCouchInfo(params.token);
		return { token: params.token, info };
	} catch (err) {
		if (err instanceof ApiError) {
			error(err.status >= 400 && err.status < 600 ? err.status : 500, err.message);
		}
		throw err;
	}
}
