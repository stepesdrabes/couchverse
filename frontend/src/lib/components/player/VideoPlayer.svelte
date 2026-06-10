<script lang="ts">
	import { goto } from '$app/navigation';
	import { Popover } from 'bits-ui';
	import type Hls from 'hls.js';
	import {
		ArrowLeft,
		Captions,
		Check,
		ListVideo,
		Maximize,
		Minimize,
		Pause,
		Play,
		RotateCcw,
		RotateCw,
		SlidersHorizontal,
		Type,
		Volume2,
		VolumeX
	} from 'lucide-svelte';
	import { onMount } from 'svelte';
	import { fade, fly } from 'svelte/transition';
	import { musicPlayer } from '$lib/features/music/player.svelte';
	import type { PlaybackInfo } from '$lib/features/playback/api';
	import { beaconProgress, jitKeepalive, reportProgress } from '$lib/features/playback/api';
	import {
		preferences,
		SUBTITLE_FONTS,
		type SubtitleSettings
	} from '$lib/features/preferences/preferences.svelte';
	import { formatClock } from '$lib/utils/format';

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
	let currentTime = $state(0);
	let duration = $state(info.durationSeconds || 0);
	let buffered = $state<{ start: number; end: number }[]>([]);
	let volume = $state(Number(localStorage.getItem('cv.volume') ?? 1));
	let muted = $state(false);
	let fullscreen = $state(false);
	let controlsVisible = $state(true);
	let nextCountdown = $state<number | null>(null);

	let hideTimer: ReturnType<typeof setTimeout>;
	let lastReported = 0;

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
		const groups = new Map<number, { episodeId: string; episodeNumber: number; name: string }[]>();
		for (const ep of info.episodes ?? []) {
			const list = groups.get(ep.seasonNumber) ?? [];
			list.push(ep);
			groups.set(ep.seasonNumber, list);
		}
		return [...groups.entries()].sort((a, b) => a[0] - b[0]);
	});

	function openEpisode(episodeId: string) {
		if (episodeId === info.currentEpisodeId) return;
		report();
		goto(`/watch/episode/${episodeId}`, { invalidateAll: true });
	}

	const remaining = $derived(duration - currentTime);
	const progressBody = () => ({
		...(titleId ? { titleId } : { episodeId: episodeId! }),
		positionSeconds: Math.floor(currentTime),
		durationSeconds: Math.floor(duration)
	});

	function report() {
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
		if (video.paused) video.play();
		else video.pause();
	}

	function skip(seconds: number) {
		if (video) video.currentTime = Math.min(Math.max(0, video.currentTime + seconds), duration);
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

	function onTimeUpdate() {
		if (!video) return;
		currentTime = video.currentTime;
		if (currentTime - lastReported >= 10) report();

		// auto-next countdown in the last 20 seconds
		if (info.nextEpisode && remaining <= 20 && remaining > 0 && nextCountdown === null) {
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
		if (!info.nextEpisode) return;
		report();
		goto(`/watch/episode/${info.nextEpisode.episodeId}`, { invalidateAll: true });
	}

	function onEnded() {
		cueHtml = '';
		report();
		if (info.nextEpisode) goNextEpisode();
		else goto(`/title/${info.display.titleSlug}`);
	}

	function seekTo(event: PointerEvent, track: HTMLElement) {
		const rect = track.getBoundingClientRect();
		const ratio = Math.min(1, Math.max(0, (event.clientX - rect.left) / rect.width));
		if (video) video.currentTime = ratio * duration;
	}

	let scrubbing = $state(false);

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

	// HLS: hls.js where needed, native playback on Safari
	let hls: Hls | null = null;
	let qualityLevels = $state<{ index: number; height: number }[]>([]);
	let currentLevel = $state(-1); // -1 = auto

	async function setupHls() {
		if (!video || !info.streamUrl) return;
		if (video.canPlayType('application/vnd.apple.mpegurl')) {
			video.src = info.streamUrl;
			return;
		}
		const { default: HlsCtor } = await import('hls.js');
		if (!HlsCtor.isSupported()) return;
		hls = new HlsCtor();
		hls.loadSource(info.streamUrl);
		hls.attachMedia(video);
		hls.on(HlsCtor.Events.MANIFEST_PARSED, () => {
			qualityLevels = (hls?.levels ?? []).map((level, index) => ({
				index,
				height: level.height
			}));
		});
	}

	function selectLevel(index: number) {
		currentLevel = index;
		if (hls) hls.currentLevel = index;
	}

	onMount(() => {
		poke();
		musicPlayer.pause(); // never play video and music together
		if (info.mode === 'hls') setupHls();

		// JIT sessions are reaped server-side without this heartbeat
		let keepaliveTimer: ReturnType<typeof setInterval> | undefined;
		if (jitSessionId) {
			keepaliveTimer = setInterval(() => jitKeepalive(jitSessionId!).catch(() => {}), 15000);
		}

		const onVisibility = () => {
			if (document.visibilityState === 'hidden' && currentTime > 5) {
				beaconProgress(progressBody());
			}
		};
		document.addEventListener('visibilitychange', onVisibility);
		return () => {
			document.removeEventListener('visibilitychange', onVisibility);
			detachCueListener();
			clearTimeout(hideTimer);
			clearInterval(keepaliveTimer);
			hls?.destroy();
			if (currentTime > 5) beaconProgress(progressBody());
		};
	});
</script>

<svelte:window onkeydown={onKeydown} />
<svelte:document onfullscreenchange={() => (fullscreen = !!document.fullscreenElement)} />

<div
	bind:this={wrapper}
	class="relative h-dvh w-full overflow-hidden bg-black {controlsVisible ? '' : 'cursor-none'}"
	onpointermove={poke}
	role="presentation"
>
	<!-- svelte-ignore a11y_media_has_caption -->
	<video
		bind:this={video}
		src={info.mode === 'direct' ? info.streamUrl : undefined}
		autoplay
		class="size-full object-contain"
		bind:volume
		bind:muted
		onplay={() => (playing = true)}
		onpause={() => {
			playing = false;
			report();
			poke();
		}}
		ontimeupdate={onTimeUpdate}
		onprogress={onProgress}
		ondurationchange={() => (duration = video?.duration || info.durationSeconds)}
		onloadedmetadata={() => {
			if (video && info.resumePosition > 5) video.currentTime = info.resumePosition;
			restorePreferredSubtitle();
		}}
		onended={onEnded}
		onclick={togglePlay}
		ondblclick={toggleFullscreen}
	>
		{#each info.subtitles as sub (sub.id)}
			<track kind="subtitles" src={sub.url} srclang={sub.lang} label={sub.label} />
		{/each}
	</video>

	{#if cueHtml}
		<div class="subtitle-overlay" class:raised={controlsVisible} style={subCssVars}>
			{@html cueHtml}
		</div>
	{/if}

	{#if controlsVisible}
		<!-- top bar -->
		<div
			transition:fade={{ duration: 200 }}
			class="absolute inset-x-0 top-0 flex items-center gap-4 bg-gradient-to-b from-black/80
				to-transparent p-5 pb-12"
		>
			<button
				class="rounded-full p-2 text-white/80 transition-colors hover:bg-white/10 hover:text-white"
				onclick={() => goto(`/title/${info.display.titleSlug}`)}
				aria-label="Back"
			>
				<ArrowLeft class="size-5" />
			</button>
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
			class="absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/90 to-transparent px-5 pt-16 pb-5"
		>
			<!-- seek bar -->
			<div
				class="group/seek relative mb-4 h-1 w-full cursor-pointer rounded-full bg-white/20"
				onpointerdown={(e) => {
					scrubbing = true;
					seekTo(e, e.currentTarget);
					e.currentTarget.setPointerCapture(e.pointerId);
				}}
				onpointermove={(e) => scrubbing && seekTo(e, e.currentTarget)}
				onpointerup={() => (scrubbing = false)}
				role="slider"
				aria-label="Seek"
				aria-valuemin={0}
				aria-valuemax={duration}
				aria-valuenow={currentTime}
				tabindex="0"
			>
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
				<button class="player-btn" onclick={togglePlay} aria-label="Play/Pause">
					{#if playing}
						<Pause class="size-5 fill-current" />
					{:else}
						<Play class="size-5 fill-current" />
					{/if}
				</button>
				<button class="player-btn" onclick={() => skip(-10)} aria-label="Back 10 seconds">
					<RotateCcw class="size-4.5" />
				</button>
				<button class="player-btn" onclick={() => skip(10)} aria-label="Forward 10 seconds">
					<RotateCw class="size-4.5" />
				</button>

				<div class="group/vol flex items-center gap-2">
					<button class="player-btn" onclick={() => (muted = !muted)} aria-label="Mute">
						{#if muted || volume === 0}
							<VolumeX class="size-4.5" />
						{:else}
							<Volume2 class="size-4.5" />
						{/if}
					</button>
					<input
						type="range"
						min="0"
						max="1"
						step="0.05"
						value={muted ? 0 : volume}
						oninput={(e) => setVolume(Number(e.currentTarget.value))}
						class="volume-slider w-0 opacity-0 transition-all duration-200
							group-hover/vol:w-20 group-hover/vol:opacity-100"
						aria-label="Volume"
					/>
				</div>

				<span class="ml-2 text-xs text-white/70 tnum">
					{formatClock(currentTime)} / {formatClock(duration)}
				</span>

				<div class="flex-1"></div>

				{#if episodesBySeason.length > 0}
					<Popover.Root>
						<Popover.Trigger class="player-btn" aria-label="Episodes">
							<ListVideo class="size-5" />
						</Popover.Trigger>
						<Popover.Portal>
							<Popover.Content
								side="top"
								sideOffset={10}
								class="z-50 max-h-[60vh] w-72 animate-pop-in overflow-y-auto rounded-card border border-edge bg-surface-2/95 p-1 shadow-xl backdrop-blur scrollbar-none"
							>
								{#each episodesBySeason as [seasonNumber, eps] (seasonNumber)}
									<p
										class="px-3 pt-2 pb-1 text-[10px] font-semibold tracking-widest text-faint uppercase"
									>
										Season {seasonNumber}
									</p>
									{#each eps as ep (ep.episodeId)}
										<button
											class="flex w-full items-center gap-2 rounded-lg px-3 py-1.5 text-left text-xs
												{ep.episodeId === info.currentEpisodeId ? 'text-text' : 'text-muted'} hover:bg-surface"
											onclick={() => openEpisode(ep.episodeId)}
										>
											<span class="w-6 shrink-0 text-faint tnum">E{ep.episodeNumber}</span>
											<span class="flex-1 truncate">{ep.name || `Episode ${ep.episodeNumber}`}</span
											>
											{#if ep.episodeId === info.currentEpisodeId}
												<Play class="size-3 shrink-0 fill-current text-accent" />
											{/if}
										</button>
									{/each}
								{/each}
							</Popover.Content>
						</Popover.Portal>
					</Popover.Root>
				{/if}

				{#if qualityLevels.length > 1}
					<Popover.Root>
						<Popover.Trigger class="player-btn" aria-label="Quality">
							<SlidersHorizontal class="size-4.5" />
						</Popover.Trigger>
						<Popover.Portal>
							<Popover.Content
								side="top"
								sideOffset={10}
								class="z-50 w-40 animate-pop-in rounded-card border border-edge bg-surface-2/95 p-1 shadow-xl backdrop-blur"
							>
								<p
									class="px-3 py-1.5 text-[10px] font-semibold tracking-widest text-faint uppercase"
								>
									Quality
								</p>
								<button
									class="flex w-full items-center justify-between rounded-lg px-3 py-1.5 text-left text-xs
										{currentLevel === -1 ? 'text-text' : 'text-muted'} hover:bg-surface"
									onclick={() => selectLevel(-1)}
								>
									Auto
									{#if currentLevel === -1}<Check class="size-3.5 text-accent" />{/if}
								</button>
								{#each qualityLevels as level (level.index)}
									<button
										class="flex w-full items-center justify-between rounded-lg px-3 py-1.5 text-left text-xs
											{currentLevel === level.index ? 'text-text' : 'text-muted'} hover:bg-surface"
										onclick={() => selectLevel(level.index)}
									>
										{level.height}p
										{#if currentLevel === level.index}<Check class="size-3.5 text-accent" />{/if}
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
							aria-label="Subtitles"
						>
							<Captions class="size-5" />
						</Popover.Trigger>
						<Popover.Portal>
							<Popover.Content
								side="top"
								sideOffset={10}
								class="z-50 max-h-[70vh] w-64 animate-pop-in overflow-y-auto rounded-card border border-edge bg-surface-2/95 p-1 shadow-xl backdrop-blur scrollbar-none"
							>
								<p
									class="px-3 py-1.5 text-[10px] font-semibold tracking-widest text-faint uppercase"
								>
									Subtitles
								</p>
								<button
									class="flex w-full items-center justify-between rounded-lg px-3 py-1.5 text-left text-xs
										{activeSub === null ? 'text-text' : 'text-muted'} hover:bg-surface"
									onclick={() => selectSubtitle(null)}
								>
									Off
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
										<Type class="size-3" /> Appearance
									</p>

									<div class="space-y-2.5 px-3 py-1.5">
										<label class="block">
											<span class="mb-1 flex justify-between text-[11px] text-muted">
												Size <span class="tnum">{subStyle.fontSizePct}%</span>
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
												aria-label="Subtitle size"
											/>
										</label>

										<label class="block">
											<span class="mb-1 flex justify-between text-[11px] text-muted">
												Background <span class="tnum">{subStyle.backgroundOpacity}%</span>
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
												aria-label="Subtitle background opacity"
											/>
										</label>

										<div class="flex items-center justify-between">
											<span class="text-[11px] text-muted">Font</span>
											<div class="flex gap-1">
												{#each ['sans', 'serif', 'rounded', 'mono'] as const as font (font)}
													<button
														class="rounded px-2 py-1 text-[10px] capitalize transition-colors
															{subStyle.fontFamily === font ? 'bg-accent text-white' : 'bg-surface text-muted hover:text-text'}"
														onclick={() => updateSubStyle('fontFamily', font)}
													>
														{font}
													</button>
												{/each}
											</div>
										</div>

										<div class="flex items-center justify-between">
											<span class="text-[11px] text-muted">Colour</span>
											<div class="flex items-center gap-1.5">
												{#each ['#ffffff', '#ffe600', '#7cf0a0', '#69b4ff'] as swatch (swatch)}
													<button
														class="size-5 rounded-full border-2 transition-transform hover:scale-110
															{subStyle.color.toLowerCase() === swatch ? 'border-text' : 'border-transparent'}"
														style="background: {swatch}"
														onclick={() => updateSubStyle('color', swatch)}
														aria-label="Subtitle colour {swatch}"
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

				<button class="player-btn" onclick={toggleFullscreen} aria-label="Fullscreen">
					{#if fullscreen}
						<Minimize class="size-4.5" />
					{:else}
						<Maximize class="size-4.5" />
					{/if}
				</button>
			</div>
		</div>
	{/if}

	{#if info.nextEpisode && nextCountdown !== null && nextCountdown > 0}
		<div
			transition:fly={{ y: 24, duration: 250 }}
			class="absolute right-6 bottom-24 w-72 rounded-card border border-edge bg-surface-2/95
				p-4 shadow-2xl shadow-black/60 backdrop-blur"
		>
			<p class="eyebrow mb-1">Up next · {nextCountdown}s</p>
			<p class="truncate text-sm font-semibold">
				S{info.nextEpisode.seasonNumber} E{info.nextEpisode.episodeNumber}
				{info.nextEpisode.name ? `· ${info.nextEpisode.name}` : ''}
			</p>
			<div class="mt-3 flex gap-2">
				<button
					class="h-8 flex-1 rounded-full bg-accent text-xs font-semibold text-white transition-colors hover:bg-accent-strong"
					onclick={goNextEpisode}
				>
					Play now
				</button>
				<button
					class="h-8 rounded-full px-3 text-xs font-semibold text-muted transition-colors hover:bg-surface hover:text-text"
					onclick={() => (nextCountdown = null)}
				>
					Cancel
				</button>
			</div>
		</div>
	{/if}
</div>

<style lang="scss">
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
