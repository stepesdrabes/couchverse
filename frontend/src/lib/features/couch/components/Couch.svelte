<script lang="ts">
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
</script>

<div class="relative inline-flex text-accent">
	<!-- seated avatars overlaid on the cushions -->
	<div
		class="absolute inset-x-0 top-1/2 z-10 flex -translate-y-[60%] items-end justify-center gap-1.5 px-4"
	>
		{#each couch.participants as p (p.id)}
			<div class="relative" data-couch-seat={p.id}>
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
			</div>
		{/each}
	</div>

	<!-- the assembled couch frame -->
	<div class="flex items-end {height}">
		<CouchLeft class="h-full w-auto" />
		{#each middles as i (i)}
			<CouchMiddle class="h-full w-auto" />
		{/each}
		<CouchRight class="h-full w-auto" />
	</div>
</div>
