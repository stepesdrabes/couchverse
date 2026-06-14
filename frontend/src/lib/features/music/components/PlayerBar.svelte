<script lang="ts">
	import {
		ListMusic,
		Pause,
		Play,
		Repeat,
		Repeat1,
		Shuffle,
		SkipBack,
		SkipForward,
		Volume2,
		VolumeX
	} from 'lucide-svelte';
	import { fly } from 'svelte/transition';
	import { artworkUrl } from '$lib/features/catalog/api';
	import { musicPlayer as player } from '$lib/features/music/player.svelte';
	import QueuePanel from './QueuePanel.svelte';
	import { formatClock } from '$lib/utils/format';
	import * as m from '$lib/paraglide/messages';

	function seekFromPointer(e: PointerEvent, el: HTMLElement) {
		const rect = el.getBoundingClientRect();
		const ratio = Math.min(1, Math.max(0, (e.clientX - rect.left) / rect.width));
		player.seek(ratio * player.duration);
	}
</script>

{#if player.current}
	<div
		transition:fly={{ y: 80, duration: 300 }}
		class="fixed inset-x-0 bottom-0 z-40 border-t border-edge bg-surface/95 backdrop-blur-md"
		style="view-transition-name: player-bar"
	>
		<div class="mx-auto flex h-20 max-w-[1700px] items-center gap-4 px-4 sm:px-6">
			<!-- now playing -->
			<div class="flex w-56 min-w-0 items-center gap-3">
				<a
					href="/music/albums/{player.current.albumId}"
					class="block size-12 shrink-0 overflow-hidden rounded-lg border border-edge/60 bg-surface-2"
				>
					{#if player.current.coverId}
						<img src={artworkUrl(player.current.coverId)} alt="" class="size-full object-cover" />
					{:else}
						<span
							class="flex size-full items-center justify-center text-xs font-bold text-accent/70"
						>
							{player.current.albumName[0]}
						</span>
					{/if}
				</a>
				<div class="min-w-0">
					<p class="truncate text-sm font-semibold">{player.current.name}</p>
					<a
						href="/music/artists/{player.current.artistId}"
						class="truncate text-xs text-faint hover:text-muted"
						tabindex="-1"
					>
						{player.current.trackArtist ?? player.current.artistName}
					</a>
				</div>
			</div>

			<!-- controls + seek -->
			<div class="flex min-w-0 flex-1 flex-col items-center gap-1.5">
				<div class="flex items-center gap-2">
					<button
						class="music-btn {player.shuffle ? 'text-accent!' : ''}"
						onclick={() => player.toggleShuffle()}
						aria-label={m.music_shuffle()}
					>
						<Shuffle class="size-4" />
					</button>
					<button class="music-btn" onclick={() => player.prev()} aria-label={m.music_previous()}>
						<SkipBack class="size-4.5 fill-current" />
					</button>
					<button
						class="flex size-9 items-center justify-center rounded-full bg-text text-bg transition-transform hover:scale-105"
						onclick={() => player.toggle()}
						aria-label={m.music_play_pause()}
					>
						{#if player.playing}
							<Pause class="size-4.5 fill-current" />
						{:else}
							<Play class="ml-0.5 size-4.5 fill-current" />
						{/if}
					</button>
					<button class="music-btn" onclick={() => player.next()} aria-label={m.music_next()}>
						<SkipForward class="size-4.5 fill-current" />
					</button>
					<button
						class="music-btn {player.repeat !== 'off' ? 'text-accent!' : ''}"
						onclick={() => player.cycleRepeat()}
						aria-label={m.music_repeat()}
					>
						{#if player.repeat === 'one'}
							<Repeat1 class="size-4" />
						{:else}
							<Repeat class="size-4" />
						{/if}
					</button>
				</div>
				<div class="hidden w-full max-w-xl items-center gap-2 sm:flex">
					<span class="w-10 text-right text-[10px] text-faint tnum">
						{formatClock(player.currentTime)}
					</span>
					<div
						class="group/seek relative h-1 flex-1 cursor-pointer rounded-full bg-surface-2"
						onpointerdown={(e) => seekFromPointer(e, e.currentTarget)}
						role="slider"
						aria-label={m.music_seek()}
						aria-valuemin={0}
						aria-valuemax={player.duration}
						aria-valuenow={player.currentTime}
						tabindex="0"
					>
						<div
							class="relative h-full rounded-full bg-accent"
							style="width: {player.duration ? (player.currentTime / player.duration) * 100 : 0}%"
						>
							<span
								class="absolute top-1/2 -right-1 size-2.5 -translate-y-1/2 scale-0 rounded-full bg-text
									transition-transform group-hover/seek:scale-100"
							></span>
						</div>
					</div>
					<span class="w-10 text-[10px] text-faint tnum">{formatClock(player.duration)}</span>
				</div>
			</div>

			<!-- right cluster -->
			<div class="flex w-56 items-center justify-end gap-1">
				<button
					class="music-btn {player.queueOpen ? 'text-accent!' : ''}"
					onclick={() => (player.queueOpen = !player.queueOpen)}
					aria-label={m.music_queue()}
				>
					<ListMusic class="size-4.5" />
				</button>
				<div class="hidden items-center gap-2 md:flex">
					<button
						class="music-btn"
						onclick={() => player.setVolume(player.muted ? 0.8 : 0)}
						aria-label={m.music_mute()}
					>
						{#if player.muted || player.volume === 0}
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
						value={player.volume}
						oninput={(e) => player.setVolume(Number(e.currentTarget.value))}
						class="w-20 accent-(--color-accent)"
						aria-label={m.music_volume()}
					/>
				</div>
			</div>
		</div>
	</div>

	<QueuePanel />
{/if}

<style lang="scss">
	:global(.music-btn) {
		border-radius: 9999px;
		padding: 0.45rem;
		color: var(--color-muted);
		transition: color 0.15s;

		&:hover {
			color: var(--color-text);
		}
	}
</style>
