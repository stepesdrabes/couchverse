import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import type { DeviceProfile } from '$lib/generated/core';
import { measureProfile, type MediaProbe } from './device-profile';

const fixture = (name: string): DeviceProfile =>
	JSON.parse(
		readFileSync(
			new URL(`../../../../../contract/fixtures/device-profiles/${name}.json`, import.meta.url),
			'utf8'
		)
	);

/** A browser that answers yes to exactly these types. */
function probe(types: string[], { mse = true, hdrScreen = false } = {}): MediaProbe {
	const yes = new Set(types);
	return {
		native: (type) => yes.has(type),
		mse: (type) => mse && yes.has(type),
		hasMse: mse,
		hdrScreen
	};
}

// what desktop Chrome on an SDR screen says about the types the measurement asks
const CHROME = [
	'video/mp4',
	'video/webm',
	'video/mp4; codecs="avc1.640028"',
	'video/mp4; codecs="avc1.640033"',
	'video/mp4; codecs="avc1.640034"',
	'video/webm; codecs="vp09.00.10.08"',
	'video/mp4; codecs="av01.0.08M.08"',
	'audio/mp4; codecs="mp4a.40.2"',
	'audio/mpeg',
	'audio/webm; codecs="opus"',
	'audio/webm; codecs="vorbis"',
	'audio/mp4; codecs="flac"'
];

describe('the device profile', () => {
	it('matches the contract for desktop Chrome', () => {
		expect(measureProfile(probe(CHROME))).toEqual(fixture('chrome-desktop'));
	});

	it('offers HDR only with a 10-bit decoder and a screen that shows it', () => {
		const tenBit = [...CHROME, 'video/mp4; codecs="av01.0.08M.10"'];
		expect(measureProfile(probe(tenBit)).hdr).toEqual([]);
		expect(measureProfile(probe(CHROME, { hdrScreen: true })).hdr).toEqual([]);
		expect(measureProfile(probe(tenBit, { hdrScreen: true })).hdr).toEqual(['hdr10', 'hlg']);
	});

	it('plays HLS natively without MSE, as Safari on an iPhone does', () => {
		const safari = probe(['video/mp4', 'application/vnd.apple.mpegurl'], { mse: false });
		expect(measureProfile(safari).hls).toEqual(['fmp4', 'ts']);
		expect(measureProfile(probe(['video/mp4'], { mse: false })).hls).toEqual([]);
	});
});
