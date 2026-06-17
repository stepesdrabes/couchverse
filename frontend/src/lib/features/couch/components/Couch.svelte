<script lang="ts">
	import { onMount } from 'svelte';
	import { backOut } from 'svelte/easing';
	import { fade, fly, slide } from 'svelte/transition';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import Tooltip from '$lib/components/ui/Tooltip.svelte';
	import { couch } from '$lib/features/couch/couch.svelte';
	import CouchLeft from './icons/CouchLeft.svelte';
	import CouchMiddle from './icons/CouchMiddle.svelte';
	import CouchRight from './icons/CouchRight.svelte';
	import Remote from './icons/Remote.svelte';

	let { height = 'h-20', avatar = 'size-9' }: { height?: string; avatar?: string } = $props();

	// left + (n-1) middle + right (one participant => left + right, no middle)
	const middles = $derived(
		Array.from({ length: Math.max(0, couch.participants.length - 1) }, (_, i) => i)
	);

	const reactionsFor = (pid: string) => couch.reactions.filter((r) => r.participantId === pid);

	// suppress the per-seat intros on first paint so launch is just the bar fly;
	// later joins then animate (middle slides in, avatar pops down)
	let ready = $state(false);
	onMount(() => {
		ready = true;
	});
	const seatIn = $derived(ready ? { y: -28, duration: 350, easing: backOut } : { duration: 0 });
</script>

<div class="relative inline-flex text-accent" in:fly={{ y: -28, duration: 350, easing: backOut }}>
	<!-- seated avatars overlaid on the cushions -->
	<div
		class="absolute inset-x-0 top-1/2 z-10 flex -translate-y-[60%] items-end justify-center gap-6 px-5"
	>
		{#each couch.participants as p (p.id)}
			<div class="relative" data-couch-seat={p.id} in:fly={seatIn} out:fade={{ duration: 150 }}>
				<Tooltip label={p.displayName}>
					{#snippet trigger(props)}
						<span {...props} class="block">
							<UserAvatar
								name={p.displayName}
								avatarId={p.avatarId ?? null}
								seed={p.seed}
								class="{avatar} rounded-lg text-[10px] ring-2 ring-black/40"
							/>
						</span>
					{/snippet}
				</Tooltip>
				{#if p.isHost}
					<span
						class="absolute -top-2.5 -right-2 flex size-4 rotate-12 items-center justify-center
							rounded-full bg-black/70 text-white shadow"
						title="Host"
					>
						<Remote class="size-2.5" />
					</span>
				{/if}
				<!-- reactions fly up from the sender's seat -->
				{#each reactionsFor(p.id) as r (r.id)}
					<span
						class="pointer-events-none absolute -top-1 left-1/2 z-20 -translate-x-1/2 text-xl"
						in:fly={{ y: -40, duration: 1200 }}
						out:fade={{ duration: 250 }}
					>
						{r.emoji}
					</span>
				{/each}
			</div>
		{/each}
	</div>

	<!-- the assembled couch frame -->
	<div class="flex items-end {height}">
		<CouchLeft class="h-full w-auto" />
		{#each middles as i (i)}
			<div class="flex h-full" transition:slide={{ axis: 'x' as const, duration: 350 }}>
				<CouchMiddle class="h-full w-auto" />
			</div>
		{/each}
		<CouchRight class="h-full w-auto" />
	</div>
</div>
