<script lang="ts">
	import { goto, preloadData } from '$app/navigation';
	import { Popover } from 'bits-ui';
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
	import { onMount, tick } from 'svelte';
	import { fade, fly, scale } from 'svelte/transition';
	import { core } from '$lib/core';
	import Artwork from '$lib/features/catalog/components/Artwork.svelte';
	import { QualityKind, type PlayerView, type QualityOption } from '$lib/generated/core';
	import { ElementPlayer } from '$lib/features/playback/element-player.svelte';
	import { PLAYER, sameTarget } from '$lib/features/playback/player';
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
	import AchievementOverlay from '$lib/features/ranks/components/AchievementOverlay.svelte';
	import { rank } from '$lib/features/ranks/rank.svelte';
	import { features } from '$lib/features/settings/features.svelte';
	import CouchButton from '$lib/features/couch/components/CouchButton.svelte';
	import HostAwayOverlay from '$lib/features/couch/components/HostAwayOverlay.svelte';
	import { focusable } from '$lib/tv/spatial-nav';
	import { isTV, mediaKey, type MediaKey } from '$lib/tv/tv';
	import * as m from '$lib/paraglide/messages';

	// The core decides what plays and how (sources, resume, tracks, what comes next) and drives
	// the video element through an ElementPlayer; this draws the controls and turns the
	// viewer's choices into the core's events.
	let { view }: { view: PlayerView } = $props();

	let video = $state<HTMLVideoElement>();
	let wrapper = $state<HTMLDivElement>();
	let seekBar = $state<HTMLDivElement>();
	let player = $state<ElementPlayer>();

	let playing = $state(false);
	let hasPlayed = $state(false); // suppress the pause indicator before autoplay starts
	let buffering = $state(false); // media is stalled waiting for data
	let currentTime = $state(0);
	let elementDuration = $state(0);
	// the element's own once it knows, else the payload's
	const duration = $derived(elementDuration || player?.durationSeconds || 0);
	let buffered = $state<{ start: number; end: number }[]>([]);
	let volume = $state(Number(localStorage.getItem('cv.volume') ?? 1));
	let muted = $state(false);
	let fullscreen = $state(false);
	let pipActive = $state(false);
	const pipSupported = typeof document !== 'undefined' && document.pictureInPictureEnabled === true;
	let controlsVisible = $state(true);

	let hideTimer: ReturnType<typeof setTimeout>;

	// a couch follower watches the host's timeline: no seeking, no switching what plays
	const linear = $derived(view.linear);

	function chooseSubtitle(id: string | null) {
		void core.send({ type: 'subtitlesChosen', content: id ? { id } : {} });
	}

	function cycleSubtitle() {
		const subtitles = view.subtitles;
		if (subtitles.length === 0) return;
		const index = subtitles.findIndex((s) => s.id === view.subtitleSelected);
		chooseSubtitle(index + 1 >= subtitles.length ? null : subtitles[index + 1].id);
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

	// episode switcher: the season dropdown follows the playing season until the user picks one
	const currentSeasonNumber = $derived(
		view.seasons.find((s) => s.episodes.some((e) => e.current))?.number ??
			view.seasons[0]?.number ??
			null
	);
	let seasonValue = $state('');
	const activeSeason = $derived(seasonValue ? Number(seasonValue) : currentSeasonNumber);
	const seasonEpisodes = $derived(
		view.seasons.find((s) => s.number === activeSeason)?.episodes ?? []
	);

	function openEpisode(id: string, current: boolean) {
		if (!current) goto(`/watch/episode/${id}`);
	}

	const nextUp = $derived(view.nextUp && view.nextUp.countdownSeconds > 0 ? view.nextUp : null);

	function qualityLabel(option: QualityOption): string {
		switch (option.kind) {
			case QualityKind.Original:
				return m.player_quality_original();
			case QualityKind.Auto:
				return m.player_quality_auto();
			case QualityKind.Rendition:
				return `${option.height}p`;
		}
	}

	function poke() {
		controlsVisible = true;
		clearTimeout(hideTimer);
		hideTimer = setTimeout(
			() => {
				// a remote has no pointer to keep an open menu alive, so on a TV the
				// controls wait while one (episodes, subtitles...) is up
				const menuOpen = isTV && !!wrapper?.querySelector('[role="dialog"],[role="listbox"]');
				if (playing && !menuOpen) controlsVisible = false;
			},
			isTV ? 5000 : 3000
		);
	}

	function togglePlay() {
		if (!video) return;
		// a follower's pause is their own: resuming catches up with the host
		if (couch.isFollower) {
			couch.pauseLocally(!video.paused);
			return;
		}
		if (video.paused) video.play().catch(() => {});
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
		if (!video || linear) return;
		video.currentTime = Math.min(Math.max(0, video.currentTime + seconds), duration);
		showSkip(seconds);
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

	function onProgress() {
		if (!video) return;
		const ranges = [];
		for (let i = 0; i < video.buffered.length; i++) {
			ranges.push({ start: video.buffered.start(i), end: video.buffered.end(i) });
		}
		buffered = ranges;
	}

	/**
	 * The end of something with nothing after it goes back to its title page. Read from the core
	 * itself: by now this player may be gone, the core having moved on to the next episode.
	 */
	function ended(ending: PlayerView) {
		if (ending.linear) return;
		// finishing something is the likeliest moment for a new badge
		if (features.rankingsEnabled) rank.check(true);
		const now = core.view<PlayerView>(PLAYER)?.target;
		if (sameTarget(now, ending.target)) goto(`/title/${ending.titleSlug}`);
	}

	function seekTo(event: PointerEvent, track: HTMLElement) {
		if (linear || !video) return;
		const rect = track.getBoundingClientRect();
		const ratio = Math.min(1, Math.max(0, (event.clientX - rect.left) / rect.width));
		video.currentTime = ratio * duration;
	}

	let scrubbing = $state(false);

	// banner accent for the player, from the title backdrop's server-extracted colour
	const accentStyle = $derived(view.backdrop?.accent ? accentVars(view.backdrop.accent) : '');
	let hoverRatio = $state<number | null>(null);
	const hoverTime = $derived(hoverRatio !== null ? hoverRatio * duration : 0);
	// bucket to 5s so the preview reuses cached frames while scrubbing
	const previewSrc = $derived(
		hoverRatio !== null && view.frameUrl
			? `${view.frameUrl}?t=${Math.floor(hoverTime / 5) * 5}`
			: ''
	);

	function onSeekHover(event: PointerEvent, track: HTMLElement) {
		const rect = track.getBoundingClientRect();
		hoverRatio = Math.min(1, Math.max(0, (event.clientX - rect.left) / rect.width));
	}

	// Remote control. Media keys work at any time. With the controls hidden or the seek
	// bar focused, OK plays/pauses and left/right skip; any other key brings the controls
	// up on the seek bar. Once a control button has focus, the arrows belong to the TV's
	// spatial navigation and OK clicks the button. Back is left to the TV shell.
	function onRemoteKey(e: KeyboardEvent) {
		const media = mediaKey(e);
		if (media) {
			e.preventDefault();
			onMediaKey(media);
			poke();
			return;
		}
		const active = document.activeElement;
		if (active !== seekBar && focusable(active) && wrapper?.contains(active)) {
			poke();
			return;
		}
		switch (e.key) {
			case 'Enter':
				togglePlay();
				break;
			case 'ArrowLeft':
				skip(-10);
				break;
			case 'ArrowRight':
				skip(10);
				break;
			case 'ArrowUp':
			case 'ArrowDown':
				// from the seek bar, up/down move on to the other controls
				if (active === seekBar && controlsVisible) {
					poke();
					return;
				}
				break;
			default:
				poke();
				return;
		}
		e.preventDefault();
		revealControls();
	}

	function onMediaKey(key: MediaKey) {
		if (!video) return;
		switch (key) {
			case 'play':
				if (video.paused) togglePlay();
				break;
			case 'pause':
			case 'stop':
				if (!video.paused) togglePlay();
				break;
			case 'toggle':
				togglePlay();
				break;
			case 'forward':
				skip(10);
				break;
			case 'rewind':
				skip(-10);
				break;
		}
	}

	async function revealControls() {
		poke();
		await tick();
		if (document.activeElement !== seekBar) seekBar?.focus({ preventScroll: true });
	}

	function onKeydown(e: KeyboardEvent) {
		if (e.target instanceof HTMLInputElement) return;
		if (isTV) {
			onRemoteKey(e);
			return;
		}
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

	// let the couch bar dodge the player controls while they are on screen
	$effect(() => {
		couch.playerControlsVisible = controlsVisible;
	});

	// the core saves progress itself; the ranks store still checks for badges on the web's
	// side, and throttles itself
	$effect(() => {
		if (!playing || linear || !features.rankingsEnabled) return;
		const timer = setInterval(() => rank.check(), 30_000);
		return () => clearInterval(timer);
	});

	onMount(() => {
		poke();
		couch.playerMounts++; // the on-screen player hosts the couch bar (so it survives fullscreen)
		const host = new ElementPlayer(video!, async (report) => {
			const ending = view;
			await core.send({ type: 'playerReported', content: report });
			if (report.ended) ended(ending);
		});
		player = host;
		const detach = core.attachPlayer(host);

		// PiP events are not in Svelte's element typings
		const onEnterPip = () => (pipActive = true);
		const onLeavePip = () => (pipActive = false);
		video?.addEventListener('enterpictureinpicture', onEnterPip);
		video?.addEventListener('leavepictureinpicture', onLeavePip);

		// warm the likely "back to title" destination so the Pi has it ready on click
		// (safe: the title read has no side effects, unlike playing)
		if (!linear && view.titleSlug) preloadData(`/title/${view.titleSlug}`).catch(() => {});

		return () => {
			video?.removeEventListener('enterpictureinpicture', onEnterPip);
			video?.removeEventListener('leavepictureinpicture', onLeavePip);
			detach();
			host.destroy();
			clearTimeout(hideTimer);
			couch.playerControlsVisible = false;
			couch.playerMounts--;
		};
	});
</script>

<!-- keydown on document rather than window so it runs before the TV shell, which skips
	keys handled (preventDefault) here -->
<svelte:document
	onkeydown={onKeydown}
	onfullscreenchange={() => (fullscreen = !!document.fullscreenElement)}
/>

<div
	bind:this={wrapper}
	class="relative h-dvh w-full overflow-hidden bg-black {controlsVisible ? '' : 'cursor-none'}"
	style={accentStyle}
	onpointermove={poke}
	role="presentation"
>
	<!-- the ElementPlayer adds the subtitle tracks the core loads -->
	<video
		bind:this={video}
		class="size-full object-contain"
		bind:volume
		bind:muted
		onplay={() => {
			playing = true;
			hasPlayed = true;
		}}
		onpause={() => {
			playing = false;
			buffering = false;
			poke();
			if (features.rankingsEnabled && !linear) rank.check();
		}}
		onwaiting={() => (buffering = true)}
		onstalled={() => (buffering = true)}
		onplaying={() => (buffering = false)}
		oncanplay={() => (buffering = false)}
		onseeking={() => {
			if (video) currentTime = video.currentTime;
		}}
		ontimeupdate={() => {
			if (video) currentTime = video.currentTime;
		}}
		onprogress={onProgress}
		ondurationchange={() => {
			const d = video?.duration ?? 0;
			elementDuration = Number.isFinite(d) ? d : 0;
		}}
		onended={() => (buffering = false)}
		onclick={togglePlay}
		ondblclick={toggleFullscreen}
	></video>

	{#if player?.cueHtml && !pipActive}
		<div class="subtitle-overlay" class:raised={controlsVisible} style={subCssVars}>
			<!-- cue markup comes from the browser's own VTT parser (getCueAsHTML) -->
			<!-- eslint-disable-next-line svelte/no-at-html-tags -->
			{@html player.cueHtml}
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

	<!-- same reasoning as the couch bar: the celebration has to be inside the
		fullscreen subtree, so it cannot ride the root layout's toaster -->
	{#if features.rankingsEnabled}
		<AchievementOverlay />
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
			{#if !linear}
				<Tooltip label={m.player_back_to_title()} side="bottom" portalTo={wrapper}>
					{#snippet trigger(props)}
						<button
							{...props}
							class="rounded-full p-2 text-white/80 transition-colors hover:bg-white/10 hover:text-white"
							onclick={() => goto(`/title/${view.titleSlug}`)}
							aria-label={m.common_back()}
						>
							<ArrowLeft class="size-5" />
						</button>
					{/snippet}
				</Tooltip>
			{/if}
			<div class="min-w-0">
				<p class="truncate font-semibold text-white">{view.title}</p>
				{#if view.subtitle}
					<p class="truncate text-xs text-white/60">{view.subtitle}</p>
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
				bind:this={seekBar}
				data-tv-autofocus
				class="group/seek relative mb-4 h-1 w-full rounded-full bg-white/20"
				class:cursor-pointer={!linear}
				class:pointer-events-none={linear}
				class:opacity-70={linear}
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
							bg-accent shadow transition-transform group-hover/seek:scale-100
							group-focus-visible/seek:scale-100"
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
				{#if !linear}
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

				<!-- a TV's remote drives the volume itself -->
				{#if !isTV}
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
				{/if}

				<span class="ml-2 text-xs text-white/70 tnum">
					{formatClock(currentTime)} / {formatClock(duration)}
				</span>

				<div class="flex-1"></div>

				<CouchButton canStart={!linear} portalTo={wrapper} />

				{#if view.shuffleAvailable && !linear}
					<Tooltip label={m.player_shuffle()} portalTo={wrapper}>
						{#snippet trigger(props)}
							<button
								{...props}
								class="player-btn {view.shuffle ? 'shuffle-on' : ''}"
								onclick={() => core.send({ type: 'shuffleToggled' })}
								aria-label={m.player_shuffle()}
								aria-pressed={view.shuffle}
							>
								<Shuffle class="size-5" />
							</button>
						{/snippet}
					</Tooltip>
				{/if}

				{#if view.seasons.length > 0 && !linear}
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
									{#if view.seasons.length > 1}
										<Select
											bind:value={seasonValue}
											label={m.catalog_season()}
											placeholder={currentSeasonNumber !== null
												? m.catalog_season_number({ number: currentSeasonNumber })
												: ''}
											items={view.seasons.map((season) => ({
												value: String(season.number),
												label: m.catalog_season_number({ number: season.number })
											}))}
											portalTo={wrapper}
										/>
									{/if}
								</div>
								<div class="space-y-1 overflow-y-auto p-2 scrollbar-none">
									{#each seasonEpisodes as ep (ep.id)}
										{@const current = ep.current}
										<button
											class="group flex w-full gap-3 rounded-lg p-1.5 text-left transition-colors
												{current ? 'bg-surface' : 'hover:bg-surface focus-visible:bg-surface'}"
											onclick={() => openEpisode(ep.id, current)}
										>
											<div
												class="relative aspect-video w-28 shrink-0 overflow-hidden rounded-md border
													border-edge/60 bg-surface-2"
											>
												<Artwork
													src={ep.still?.url}
													name={ep.name || m.player_episode_number({ number: ep.number })}
												/>
												<div
													class="absolute inset-0 flex items-center justify-center bg-black/45 transition-opacity
														{current ? 'opacity-100' : 'opacity-0 group-hover:opacity-100 group-focus-visible:opacity-100'}"
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
													<span class="text-xs font-semibold text-faint tnum">E{ep.number}</span>
													{#if current}
														<span class="text-[10px] font-semibold text-accent-ink"
															>{m.player_now_playing()}</span
														>
													{/if}
												</div>
												<p
													class="mt-0.5 line-clamp-2 text-xs font-medium
														{current ? 'text-text' : 'text-muted'} group-hover:text-accent-ink"
												>
													{ep.name || m.player_episode_number({ number: ep.number })}
												</p>
											</div>
										</button>
									{/each}
								</div>
							</Popover.Content>
						</Popover.Portal>
					</Popover.Root>
				{/if}

				{#if view.qualities.length > 1}
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
								{#each view.qualities as option (option.key)}
									<button
										class="flex w-full items-center justify-between rounded-lg px-3 py-1.5 text-left text-xs
											{view.quality === option.key ? 'text-text' : 'text-muted'} hover:bg-surface"
										onclick={() =>
											core.send({ type: 'qualityChosen', content: { key: option.key } })}
									>
										{qualityLabel(option)}
										{#if view.quality === option.key}<Check class="size-3.5 text-accent-ink" />{/if}
									</button>
								{/each}
							</Popover.Content>
						</Popover.Portal>
					</Popover.Root>
				{/if}

				{#if view.audio.length > 1}
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
								{#each view.audio as track (track.id)}
									<button
										class="flex w-full items-center justify-between rounded-lg px-3 py-1.5 text-left text-xs
											{view.audioSelected === track.id ? 'text-text' : 'text-muted'} hover:bg-surface"
										onclick={() => core.send({ type: 'audioChosen', content: { id: track.id } })}
									>
										{track.label}
										{#if view.audioSelected === track.id}<Check
												class="size-3.5 text-accent-ink"
											/>{/if}
									</button>
								{/each}
							</Popover.Content>
						</Popover.Portal>
					</Popover.Root>
				{/if}

				{#if view.subtitles.length > 0}
					<Popover.Root>
						<Popover.Trigger
							class="player-btn {view.subtitleSelected ? 'text-accent-ink!' : ''}"
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
										{!view.subtitleSelected ? 'text-text' : 'text-muted'} hover:bg-surface"
									onclick={() => chooseSubtitle(null)}
								>
									{m.player_subtitle_off()}
									{#if !view.subtitleSelected}<Check class="size-3.5 text-accent-ink" />{/if}
								</button>
								{#each view.subtitles as sub (sub.id)}
									<button
										class="flex w-full items-center justify-between rounded-lg px-3 py-1.5 text-left text-xs
											{view.subtitleSelected === sub.id ? 'text-text' : 'text-muted'} hover:bg-surface"
										onclick={() => chooseSubtitle(sub.id)}
									>
										{sub.label}
										{#if view.subtitleSelected === sub.id}<Check
												class="size-3.5 text-accent-ink"
											/>{/if}
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

				<!-- a TV app is always full screen and has no picture-in-picture -->
				{#if pipSupported && !isTV}
					<Tooltip label={m.player_picture_in_picture()} portalTo={wrapper}>
						{#snippet trigger(props)}
							<button
								{...props}
								class="player-btn {pipActive ? 'text-accent-ink!' : ''}"
								onclick={togglePip}
								aria-label={m.player_picture_in_picture()}
							>
								<PictureInPicture2 class="size-4.5" />
							</button>
						{/snippet}
					</Tooltip>
				{/if}

				{#if !isTV}
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
				{/if}
			</div>
		</div>
	{/if}

	{#if nextUp}
		<div
			transition:fly={{ y: 24, duration: 250 }}
			class="absolute right-6 bottom-24 w-72 rounded-card border border-edge bg-surface-2/95
				p-4 shadow-2xl shadow-black/60 backdrop-blur"
			data-testid="up-next"
		>
			<p class="eyebrow mb-1 flex items-center gap-1.5">
				{#if nextUp.shuffled}<Shuffle class="size-3" />{/if}
				{m.player_up_next({ seconds: nextUp.countdownSeconds })}
			</p>
			<p class="truncate text-sm font-semibold">
				S{nextUp.season} E{nextUp.episode}
				{nextUp.name ? `· ${nextUp.name}` : ''}
			</p>
			<div class="mt-3 flex gap-2">
				<button
					class="h-8 flex-1 rounded-full bg-accent text-xs font-semibold text-white transition-colors hover:bg-accent-strong"
					onclick={() => core.send({ type: 'nextEpisodeRequested' })}
				>
					{m.player_play_now()}
				</button>
				<button
					class="h-8 rounded-full px-3 text-xs font-semibold text-muted transition-colors hover:bg-surface hover:text-text"
					onclick={() => core.send({ type: 'nextEpisodeCancelled' })}
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
