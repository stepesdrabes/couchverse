<script lang="ts">
	import { onMount } from 'svelte';
	import { LogOut, Pause, Play, RotateCcw, RotateCw, SkipBack, SkipForward } from 'lucide-svelte';
	import { core } from '$lib/core';
	import { RemoteAction } from '$lib/generated/core';
	import { couch } from '$lib/features/couch/couch.svelte';
	import { formatClock } from '$lib/utils/format';
	import * as m from '$lib/paraglide/messages';

	// the host's account on this device: the host's player plays elsewhere, this steers it
	const view = $derived(couch.view);
	const playing = $derived(!!view?.playing);

	// the host's position runs on from its last report while it plays
	let now = $state(core.nowMs());
	onMount(() => {
		const tick = setInterval(() => (now = core.nowMs()), 500);
		return () => clearInterval(tick);
	});
	const position = $derived(
		view ? view.positionSeconds + (playing ? Math.max(0, now - view.positionAtMs) / 1000 : 0) : 0
	);

	const status = $derived.by(() => {
		if (view?.hostAway) return m.couch_host_away();
		if (view && !view.media) {
			return couch.host
				? m.couch_host_choosing({ name: couch.host.displayName })
				: m.couch_host_choosing_generic();
		}
		return '';
	});

	function seek(by: number) {
		couch.steer({ action: RemoteAction.Seek, positionSeconds: Math.max(0, position + by) });
	}

	const round =
		'flex size-14 items-center justify-center rounded-full border border-edge bg-surface-2/80 text-text transition-colors hover:bg-surface-2 hover:text-accent-ink';
</script>

<div class="flex h-dvh w-full items-center justify-center bg-black px-6 text-center">
	<div class="flex max-w-sm flex-col items-center gap-6">
		<div class="space-y-1">
			<h1 class="text-2xl font-bold text-white">{m.couch_remote_title()}</h1>
			{#if status}
				<p class="text-sm text-muted">{status}</p>
			{/if}
		</div>

		<p class="text-5xl font-semibold text-white tnum">{formatClock(position)}</p>

		<div class="flex items-center gap-5">
			<button class={round} onclick={() => seek(-10)} aria-label={m.player_back_10_seconds()}>
				<RotateCcw class="size-6" />
			</button>
			<button
				class="flex size-20 items-center justify-center rounded-full bg-accent text-[var(--color-on-accent)]
					shadow-lg transition-colors hover:bg-accent-strong"
				onclick={() => couch.steer({ action: playing ? RemoteAction.Pause : RemoteAction.Play })}
				aria-label={playing ? m.common_pause() : m.common_play()}
				data-tv-autofocus
			>
				{#if playing}
					<Pause class="size-8 fill-current" />
				{:else}
					<Play class="size-8 fill-current" />
				{/if}
			</button>
			<button class={round} onclick={() => seek(10)} aria-label={m.player_forward_10_seconds()}>
				<RotateCw class="size-6" />
			</button>
		</div>

		<div class="flex items-center gap-5">
			<button
				class={round}
				onclick={() => couch.steer({ action: RemoteAction.Previous })}
				aria-label={m.couch_remote_previous()}
			>
				<SkipBack class="size-6" />
			</button>
			<button
				class={round}
				onclick={() => couch.steer({ action: RemoteAction.Next })}
				aria-label={m.couch_remote_next()}
			>
				<SkipForward class="size-6" />
			</button>
		</div>

		<button
			class="inline-flex h-10 items-center gap-2 rounded-full border border-edge px-5 text-sm font-semibold
				text-muted transition-colors hover:bg-surface hover:text-text"
			onclick={() => couch.leave()}
		>
			<LogOut class="size-4" />
			{m.couch_leave()}
		</button>
	</div>
</div>
