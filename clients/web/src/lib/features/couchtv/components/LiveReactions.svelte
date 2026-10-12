<script lang="ts">
	import type { CouchTvView } from '$lib/generated/core';

	// Everyone's reactions float up the picture's left side with the sender's name, whether
	// the controls are up or not. The core drops each one once its rise is over.
	let { view, active }: { view: CouchTvView; active: boolean } = $props();

	const names = $derived(new Map(view.members.map((v) => [v.id, v.displayName])));

	// spread over a lane by id, so a burst does not stack in one column
	const lane = (id: number) => ((id * 37) % 9) * 1.1;
</script>

<!-- decorative: a reaction says nothing a screen reader could act on -->
<div
	class="lane pointer-events-none absolute left-6 z-10 h-0 w-48 sm:left-10"
	style="bottom: {active ? '7.5rem' : '3rem'}"
	aria-hidden="true"
>
	{#each view.reactions as reaction (reaction.id)}
		<span class="couchtv-reaction absolute bottom-0" style="left: {lane(reaction.id)}rem">
			<span class="block text-3xl leading-none">{reaction.emoji}</span>
			{#if names.get(reaction.viewerId)}
				<span
					class="mt-1 block max-w-24 truncate rounded-full bg-black/60 px-1.5 text-[10px] font-semibold
						text-white"
				>
					{names.get(reaction.viewerId)}
				</span>
			{/if}
		</span>
	{/each}
</div>

<style>
	.lane {
		transition: bottom 0.25s ease;
	}

	/* the couch's rise and fade, a little taller for a full-screen picture; the core keeps a
	   reaction for 2.2 s */
	.couchtv-reaction {
		display: flex;
		flex-direction: column;
		align-items: center;
		animation: couchtv-reaction-rise 2.2s cubic-bezier(0.25, 0.6, 0.3, 1) forwards;
	}

	@keyframes couchtv-reaction-rise {
		0% {
			transform: translateY(8px) scale(0.5);
			opacity: 0;
		}
		15% {
			opacity: 1;
		}
		70% {
			opacity: 1;
		}
		100% {
			transform: translateY(-160px) scale(1.4);
			opacity: 0;
		}
	}
</style>
