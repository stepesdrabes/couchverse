<script lang="ts">
	import { CalendarDays, Loader, LogIn, RotateCw, SkipBack } from 'lucide-svelte';
	import { onMount, untrack } from 'svelte';
	import { core } from '$lib/core';
	import { problemMessage } from '$lib/api/problem';
	import Button from '$lib/components/ui/Button.svelte';
	import Tooltip from '$lib/components/ui/Tooltip.svelte';
	import { session } from '$lib/features/auth/session.svelte';
	import VideoPlayer from '$lib/features/playback/components/VideoPlayer.svelte';
	import { watchPath, type LivePlayback } from '$lib/features/playback/player';
	import { CouchTvStatus, type CouchTvView } from '$lib/generated/core';
	import { currentLang } from '$lib/i18n/locale.svelte';
	import * as m from '$lib/paraglide/messages';
	import {
		chooseAudio,
		chooseSubtitles,
		clock,
		COUCH_TV,
		isProgram,
		livePlayerView,
		pictureDate,
		progressAt,
		tuneIn,
		tuneOut
	} from '../channel';
	import ChannelScreen from '../components/ChannelScreen.svelte';
	import LiveOverlay from '../components/LiveOverlay.svelte';
	import LiveReactions from '../components/LiveReactions.svelte';
	import PresenceBar from '../components/PresenceBar.svelte';

	// The live channel, full-bleed: the core tunes in (signed in, or as the guest the channel
	// cookie names), holds the picture on everyone's moment and says what is on by the picture's
	// clock; this page draws the player's live mode, the overlays, who is watching and their
	// reactions.
	$effect(() => core.watch(COUCH_TV));
	const view = $derived(core.view<CouchTvView>(COUCH_TV));

	// the core's view of an earlier visit (a failure, say) is not this one's until it tuned in
	let tuned = $state(false);

	// the channel plays while the page is up; leaving it, or the tab, stops it and leaves the room
	$effect(() => {
		untrack(retune);
		const close = () => void tuneOut();
		window.addEventListener('pagehide', close);
		return () => {
			window.removeEventListener('pagehide', close);
			close();
		};
	});

	function retune() {
		tuned = false;
		void tuneIn().finally(() => (tuned = true));
	}

	// the bug's clock and the progress line run on between the core's renders
	let nowMs = $state(core.nowMs());
	onMount(() => {
		const timer = setInterval(() => (nowMs = core.nowMs()), 1000);
		return () => clearInterval(timer);
	});

	const lang = $derived(currentLang());
	const at = $derived(view ? pictureDate(view, nowMs) : undefined);
	const signedIn = $derived(!!session.user);
	const live = $derived<LivePlayback>({
		progress: view?.now && at !== undefined ? progressAt(view.now, at) : undefined,
		back: signedIn ? '/' : undefined,
		chooseAudio: (id) => void chooseAudio(id),
		chooseSubtitles: (id) => void chooseSubtitles(id)
	});
	const playing = $derived(
		view?.status === CouchTvStatus.Tuning || view?.status === CouchTvStatus.OnAir
	);
	const nextUp = $derived(view?.next);
	const playerView = $derived(view && livePlayerView(view));
	// "Watch from the start" in the on-demand player: the core offers it to members only
	const fromStart = $derived(isProgram(view?.now) ? view.now.play : undefined);
</script>

<svelte:head>
	<title>{m.couchtv_page_title()}</title>
</svelte:head>

{#if view && playerView && tuned && playing}
	<VideoPlayer view={playerView} {live}>
		{#snippet overlay({ controlsVisible })}
			<LiveOverlay {view} {at} active={controlsVisible} />
			<LiveReactions {view} active={controlsVisible} />
			{#if view.status === CouchTvStatus.Tuning}
				<div
					class="pointer-events-none absolute inset-0 z-10 flex flex-col items-center justify-center gap-3"
				>
					<Loader class="size-8 animate-spin text-white/80" />
					<p class="text-sm font-medium text-white/80">{m.couchtv_tuning()}</p>
				</div>
			{/if}
		{/snippet}
		{#snippet controls({ portalTo })}
			<PresenceBar {view} {portalTo} />
		{/snippet}
		{#snippet actions({ portalTo })}
			{#if fromStart}
				<Tooltip label={m.couchtv_watch_from_start()} {portalTo}>
					{#snippet trigger(props)}
						<a
							{...props}
							href={watchPath(fromStart)}
							data-sveltekit-preload-data="tap"
							class="player-btn"
							aria-label={m.couchtv_watch_from_start()}
						>
							<SkipBack class="size-5" />
						</a>
					{/snippet}
				</Tooltip>
			{/if}
			{#if signedIn}
				<Tooltip label={m.couchtv_guide()} {portalTo}>
					{#snippet trigger(props)}
						<a {...props} href="/tv/guide" class="player-btn" aria-label={m.couchtv_guide()}>
							<CalendarDays class="size-5" />
						</a>
					{/snippet}
				</Tooltip>
			{/if}
		{/snippet}
	</VideoPlayer>
{:else if view && tuned && view.status === CouchTvStatus.OffAir}
	<ChannelScreen
		title={m.couchtv_off_air_title()}
		message={nextUp
			? m.couchtv_back_on_air({ time: clock(nextUp.startsAtMs, lang), title: nextUp.title ?? '' })
			: m.couchtv_nothing_planned()}
	>
		{#if signedIn}
			<a
				href="/tv/guide"
				class="inline-flex h-10 items-center gap-2 rounded-full border border-edge bg-surface/60 px-5
					text-sm font-semibold transition-colors hover:border-faint hover:bg-surface-2"
				data-tv-autofocus
			>
				<CalendarDays class="size-4" />
				{m.couchtv_guide()}
			</a>
		{/if}
	</ChannelScreen>
{:else if view && tuned && view.status === CouchTvStatus.Failed}
	{#if view.problem?.code === 'no_channel_guest'}
		<!-- signed out without a working channel link -->
		<ChannelScreen title={m.couchtv_sign_in_title()} message={m.problem_no_channel_guest()}>
			<a
				href="/login?next=%2Ftv"
				class="inline-flex h-10 items-center gap-2 rounded-full bg-accent px-5 text-sm font-semibold
					text-[var(--color-on-accent)] transition-colors hover:bg-accent-strong"
				data-tv-autofocus
			>
				<LogIn class="size-4" />
				{m.login_submit()}
			</a>
		</ChannelScreen>
	{:else}
		<ChannelScreen
			title={m.couchtv_unavailable_title()}
			message={problemMessage(view.problem, m.couchtv_failed())}
		>
			<Button onclick={retune} data-tv-autofocus>
				<RotateCw class="size-4" />
				{m.common_retry()}
			</Button>
		</ChannelScreen>
	{/if}
{:else}
	<div class="flex min-h-dvh items-center justify-center bg-black">
		<Loader class="size-7 animate-spin text-accent-ink" />
	</div>
{/if}
