import assert from 'node:assert/strict';
import fs from 'node:fs';
import test from 'node:test';
import vm from 'node:vm';
import ts from 'typescript';

const component = fs.readFileSync(
	new URL('../src/lib/features/playback/components/VideoPlayer.svelte', import.meta.url),
	'utf8'
);
const source = ts.createSourceFile(
	'player.ts',
	component.match(/<script[^>]*>([\s\S]*?)<\/script>/)[1],
	ts.ScriptTarget.Latest,
	true
);
const handlerNames = new Set(['applySelectedAudio', 'selectAudio', 'onLoadedMetadata']);
const handlers = source.statements.filter(
	(node) => ts.isFunctionDeclaration(node) && handlerNames.has(node.name?.text)
);
assert.equal(handlers.length, handlerNames.size, 'Player audio handlers must remain available');

// Exercise the production handlers without a browser decoder or live streams.
const handlerScript = ts.transpileModule(handlers.map((node) => node.getText(source)).join('\n'), {
	compilerOptions: { target: ts.ScriptTarget.ES2022 }
}).outputText;
const rootUrl = '/api/v1/stream/primary/hls/multiaudio/master.m3u8';

function player() {
	let playCount = 0;
	const tracks = [
		{ id: 'en', source: 'embedded', lang: 'en', hlsUrl: rootUrl, hlsAudioIndex: 0 },
		{ id: 'cs', source: 'embedded', lang: 'cs', hlsUrl: rootUrl, hlsAudioIndex: 1 },
		{ id: 'de', source: 'file', lang: 'de', streamUrl: '/api/v1/stream/german' },
		{
			id: 'alt-cs',
			source: 'embedded',
			lang: 'cs',
			hlsUrl: '/api/v1/stream/alternate/hls/multiaudio/master.m3u8',
			hlsAudioIndex: 1
		}
	];
	const hls = () => ({
		audioTracks: [{ lang: 'eng' }, { lang: 'ces' }],
		audioTrack: 0,
		destroy() {}
	});
	const context = vm.createContext({
		audioTracks: tracks,
		activeAudioId: 'en',
		currentHlsUrl: rootUrl,
		switchHlsUrl: rootUrl,
		video: {
			currentTime: 87,
			paused: false,
			play() {
				playCount++;
				return Promise.resolve();
			}
		},
		hls: hls(),
		pendingResume: null,
		videoSrc: undefined,
		quality: 'auto',
		info: { resumePosition: 0 },
		didRestoreSub: false,
		restorePreferredSubtitle() {},
		applySubtitles() {},
		localStorage: { setItem() {}, removeItem() {} }
	});
	context.attachHls = (url) => {
		context.currentHlsUrl = url;
		context.hls = hls();
		context.applySelectedAudio();
	};
	vm.runInContext(handlerScript, context);
	return { context, tracks, playCount: () => playCount };
}

test('embedded English/Czech switching keeps the source and playback position', () => {
	const { context, tracks } = player();
	const initialHls = context.hls;
	context.selectAudio(tracks[1]);
	assert.equal(context.hls, initialHls);
	assert.equal(context.hls.audioTrack, 1);
	assert.equal(context.video.currentTime, 87);
	assert.equal(context.pendingResume, null);
});

test('switching to a direct sibling and back restores the primary Czech HLS and position', () => {
	const { context, tracks, playCount } = player();
	context.selectAudio(tracks[2]);
	assert.equal(context.videoSrc, '/api/v1/stream/german');
	assert.equal(context.pendingResume.at, 87);
	context.video.currentTime = 0;
	context.onLoadedMetadata();
	assert.equal(context.video.currentTime, 87);
	context.selectAudio(tracks[1]);
	assert.equal(context.currentHlsUrl, rootUrl);
	assert.equal(context.hls.audioTrack, 1);
	assert.equal(context.pendingResume.at, 87);
	context.video.currentTime = 0;
	context.onLoadedMetadata();
	assert.equal(context.video.currentTime, 87);
	assert.equal(playCount(), 2);
});

test('a combined sibling selects its own HLS track and restores paused playback position', () => {
	const { context, tracks, playCount } = player();
	context.video.paused = true;
	context.selectAudio(tracks[3]);
	assert.equal(context.currentHlsUrl, tracks[3].hlsUrl);
	assert.equal(context.hls.audioTrack, 1);
	assert.equal(context.pendingResume.at, 87);
	context.video.currentTime = 0;
	context.onLoadedMetadata();
	assert.equal(context.video.currentTime, 87);
	assert.equal(playCount(), 0);
});

test('explicit HLS audio indexes distinguish two streams with the same language', () => {
	const { context, tracks } = player();
	context.hls.audioTracks[1].lang = 'eng';
	tracks[1].lang = 'en';
	context.selectAudio(tracks[1]);
	assert.equal(context.hls.audioTrack, 1);
});
