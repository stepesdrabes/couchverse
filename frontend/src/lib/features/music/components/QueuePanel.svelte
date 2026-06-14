<script lang="ts">
	import { X } from 'lucide-svelte';
	import { fly } from 'svelte/transition';
	import { musicPlayer as player } from '$lib/features/music/player.svelte';
	import { formatClock } from '$lib/utils/format';
	import * as m from '$lib/paraglide/messages';
</script>

{#if player.queueOpen}
	<aside
		transition:fly={{ x: 320, duration: 250 }}
		class="fixed top-0 right-0 bottom-20 z-40 flex w-80 flex-col border-l border-edge bg-surface/95 backdrop-blur-md"
	>
		<div class="flex items-center justify-between border-b border-edge/60 px-5 py-4">
			<h2 class="eyebrow">{m.music_queue()}</h2>
			<button
				class="rounded-full p-1.5 text-muted transition-colors hover:bg-surface-2 hover:text-text"
				onclick={() => (player.queueOpen = false)}
				aria-label={m.music_close_queue()}
			>
				<X class="size-4" />
			</button>
		</div>
		<ul class="flex-1 overflow-y-auto p-2">
			{#each player.queue as track, i (track.id + '-' + i)}
				<li
					class="group flex items-center gap-3 rounded-lg px-3 py-2
						{i === player.index ? 'bg-accent-soft/40' : 'hover:bg-surface-2/60'}"
				>
					<button class="min-w-0 flex-1 text-left" onclick={() => player.jumpTo(i)}>
						<p class="truncate text-sm {i === player.index ? 'font-semibold text-accent' : ''}">
							{track.name}
						</p>
						<p class="truncate text-xs text-faint">
							{track.trackArtist ?? track.artistName}
						</p>
					</button>
					<span class="text-[10px] text-faint tnum">{formatClock(track.durationSeconds)}</span>
					{#if i !== player.index}
						<button
							class="rounded-full p-1 text-faint opacity-0 transition-opacity group-hover:opacity-100 hover:text-danger"
							onclick={() => player.removeFromQueue(i)}
							aria-label={m.music_remove_from_queue()}
						>
							<X class="size-3.5" />
						</button>
					{/if}
				</li>
			{/each}
		</ul>
	</aside>
{/if}
