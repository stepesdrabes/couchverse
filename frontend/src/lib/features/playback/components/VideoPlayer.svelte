<script lang="ts">
	import { goto } from '$app/navigation';
	import { Popover } from 'bits-ui';
	import type Hls from 'hls.js';
	import {
		ArrowLeft,
		Captions,
		Check,
		Languages,
		ListVideo,
		Maximize,
		Minimize,
		Pause,
		PictureInPicture2,
		Play,
		RotateCcw,
		RotateCw,
		Settings,
		Shuffle,
		Type,
		Volume2,
		VolumeX
	} from 'lucide-svelte';
	import { onMount } from 'svelte';
	import { fade, fly, scale } from 'svelte/transition';
	import Artwork from '$lib/features/catalog/components/Artwork.svelte';
	import { musicPlayer } from '$lib/features/music/player.svelte';
	import type {
		AudioTrack,
		EpisodeRef,
		PlaybackInfo,
		SeriesEpisode
	} from '$lib/features/playback/api';
	import {
		beaconProgress,
		frameUrl,
		jitKeepalive,
		reportProgress
	} from '$lib/features/playback/api';
	import {
		preferences,
		SUBTITLE_FONTS,
		type SubtitleSettings
	} from '$lib/features/preferences/preferences.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Tooltip from '$lib/components/ui/Tooltip.svelte';
	import { accentVars } from '$lib/theme';
	import { formatClock } from '$lib/utils/format';
	import { couch } from '$lib/features/couch/couch.svelte';
	import CouchBar from '$lib/features/couch/components/CouchBar.svelte';
	import CouchButton from '$lib/features/couch/components/CouchButton.svelte';
	import HostAwayOverlay from '$lib/features/couch/components/HostAwayOverlay.svelte';
	import * as m from '$lib/paraglide/messages';

	let {
		info,
		titleId = null,
		episodeId = null,
		jitSessionId = null
	}: {
		info: PlaybackInfo;
		titleId?: string | null;
		episodeId?: string | null;
		jitSessionId?: string | null;
	} = $props();

	let video = $state<HTMLVideoElement>();
	let wrapper = $state<HTMLDivElement>();

	let playing = $state(false);
	let hasPlayed = $state(false); // suppress the pause indicator before autoplay starts
	let buffering = $state(false); // media is stalled waiting for data
	let currentTime = $state(0);
	let duration = $state(info.durationSeconds || 0);
	let buffered = $state<{ start: number; end: number }[]>([]);
	let volume = $state(Number(localStorage.getItem('cv.volume') ?? 1));
	let muted = $state(false);
	let fullscreen = $state(false);
	let pipActive = $state(false);
	const pipSupported = typeof document !== 'undefined' && document.pictureInPictureEnabled === true;
	let controlsVisible = $state(true);
	let nextCountdown = $state<number | null>(null);

	let hideTimer: ReturnType<typeof setTimeout>;
	let lastReported = 0;
	// actually-played seconds since the last beacon (pauses/seeks excluded)
	let watchedSeconds = 0;
	let lastTickTime = -1;

	// subtitle selection: track id or null (off); restore the preferred language
	let activeSub = $state<string | null>(null);

	// cues render in a custom overlay; tracks stay hidden so they still load and fire cuechange
	let cueHtml = $state('');
	let cueTrack: TextTrack | null = null;

	function syncCues(track: TextTrack) {
		if (!track.activeCues?.length) {
			cueHtml = '';
			return;
		}
		const div = document.createElement('div');
		for (let i = 0; i < track.activeCues.length; i++) {
			div.append((track.activeCues[i] as VTTCue).getCueAsHTML());
		}
		cueHtml = div.innerHTML;
	}

	function handleCueChange() {
		if (cueTrack) syncCues(cueTrack);
	}

	function detachCueListener() {
		cueTrack?.removeEventListener('cuechange', handleCueChange);
		cueTrack = null;
		cueHtml = '';
	}

	function applySubtitles() {
		if (!video) return;
		detachCueListener();
		const selected = info.subtitles.findIndex((s) => s.id === activeSub);
		for (let i = 0; i < video.textTracks.length; i++) {
			video.textTracks[i].mode = 'hidden';
		}
		if (selected >= 0 && video.textTracks[selected]) {
			cueTrack = video.textTracks[selected];
			// in PiP the browser only paints the video element, so let it render
			// native captions; otherwise keep them hidden and use the custom overlay
			cueTrack.mode = pipActive ? 'showing' : 'hidden';
			cueTrack.addEventListener('cuechange', handleCueChange);
			syncCues(cueTrack);
		}
	}

	function selectSubtitle(id: string | null) {
		activeSub = id;
		const lang = info.subtitles.find((s) => s.id === id)?.lang;
		if (lang) localStorage.setItem('cv.subLang', lang);
		else localStorage.removeItem('cv.subLang');
		applySubtitles();
	}

	function cycleSubtitle() {
		if (info.subtitles.length === 0) return;
		const idx = info.subtitles.findIndex((s) => s.id === activeSub);
		const next = idx + 1 >= info.subtitles.length ? null : info.subtitles[idx + 1].id;
		selectSubtitle(next ?? null);
	}

	function restorePreferredSubtitle() {
		const preferred = localStorage.getItem('cv.subLang');
		if (!preferred) return;
		const match = info.subtitles.find((s) => s.lang === preferred);
		if (match) {
			activeSub = match.id;
			applySubtitles();
		}
	}

	// subtitle appearance, persisted per account
	let subStyle = $state<SubtitleSettings>({ ...preferences.subtitles });
	const subCssVars = $derived(
		`--sub-scale:${subStyle.fontSizePct / 100};` +
			`--sub-color:${subStyle.color};` +
			`--sub-font:${SUBTITLE_FONTS[subStyle.fontFamily]};` +
			`--sub-bg:rgba(0,0,0,${subStyle.backgroundOpacity / 100})`
	);
	let saveSubTimer: ReturnType<typeof setTimeout>;
	function updateSubStyle<K extends keyof SubtitleSettings>(key: K, value: SubtitleSettings[K]) {
		subStyle = { ...subStyle, [key]: value };
		clearTimeout(saveSubTimer);
		saveSubTimer = setTimeout(() => preferences.saveSubtitles(subStyle).catch(() => {}), 600);
	}

	// episode switcher: group the series' playable episodes by season
	const episodesBySeason = $derived.by(() => {
		// transient within the derived, recomputed each run - not reactive state
		// eslint-disable-next-line svelte/prefer-svelte-reactivity
		const groups = new Map<number, SeriesEpisode[]>();
		for (const ep of info.episodes ?? []) {
			const list = groups.get(ep.seasonNumber) ?? [];
			list.push(ep);
			groups.set(ep.seasonNumber, list);
		}
		return [...groups.entries()].sort((a, b) => a[0] - b[0]);
	});

	const currentSeasonNumber = $derived(
		(info.episodes ?? []).find((e) => e.episodeId === info.currentEpisodeId)?.seasonNumber ??
			episodesBySeason[0]?.[0] ??
			null
	);
	// the season dropdown follows the playing season until the user picks one
	let seasonValue = $state('');
	const activeSeason = $derived(seasonValue ? Number(seasonValue) : currentSeasonNumber);
	const seasonEpisodes = $derived(episodesBySeason.find(([n]) => n === activeSeason)?.[1] ?? []);

	function openEpisode(episodeId: string) {
		if (episodeId === info.currentEpisodeId) return;
		report();
		goto(`/watch/episode/${episodeId}`, { invalidateAll: true });
	}

	// shuffle: when enabled on a flagged series, auto-next jumps to a random episode.
	// The choice persists globally (like volume) but only acts on flagged multi-episode series.
	let shuffle = $state(localStorage.getItem('cv.shuffle') === '1');
	const canShuffle = $derived(!!info.allowRandomPlayback && (info.episodes?.length ?? 0) > 1);
	function toggleShuffle() {
		shuffle = !shuffle;
		localStorage.setItem('cv.shuffle', shuffle ? '1' : '0');
	}
	// the decided upcoming episode, held in state so the up-next card and the actual
	// jump agree (picking inside a derived would re-randomise on every render)
	let nextTarget = $state<SeriesEpisode | EpisodeRef | null>(null);
	function pickNextTarget(): SeriesEpisode | EpisodeRef | null {
		if (shuffle && canShuffle) {
			const pool = (info.episodes ?? []).filter((e) => e.episodeId !== info.currentEpisodeId);
			if (pool.length) return pool[Math.floor(Math.random() * pool.length)];
		}
		return info.nextEpisode;
	}

	const remaining = $derived(duration - currentTime);
	const progressBody = () => {
		const watched = Math.floor(watchedSeconds);
		watchedSeconds -= watched; // keep the sub-second remainder
		return {
			...(titleId ? { titleId } : { episodeId: episodeId! }),
			positionSeconds: Math.floor(currentTime),
			durationSeconds: Math.floor(duration),
			watchedSeconds: watched
		};
	};

	function report() {
		// followers never post progress: anonymous can't, and logged-in followers
		// must not pollute their own watch-time/analytics with a synced session
		if (couch.isFollower) return;
		if (currentTime < 5) return;
		lastReported = currentTime;
		reportProgress(progressBody()).catch(() => {});
	}

	function poke() {
		controlsVisible = true;
		clearTimeout(hideTimer);
		hideTimer = setTimeout(() => {
			if (playing) controlsVisible = false;
		}, 3000);
	}

	function togglePlay() {
		if (!video) return;
		// a follower's pause/unpause is local: unpausing resyncs to the host
		if (couch.isFollower) {
			if (video.paused) couch.onLocalUnpause();
			else {
				couch.markLocalPause();
				video.pause();
			}
			return;
		}
		if (video.paused) video.play();
		else video.pause();
	}

	// center skip indicator: accumulates the amount on rapid presses (10s, 20s...)
	let skipDir = $state<-1 | 1>(1);
	let skipAmount = $state(0);
	let skipSeq = $state(0);
	let skipHideTimer: ReturnType<typeof setTimeout>;

	function showSkip(seconds: number) {
		const dir = seconds < 0 ? -1 : 1;
		if (dir === skipDir && skipAmount > 0) skipAmount += Math.abs(seconds);
		else {
			skipDir = dir;
			skipAmount = Math.abs(seconds);
		}
		skipSeq++;
		clearTimeout(skipHideTimer);
		skipHideTimer = setTimeout(() => (skipAmount = 0), 700);
	}

	function skip(seconds: number) {
		if (!video || couch.followerLocked) return; // followers have no timeline control
		video.currentTime = Math.min(Math.max(0, video.currentTime + seconds), duration);
		showSkip(seconds);
		couch.onSeek(video.currentTime);
	}

	function setVolume(v: number) {
		volume = Math.min(1, Math.max(0, v));
		muted = volume === 0;
		localStorage.setItem('cv.volume', String(volume));
	}

	function toggleFullscreen() {
		if (document.fullscreenElement) document.exitFullscreen();
		else wrapper?.requestFullscreen();
	}

	async function togglePip() {
		if (!video) return;
		try {
			if (document.pictureInPictureElement) await document.exitPictureInPicture();
			else await video.requestPictureInPicture();
		} catch {
			// PiP unsupported or blocked by the browser
		}
	}

	function onPipChange(active: boolean) {
		pipActive = active;
		applySubtitles(); // swap captions between native (PiP) and the overlay
	}

	function onTimeUpdate() {
		if (!video) return;
		// timeupdate fires ~4x/s while playing; bigger jumps are seeks
		const tick = video.currentTime - lastTickTime;
		if (lastTickTime >= 0 && tick > 0 && tick < 2) watchedSeconds += tick;
		lastTickTime = video.currentTime;
		currentTime = video.currentTime;
		if (currentTime - lastReported >= 10) report();

		// auto-next countdown in the last 20 seconds (a follower's episode changes
		// only when the host switches, never via local autoplay). Shuffle also
		// advances past the last episode; the target is decided once here.
		const hasNext = info.nextEpisode || (shuffle && canShuffle);
		if (
			hasNext &&
			!couch.isFollower &&
			remaining <= 20 &&
			remaining > 0 &&
			nextCountdown === null
		) {
			nextTarget = pickNextTarget();
			nextCountdown = Math.ceil(remaining);
		}
		if (nextCountdown !== null) {
			nextCountdown = Math.max(0, Math.ceil(remaining));
		}
	}

	function onProgress() {
		if (!video) return;
		const ranges = [];
		for (let i = 0; i < video.buffered.length; i++) {
			ranges.push({ start: video.buffered.start(i), end: video.buffered.end(i) });
		}
		buffered = ranges;
	}

	function goNextEpisode() {
		if (couch.isFollower) return;
		const target = nextTarget ?? pickNextTarget();
		if (!target) return;
		report();
		goto(`/watch/episode/${target.episodeId}`, { invalidateAll: true });
	}

	function onEnded() {
		cueHtml = '';
		buffering = false;
		// a follower stays put at the end; the host's next-media choice drives it
		if (couch.isFollower) return;
		const target = nextTarget ?? pickNextTarget();
		report();
		if (target) goto(`/watch/episode/${target.episodeId}`, { invalidateAll: true });
		else goto(`/title/${info.display.titleSlug}`);
	}

	function seekTo(event: PointerEvent, track: HTMLElement) {
		if (couch.followerLocked) return; // host-only timeline
		const rect = track.getBoundingClientRect();
		const ratio = Math.min(1, Math.max(0, (event.clientX - rect.left) / rect.width));
		if (video) {
			video.currentTime = ratio * duration;
			couch.onSeek(video.currentTime);
		}
	}

	let scrubbing = $state(false);

	// banner accent for the player, from the title backdrop's server-extracted colour
	const accentStyle = $derived(
		info.display.backdropAccent ? accentVars(info.display.backdropAccent) : ''
	);
	let hoverRatio = $state<number | null>(null);
	const hoverTime = $derived(hoverRatio !== null ? hoverRatio * duration : 0);
	// bucket to 5s so the preview reuses cached frames while scrubbing
	const previewSrc = $derived(
		hoverRatio !== null ? frameUrl(info.mediaFileId, Math.floor(hoverTime / 5) * 5) : ''
	);

	function onSeekHover(event: PointerEvent, track: HTMLElement) {
		const rect = track.getBoundingClientRect();
		hoverRatio = Math.min(1, Math.max(0, (event.clientX - rect.left) / rect.width));
	}

	function onKeydown(e: KeyboardEvent) {
		if (e.target instanceof HTMLInputElement) return;
		switch (e.key) {
			case ' ':
			case 'k':
				e.preventDefault();
				togglePlay();
				break;
			case 'ArrowLeft':
				skip(-10);
				break;
			case 'ArrowRight':
				skip(10);
				break;
			case 'ArrowUp':
				e.preventDefault();
				setVolume(volume + 0.1);
				break;
			case 'ArrowDown':
				e.preventDefault();
				setVolume(volume - 0.1);
				break;
			case 'f':
				toggleFullscreen();
				break;
			case 'm':
				muted = !muted;
				break;
			case 'c':
				cycleSubtitle();
				break;
		}
		poke();
	}

	// playback source + quality. The source can direct-play the original or
	// stream the transcoded HLS ladder; the quality menu switches between them.
	let hls: Hls | null = null;
	let videoSrc = $state<string | undefined>(info.mode === 'direct' ? info.streamUrl : undefined);
	const directUrl = info.mode === 'direct' ? (info.streamUrl ?? null) : null;
	// initial HLS load uses streamUrl (also the JIT session playlist); quality
	// switches from direct-play use the variants master at hlsUrl
	const initialHlsUrl = info.mode === 'hls' ? (info.streamUrl ?? null) : null;
	const switchHlsUrl = info.hlsUrl ?? initialHlsUrl;

	// 'direct' | 'auto' | a rendition name ("1080p")
	let quality = $state(info.mode === 'direct' ? 'direct' : 'auto');
	let didRestoreSub = false;
	let pendingResume: { at: number; play: boolean } | null = null;

	const qualityOptions = $derived.by(() => {
		const opts: { key: string; label: string }[] = [];
		if (directUrl) opts.push({ key: 'direct', label: m.player_quality_original() });
		if (switchHlsUrl && (info.variants?.length ?? 0) > 0)
			opts.push({ key: 'auto', label: m.player_quality_auto() });
		for (const v of info.variants ?? []) opts.push({ key: v.name, label: `${v.height}p` });
		return opts;
	});

	async function attachHls(url: string, pinName: string | null) {
		if (!video) return;
		// Safari plays HLS natively but exposes no level API - adaptive only
		if (video.canPlayType('application/vnd.apple.mpegurl')) {
			videoSrc = url;
			return;
		}
		const { default: HlsCtor } = await import('hls.js');
		if (!HlsCtor.isSupported()) return;
		hls = new HlsCtor();
		hls.loadSource(url);
		hls.attachMedia(video);
		hls.on(HlsCtor.Events.MANIFEST_PARSED, () => {
			if (!hls) return;
			const idx = pinName ? hls.levels.findIndex((l) => l.name === pinName) : -1;
			hls.currentLevel = idx;
		});
	}

	function selectQuality(key: string) {
		if (key === quality || !video) return;
		pendingResume = { at: video.currentTime, play: !video.paused };
		quality = key;
		hls?.destroy();
		hls = null;
		if (key === 'direct' && directUrl) {
			videoSrc = directUrl;
		} else {
			videoSrc = undefined;
			attachHls(switchHlsUrl!, key === 'auto' ? null : key);
		}
	}

	const audioTracks = $derived(info.audio ?? []);
	let activeAudioId = $state<string | null>(info.audio?.find((a) => a.default)?.id ?? null);

	// Audio switch. Embedded (model A): switch the HLS audio rendition in place via
	// hls.js (or Safari's native video.audioTracks). File (model B): swap the whole
	// source to the chosen-language file and re-seek, since browsers can't switch
	// the audio of a progressive file.
	function selectAudio(track: AudioTrack) {
		if (track.id === activeAudioId || !video) return;
		activeAudioId = track.id;
		if (track.lang) localStorage.setItem('cv.audioLang', track.lang);
		else localStorage.removeItem('cv.audioLang');

		if (track.source === 'embedded') {
			if (hls) {
				const idx = hls.audioTracks.findIndex((t) => t.lang === track.lang);
				if (idx >= 0) hls.audioTrack = idx;
			} else {
				// Safari plays HLS natively and exposes the audio group here
				const native = video as HTMLVideoElement & {
					audioTracks?: { length: number; [i: number]: { language: string; enabled: boolean } };
				};
				const list = native.audioTracks;
				if (list) {
					for (let i = 0; i < list.length; i++) {
						list[i].enabled = list[i].language === track.lang;
					}
				}
			}
			return;
		}

		pendingResume = { at: video.currentTime, play: !video.paused };
		hls?.destroy();
		hls = null;
		if (track.streamUrl) {
			quality = 'direct';
			videoSrc = track.streamUrl;
		} else if (track.hlsUrl) {
			quality = 'auto';
			videoSrc = undefined;
			attachHls(track.hlsUrl, null);
		}
	}

	function onLoadedMetadata() {
		if (!video) return;
		if (pendingResume) {
			video.currentTime = pendingResume.at;
			if (pendingResume.play) video.play().catch(() => {});
			pendingResume = null;
		} else if (info.resumePosition > 5) {
			video.currentTime = info.resumePosition;
		}
		if (!didRestoreSub) {
			restorePreferredSubtitle();
			didRestoreSub = true;
		} else {
			applySubtitles(); // re-bind the active track after a source switch
		}
	}

	// hand the media element to the couch store so it can drive a follower's sync
	// (seek to the host position) and read a host's position for broadcasts
	$effect(() => {
		couch.bindVideo(video);
		return () => couch.bindVideo(undefined);
	});

	// let the couch bar dodge the player controls while they are on screen
	$effect(() => {
		couch.playerControlsVisible = controlsVisible;
	});

	onMount(() => {
		poke();
		couch.playerMounts++; // the on-screen player hosts the couch bar (so it survives fullscreen)
		musicPlayer.pause(); // never play video and music together
		if (info.mode === 'hls' && initialHlsUrl) attachHls(initialHlsUrl, null);

		// JIT sessions are reaped server-side without this heartbeat
		let keepaliveTimer: ReturnType<typeof setInterval> | undefined;
		if (jitSessionId) {
			keepaliveTimer = setInterval(() => jitKeepalive(jitSessionId!).catch(() => {}), 15000);
		}

		const onVisibility = () => {
			if (document.visibilityState === 'hidden' && currentTime > 5 && !couch.isFollower) {
				beaconProgress(progressBody());
			}
		};
		document.addEventListener('visibilitychange', onVisibility);

		// PiP events aren't in Svelte's element typings, so bind them here
		const onEnterPip = () => onPipChange(true);
		const onLeavePip = () => onPipChange(false);
		video?.addEventListener('enterpictureinpicture', onEnterPip);
		video?.addEventListener('leavepictureinpicture', onLeavePip);

		return () => {
			document.removeEventListener('visibilitychange', onVisibility);
			video?.removeEventListener('enterpictureinpicture', onEnterPip);
			video?.removeEventListener('leavepictureinpicture', onLeavePip);
			detachCueListener();
			clearTimeout(hideTimer);
			clearInterval(keepaliveTimer);
			hls?.destroy();
			couch.playerControlsVisible = false;
			couch.playerMounts--;
			if (currentTime > 5 && !couch.isFollower) beaconProgress(progressBody());
		};
	});
</script>

<svelte:window onkeydown={onKeydown} />
<svelte:document onfullscreenchange={() => (fullscreen = !!document.fullscreenElement)} />

<div
	bind:this={wrapper}
	class="relative h-dvh w-full overflow-hidden bg-black {controlsVisible ? '' : 'cursor-none'}"
	style={accentStyle}
	onpointermove={poke}
	role="presentation"
>
	<!-- svelte-ignore a11y_media_has_caption -->
	<video
		bind:this={video}
		src={videoSrc}
		autoplay
		class="size-full object-contain"
		bind:volume
		bind:muted
		onplay={() => {
			playing = true;
			hasPlayed = true;
			couch.onPlayStateChange(true, video?.currentTime ?? 0); // host broadcasts; follower no-op
		}}
		onpause={() => {
			playing = false;
			buffering = false;
			report();
			poke();
			couch.onPlayStateChange(false, video?.currentTime ?? 0);
		}}
		onwaiting={() => (buffering = true)}
		onstalled={() => (buffering = true)}
		onplaying={() => (buffering = false)}
		oncanplay={() => (buffering = false)}
		onseeking={() => {
			if (video) currentTime = video.currentTime;
		}}
		ontimeupdate={onTimeUpdate}
		onprogress={onProgress}
		ondurationchange={() => (duration = video?.duration || info.durationSeconds)}
		onloadedmetadata={onLoadedMetadata}
		onended={onEnded}
		onclick={togglePlay}
		ondblclick={toggleFullscreen}
	>
		{#each info.subtitles as sub (sub.id)}
			<track kind="subtitles" src={sub.url} srclang={sub.lang} label={sub.label} />
		{/each}
	</video>

	{#if cueHtml && !pipActive}
		<div class="subtitle-overlay" class:raised={controlsVisible} style={subCssVars}>
			<!-- cue markup comes from the browser's own VTT parser (getCueAsHTML) -->
			<!-- eslint-disable-next-line svelte/no-at-html-tags -->
			{@html cueHtml}
		</div>
	{/if}

	<!-- centre pause indicator (clicks pass through to the video) -->
	{#if !playing && hasPlayed && !buffering}
		<div
			transition:scale={{ duration: 220, start: 0.6 }}
			class="pointer-events-none absolute inset-0 flex items-center justify-center"
		>
			<span
				class="flex size-20 items-center justify-center rounded-full bg-black/45 text-white
					shadow-xl shadow-black/40 backdrop-blur-sm"
			>
				<Play class="size-9 translate-x-0.5 fill-current" />
			</span>
		</div>
	{/if}

	<!-- buffering spinner while the media stalls for data -->
	{#if buffering && !couch.waiting}
		<div
			transition:fade={{ duration: 150 }}
			class="pointer-events-none absolute inset-0 flex items-center justify-center"
		>
			<span
				class="flex size-16 items-center justify-center rounded-full bg-black/45 text-white
					shadow-xl shadow-black/40 backdrop-blur-sm"
			>
				<svg class="size-9 animate-spin" viewBox="0 0 24 24" fill="none" aria-hidden="true">
					<circle
						class="opacity-20"
						cx="12"
						cy="12"
						r="10"
						stroke="currentColor"
						stroke-width="2.5"
					/>
					<path
						class="opacity-90"
						d="M22 12a10 10 0 0 1-10 10"
						stroke="var(--color-accent)"
						stroke-width="2.5"
						stroke-linecap="round"
					/>
				</svg>
			</span>
		</div>
	{/if}

	<!-- skip indicators: a directional pill that pops on each +/-10s -->
	{#if skipAmount > 0}
		<div
			transition:fade={{ duration: 180 }}
			class="pointer-events-none absolute inset-y-0 flex items-center
				{skipDir < 0 ? 'left-[6%] justify-start' : 'right-[6%] justify-end'}"
		>
			{#key skipSeq}
				<div
					class="skip-pop flex flex-col items-center gap-1.5 rounded-2xl bg-black/55 px-7 py-6
						text-white backdrop-blur-sm"
				>
					{#if skipDir < 0}
						<RotateCcw class="size-8" />
					{:else}
						<RotateCw class="size-8" />
					{/if}
					<span class="text-sm font-semibold tnum">{skipAmount}s</span>
				</div>
			{/key}
		</div>
	{/if}

	<!-- the couch bar lives inside the player wrapper so it is part of the
		fullscreen subtree (a fixed root-layout element would vanish in fullscreen),
		and it rises above the controls while they're shown -->
	{#if couch.active}
		<div
			class="absolute right-4 z-30"
			style="bottom: {controlsVisible ? '5.5rem' : '1rem'}; transition: bottom 0.25s ease;"
		>
			<CouchBar portalTo={wrapper} showEmoji={controlsVisible} />
		</div>
	{/if}

	<!-- couch follower states: host away/choosing, host paused, transient resync -->
	{#if couch.waiting}
		<HostAwayOverlay />
	{/if}
	{#if couch.hostPaused}
		<div
			transition:fade={{ duration: 150 }}
			class="pointer-events-none absolute top-6 left-1/2 z-30 -translate-x-1/2 rounded-full
				bg-black/70 px-4 py-1.5 text-sm font-medium text-white backdrop-blur"
		>
			{m.couch_host_paused()}
		</div>
	{/if}
	{#if couch.resyncVisible}
		<div
			transition:fade={{ duration: 150 }}
			class="pointer-events-none absolute top-6 left-1/2 z-30 -translate-x-1/2 rounded-full
				bg-accent/90 px-4 py-1.5 text-sm font-medium text-[var(--color-on-accent)]"
		>
			{m.couch_resynced()}
		</div>
	{/if}

	{#if controlsVisible}
		<!-- top bar -->
		<div
			transition:fade={{ duration: 200 }}
			class="absolute inset-x-0 top-0 flex items-center gap-4 bg-gradient-to-b from-black/80
				to-transparent p-5 pb-12"
		>
			{#if !couch.isFollower}
				<Tooltip label={m.player_back_to_title()} side="bottom" portalTo={wrapper}>
					{#snippet trigger(props)}
						<button
							{...props}
							class="rounded-full p-2 text-white/80 transition-colors hover:bg-white/10 hover:text-white"
							onclick={() => goto(`/title/${info.display.titleSlug}`)}
							aria-label={m.common_back()}
						>
							<ArrowLeft class="size-5" />
						</button>
					{/snippet}
				</Tooltip>
			{/if}
			<div class="min-w-0">
				<p class="truncate font-semibold text-white">{info.display.title}</p>
				{#if info.display.subtitle}
					<p class="truncate text-xs text-white/60">{info.display.subtitle}</p>
				{/if}
			</div>
		</div>

		<!-- bottom controls -->
		<div
			transition:fade={{ duration: 200 }}
			class="absolute inset-x-0 bottom-0 bg-linear-to-t from-black/90 to-transparent px-5 pt-16 pb-5"
		>
			<!-- seek bar (followers have no timeline control) -->
			<div
				class="group/seek relative mb-4 h-1 w-full rounded-full bg-white/20"
				class:cursor-pointer={!couch.followerLocked}
				class:pointer-events-none={couch.followerLocked}
				class:opacity-70={couch.followerLocked}
				onpointerdown={(e) => {
					scrubbing = true;
					seekTo(e, e.currentTarget);
					e.currentTarget.setPointerCapture(e.pointerId);
				}}
				onpointermove={(e) => {
					onSeekHover(e, e.currentTarget);
					if (scrubbing) seekTo(e, e.currentTarget);
				}}
				onpointerup={() => (scrubbing = false)}
				onpointerleave={() => (hoverRatio = null)}
				role="slider"
				aria-label={m.player_seek()}
				aria-valuemin={0}
				aria-valuemax={duration}
				aria-valuenow={currentTime}
				tabindex="0"
			>
				{#if hoverRatio !== null && duration > 0}
					<div
						class="pointer-events-none absolute bottom-full mb-3 flex -translate-x-1/2 flex-col
							items-center gap-1"
						style="left: {Math.min(96, Math.max(4, hoverRatio * 100))}%"
					>
						{#key previewSrc}
							<img
								src={previewSrc}
								alt=""
								class="h-[4.5rem] w-32 rounded-md border border-white/15 bg-black object-cover shadow-xl"
								onerror={(e) => ((e.currentTarget as HTMLImageElement).style.display = 'none')}
							/>
						{/key}
						<span class="rounded bg-black/85 px-1.5 py-0.5 text-[11px] font-medium text-white tnum">
							{formatClock(hoverTime)}
						</span>
					</div>
				{/if}
				{#each buffered as range (range.start)}
					<div
						class="absolute h-full rounded-full bg-white/25"
						style="left: {(range.start / duration) * 100}%; width: {((range.end - range.start) /
							duration) *
							100}%"
					></div>
				{/each}
				<div
					class="relative h-full rounded-full bg-accent"
					style="width: {duration ? (currentTime / duration) * 100 : 0}%"
				>
					<span
						class="absolute top-1/2 -right-1.5 size-3 -translate-y-1/2 scale-0 rounded-full
							bg-accent shadow transition-transform group-hover/seek:scale-100"
					></span>
				</div>
			</div>

			<div class="flex items-center gap-3">
				<Tooltip label={playing ? m.common_pause() : m.common_play()} portalTo={wrapper}>
					{#snippet trigger(props)}
						<button
							{...props}
							class="player-btn"
							onclick={togglePlay}
							aria-label={m.player_play_pause()}
						>
							{#if playing}
								<Pause class="size-5 fill-current" />
							{:else}
								<Play class="size-5 fill-current" />
							{/if}
						</button>
					{/snippet}
				</Tooltip>
				{#if !couch.followerLocked}
					<Tooltip label={m.player_back_10_seconds()} portalTo={wrapper}>
						{#snippet trigger(props)}
							<button
								{...props}
								class="player-btn"
								onclick={() => skip(-10)}
								aria-label={m.player_back_10_seconds()}
							>
								<RotateCcw class="size-4.5" />
							</button>
						{/snippet}
					</Tooltip>
					<Tooltip label={m.player_forward_10_seconds()} portalTo={wrapper}>
						{#snippet trigger(props)}
							<button
								{...props}
								class="player-btn"
								onclick={() => skip(10)}
								aria-label={m.player_forward_10_seconds()}
							>
								<RotateCw class="size-4.5" />
							</button>
						{/snippet}
					</Tooltip>
				{/if}

				<div class="group/vol flex items-center gap-2">
					<Tooltip label={muted ? m.player_unmute() : m.player_mute()} portalTo={wrapper}>
						{#snippet trigger(props)}
							<button
								{...props}
								class="player-btn"
								onclick={() => (muted = !muted)}
								aria-label={m.player_mute()}
							>
								{#if muted || volume === 0}
									<VolumeX class="size-4.5" />
								{:else}
									<Volume2 class="size-4.5" />
								{/if}
							</button>
						{/snippet}
					</Tooltip>
					<input
						type="range"
						min="0"
						max="1"
						step="0.05"
						value={muted ? 0 : volume}
						oninput={(e) => setVolume(Number(e.currentTarget.value))}
						class="volume-slider w-0 opacity-0 transition-all duration-200
							group-hover/vol:w-20 group-hover/vol:opacity-100"
						aria-label={m.player_volume()}
					/>
				</div>

				<span class="ml-2 text-xs text-white/70 tnum">
					{formatClock(currentTime)} / {formatClock(duration)}
				</span>

				<div class="flex-1"></div>

				<CouchButton
					kind={titleId ? 'movie' : 'episode'}
					id={titleId ?? episodeId ?? ''}
					portalTo={wrapper}
				/>

				{#if canShuffle && !couch.isFollower}
					<Tooltip label={m.player_shuffle()} portalTo={wrapper}>
						{#snippet trigger(props)}
							<button
								{...props}
								class="player-btn {shuffle ? 'shuffle-on' : ''}"
								onclick={toggleShuffle}
								aria-label={m.player_shuffle()}
							>
								<Shuffle class="size-5" />
							</button>
						{/snippet}
					</Tooltip>
				{/if}

				{#if episodesBySeason.length > 0 && !couch.isFollower}
					<Popover.Root>
						<Popover.Trigger
							class="player-btn"
							aria-label={m.player_episodes()}
							title={m.player_episodes()}
						>
							<ListVideo class="size-5" />
						</Popover.Trigger>
						<Popover.Portal to={wrapper}>
							<Popover.Content
								side="top"
								sideOffset={10}
								class="z-50 flex max-h-[65vh] w-[26rem] max-w-[calc(100vw-2rem)] animate-pop-in flex-col rounded-card border border-edge bg-surface-2/95 shadow-xl backdrop-blur"
							>
								<div
									class="flex items-center justify-between gap-2 border-b border-edge/70 px-3 py-2.5"
								>
									<p class="text-xs font-semibold">{m.player_episodes()}</p>
									{#if episodesBySeason.length > 1}
										<Select
											bind:value={seasonValue}
											label={m.catalog_season()}
											placeholder={currentSeasonNumber !== null
												? m.catalog_season_number({ number: currentSeasonNumber })
												: ''}
											items={episodesBySeason.map(([n]) => ({
												value: String(n),
												label: m.catalog_season_number({ number: n })
											}))}
											portalTo={wrapper}
										/>
									{/if}
								</div>
								<div class="space-y-1 overflow-y-auto p-2 scrollbar-none">
									{#each seasonEpisodes as ep (ep.episodeId)}
										{@const current = ep.episodeId === info.currentEpisodeId}
										<button
											class="group flex w-full gap-3 rounded-lg p-1.5 text-left transition-colors
												{current ? 'bg-surface' : 'hover:bg-surface'}"
											onclick={() => openEpisode(ep.episodeId)}
										>
											<div
												class="relative aspect-video w-28 shrink-0 overflow-hidden rounded-md border
													border-edge/60 bg-surface-2"
											>
												<Artwork
													artworkId={ep.thumbId ?? null}
													v={ep.thumbVer}
													name={ep.name || m.player_episode_number({ number: ep.episodeNumber })}
												/>
												<div
													class="absolute inset-0 flex items-center justify-center bg-black/45 transition-opacity
														{current ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'}"
												>
													<span
														class="rounded-full bg-accent p-1.5 text-[var(--color-on-accent)] shadow-lg"
													>
														<Play class="size-3.5 fill-current" />
													</span>
												</div>
											</div>
											<div class="min-w-0 flex-1 py-0.5">
												<div class="flex items-center gap-1.5">
													<span class="text-xs font-semibold text-faint tnum"
														>E{ep.episodeNumber}</span
													>
													{#if current}
														<span class="text-[10px] font-semibold text-accent"
															>{m.player_now_playing()}</span
														>
													{/if}
												</div>
												<p
													class="mt-0.5 line-clamp-2 text-xs font-medium
														{current ? 'text-text' : 'text-muted'} group-hover:text-accent"
												>
													{ep.name || m.player_episode_number({ number: ep.episodeNumber })}
												</p>
											</div>
										</button>
									{/each}
								</div>
							</Popover.Content>
						</Popover.Portal>
					</Popover.Root>
				{/if}

				{#if qualityOptions.length > 1}
					<Popover.Root>
						<Popover.Trigger
							class="player-btn"
							aria-label={m.player_quality()}
							title={m.player_quality()}
						>
							<Settings class="size-4.5" />
						</Popover.Trigger>
						<Popover.Portal to={wrapper}>
							<Popover.Content
								side="top"
								sideOffset={10}
								class="z-50 w-40 animate-pop-in rounded-card border border-edge bg-surface-2/95 p-1 shadow-xl backdrop-blur"
							>
								<p
									class="px-3 py-1.5 text-[10px] font-semibold tracking-widest text-faint uppercase"
								>
									{m.player_quality()}
								</p>
								{#each qualityOptions as opt (opt.key)}
									<button
										class="flex w-full items-center justify-between rounded-lg px-3 py-1.5 text-left text-xs
											{quality === opt.key ? 'text-text' : 'text-muted'} hover:bg-surface"
										onclick={() => selectQuality(opt.key)}
									>
										{opt.label}
										{#if quality === opt.key}<Check class="size-3.5 text-accent" />{/if}
									</button>
								{/each}
							</Popover.Content>
						</Popover.Portal>
					</Popover.Root>
				{/if}

				{#if audioTracks.length > 1}
					<Popover.Root>
						<Popover.Trigger
							class="player-btn"
							aria-label={m.player_audio()}
							title={m.player_audio()}
						>
							<Languages class="size-5" />
						</Popover.Trigger>
						<Popover.Portal to={wrapper}>
							<Popover.Content
								side="top"
								sideOffset={10}
								class="z-50 w-44 animate-pop-in rounded-card border border-edge bg-surface-2/95 p-1 shadow-xl backdrop-blur"
							>
								<p
									class="px-3 py-1.5 text-[10px] font-semibold tracking-widest text-faint uppercase"
								>
									{m.player_audio()}
								</p>
								{#each audioTracks as track (track.id)}
									<button
										class="flex w-full items-center justify-between rounded-lg px-3 py-1.5 text-left text-xs
											{activeAudioId === track.id ? 'text-text' : 'text-muted'} hover:bg-surface"
										onclick={() => selectAudio(track)}
									>
										{track.label}
										{#if activeAudioId === track.id}<Check class="size-3.5 text-accent" />{/if}
									</button>
								{/each}
							</Popover.Content>
						</Popover.Portal>
					</Popover.Root>
				{/if}

				{#if info.subtitles.length > 0}
					<Popover.Root>
						<Popover.Trigger
							class="player-btn {activeSub !== null ? 'text-accent!' : ''}"
							aria-label={m.player_subtitles()}
							title={m.player_subtitles()}
						>
							<Captions class="size-5" />
						</Popover.Trigger>
						<Popover.Portal to={wrapper}>
							<Popover.Content
								side="top"
								sideOffset={10}
								class="z-50 max-h-[70vh] w-64 animate-pop-in overflow-y-auto rounded-card border border-edge bg-surface-2/95 p-1 shadow-xl backdrop-blur scrollbar-none"
							>
								<p
									class="px-3 py-1.5 text-[10px] font-semibold tracking-widest text-faint uppercase"
								>
									{m.player_subtitles()}
								</p>
								<button
									class="flex w-full items-center justify-between rounded-lg px-3 py-1.5 text-left text-xs
										{activeSub === null ? 'text-text' : 'text-muted'} hover:bg-surface"
									onclick={() => selectSubtitle(null)}
								>
									{m.player_subtitle_off()}
									{#if activeSub === null}<Check class="size-3.5 text-accent" />{/if}
								</button>
								{#each info.subtitles as sub (sub.id)}
									<button
										class="flex w-full items-center justify-between rounded-lg px-3 py-1.5 text-left text-xs
											{activeSub === sub.id ? 'text-text' : 'text-muted'} hover:bg-surface"
										onclick={() => selectSubtitle(sub.id)}
									>
										{sub.label}
										{#if activeSub === sub.id}<Check class="size-3.5 text-accent" />{/if}
									</button>
								{/each}

								<div class="mt-1 border-t border-edge/70 pt-2">
									<p
										class="flex items-center gap-1.5 px-3 py-1 text-[10px] font-semibold tracking-widest text-faint uppercase"
									>
										<Type class="size-3" />
										{m.player_subtitle_appearance()}
									</p>

									<div class="space-y-2.5 px-3 py-1.5">
										<label class="block">
											<span class="mb-1 flex justify-between text-[11px] text-muted">
												{m.player_subtitle_size()} <span class="tnum">{subStyle.fontSizePct}%</span>
											</span>
											<input
												type="range"
												min="50"
												max="200"
												step="10"
												value={subStyle.fontSizePct}
												oninput={(e) =>
													updateSubStyle('fontSizePct', Number(e.currentTarget.value))}
												class="volume-slider w-full"
												aria-label={m.player_subtitle_size()}
											/>
										</label>

										<label class="block">
											<span class="mb-1 flex justify-between text-[11px] text-muted">
												{m.player_subtitle_background()}
												<span class="tnum">{subStyle.backgroundOpacity}%</span>
											</span>
											<input
												type="range"
												min="0"
												max="100"
												step="5"
												value={subStyle.backgroundOpacity}
												oninput={(e) =>
													updateSubStyle('backgroundOpacity', Number(e.currentTarget.value))}
												class="volume-slider w-full"
												aria-label={m.player_subtitle_background_opacity()}
											/>
										</label>

										<div class="flex items-center justify-between">
											<span class="text-[11px] text-muted">{m.player_subtitle_font()}</span>
											<div class="flex gap-1">
												{#each ['sans', 'serif', 'rounded', 'mono'] as const as font (font)}
													<button
														class="rounded px-2 py-1 text-[10px] capitalize transition-colors
															{subStyle.fontFamily === font
															? 'bg-accent text-[var(--color-on-accent)]'
															: 'bg-surface text-muted hover:text-text'}"
														onclick={() => updateSubStyle('fontFamily', font)}
													>
														{font}
													</button>
												{/each}
											</div>
										</div>

										<div class="flex items-center justify-between">
											<span class="text-[11px] text-muted">{m.player_subtitle_colour()}</span>
											<div class="flex items-center gap-1.5">
												{#each ['#ffffff', '#ffe600', '#7cf0a0', '#69b4ff'] as swatch (swatch)}
													<button
														class="size-5 rounded-full border-2 transition-transform hover:scale-110
															{subStyle.color.toLowerCase() === swatch ? 'border-text' : 'border-transparent'}"
														style="background: {swatch}"
														onclick={() => updateSubStyle('color', swatch)}
														aria-label={m.player_subtitle_colour_swatch({ colour: swatch })}
													></button>
												{/each}
											</div>
										</div>
									</div>
								</div>
							</Popover.Content>
						</Popover.Portal>
					</Popover.Root>
				{/if}

				{#if pipSupported}
					<Tooltip label={m.player_picture_in_picture()} portalTo={wrapper}>
						{#snippet trigger(props)}
							<button
								{...props}
								class="player-btn {pipActive ? 'text-accent!' : ''}"
								onclick={togglePip}
								aria-label={m.player_picture_in_picture()}
							>
								<PictureInPicture2 class="size-4.5" />
							</button>
						{/snippet}
					</Tooltip>
				{/if}

				<Tooltip
					label={fullscreen ? m.player_exit_fullscreen() : m.player_fullscreen()}
					portalTo={wrapper}
				>
					{#snippet trigger(props)}
						<button
							{...props}
							class="player-btn"
							onclick={toggleFullscreen}
							aria-label={m.player_fullscreen()}
						>
							{#if fullscreen}
								<Minimize class="size-4.5" />
							{:else}
								<Maximize class="size-4.5" />
							{/if}
						</button>
					{/snippet}
				</Tooltip>
			</div>
		</div>
	{/if}

	{#if nextTarget && nextCountdown !== null && nextCountdown > 0}
		<div
			transition:fly={{ y: 24, duration: 250 }}
			class="absolute right-6 bottom-24 w-72 rounded-card border border-edge bg-surface-2/95
				p-4 shadow-2xl shadow-black/60 backdrop-blur"
		>
			<p class="eyebrow mb-1 flex items-center gap-1.5">
				{#if shuffle && canShuffle}<Shuffle class="size-3" />{/if}
				{m.player_up_next({ seconds: nextCountdown })}
			</p>
			<p class="truncate text-sm font-semibold">
				S{nextTarget.seasonNumber} E{nextTarget.episodeNumber}
				{nextTarget.name ? `· ${nextTarget.name}` : ''}
			</p>
			<div class="mt-3 flex gap-2">
				<button
					class="h-8 flex-1 rounded-full bg-accent text-xs font-semibold text-white transition-colors hover:bg-accent-strong"
					onclick={goNextEpisode}
				>
					{m.player_play_now()}
				</button>
				<button
					class="h-8 rounded-full px-3 text-xs font-semibold text-muted transition-colors hover:bg-surface hover:text-text"
					onclick={() => (nextCountdown = null)}
				>
					{m.common_cancel()}
				</button>
			</div>
		</div>
	{/if}
</div>

<style lang="scss">
	// each +/-10s press re-keys the pill so this pop replays
	.skip-pop {
		animation: skip-pop 0.4s ease-out;
	}
	@keyframes skip-pop {
		0% {
			transform: scale(0.7);
			opacity: 0.5;
		}
		45% {
			transform: scale(1.08);
			opacity: 1;
		}
		100% {
			transform: scale(1);
			opacity: 1;
		}
	}

	:global(.player-btn) {
		border-radius: 9999px;
		padding: 0.5rem;
		color: rgb(255 255 255 / 0.85);
		transition:
			background-color 0.15s,
			color 0.15s;

		&:hover {
			background-color: rgb(255 255 255 / 0.1);
			color: white;
		}
	}

	:global(.player-btn.shuffle-on),
	:global(.player-btn.shuffle-on:hover) {
		color: var(--color-accent);
		background-color: var(--color-accent-soft);
	}

	.volume-slider {
		accent-color: var(--color-accent);
	}

	.subtitle-overlay {
		--sub-scale: 1;
		--sub-color: #fff;
		--sub-font: var(--font-sans);
		--sub-bg: rgb(0 0 0 / 0.55);

		position: absolute;
		left: 50%;
		bottom: 4.5rem;
		transform: translateX(-50%);
		max-width: 85%;
		padding: 0.25em 0.6em;
		border-radius: 0.5rem;
		background: var(--sub-bg);
		color: var(--sub-color);
		font-family: var(--sub-font);
		text-align: center;
		text-shadow: 0 1px 3px rgb(0 0 0 / 0.9);
		font-size: clamp(1rem, calc(2.2vw * var(--sub-scale)), 2.6rem);
		line-height: 1.4;
		white-space: pre-line;
		pointer-events: none;
		transition: bottom 0.25s ease;

		&.raised {
			bottom: 8rem;
		}

		:global(b) {
			font-weight: 700;
		}
		:global(i) {
			font-style: italic;
		}
		:global(u) {
			text-decoration: underline;
		}
	}
</style>
