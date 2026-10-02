import { browser } from '$app/environment';

// Titan OS and the older Philips Linux platforms it replaced; desktop browsers never
// send these tokens
const TV_AGENT = /TitanOS\/|WhaleTV\/|SmartTvA\//;
// `?tv=1` turns TV mode on in a desktop browser for development, `?tv=0` off again
const FORCE_KEY = 'cv.tv';

function detect(): boolean {
	if (!browser) return false;
	const flag = new URLSearchParams(location.search).get('tv');
	if (flag === '1') localStorage.setItem(FORCE_KEY, '1');
	else if (flag === '0') localStorage.removeItem(FORCE_KEY);
	return TV_AGENT.test(navigator.userAgent) || localStorage.getItem(FORCE_KEY) === '1';
}

/** Running on a TV: remote-control navigation and the 10-foot layout. Fixed per page load. */
export const isTV = detect();

// the TV styles in app.css key off this attribute
if (isTV) document.documentElement.dataset.tv = '';

/**
 * The remote's Back button. Titan OS reports it as "Backspace" with keyCode 8 on
 * Philips and 461 on JVC/Vestel.
 */
export function isBackKey(e: KeyboardEvent): boolean {
	return e.key === 'Backspace' || e.key === 'GoBack' || e.keyCode === 461;
}

export type MediaKey = 'play' | 'pause' | 'toggle' | 'stop' | 'forward' | 'rewind';

// Titan OS keyCodes, for remotes whose `key` is not a standard media key name
const MEDIA_KEY_CODES: Record<number, MediaKey> = {
	415: 'play',
	19: 'pause',
	179: 'toggle',
	413: 'stop',
	417: 'forward',
	412: 'rewind'
};

export function mediaKey(e: KeyboardEvent): MediaKey | null {
	switch (e.key) {
		case 'MediaPlay':
			return 'play';
		case 'MediaPause':
			return 'pause';
		case 'MediaPlayPause':
			return 'toggle';
		case 'MediaStop':
			return 'stop';
		case 'MediaFastForward':
		case 'MediaTrackNext':
			return 'forward';
		case 'MediaRewind':
		case 'MediaTrackPrevious':
			return 'rewind';
	}
	return MEDIA_KEY_CODES[e.keyCode] ?? null;
}

/** Leave the app back to the TV launcher (Titan OS' documented exit sequence). */
export function exitApp() {
	const api = (window as { SmartTvA_API?: { exit?: () => void } }).SmartTvA_API;
	if (api?.exit) api.exit();
	else window.close();
}
