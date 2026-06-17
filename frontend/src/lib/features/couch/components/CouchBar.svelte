<script lang="ts">
	import { fly } from 'svelte/transition';
	import { page } from '$app/state';
	import Couch from './Couch.svelte';
	import CouchPopover from './CouchPopover.svelte';
	import EmojiReactionButton from './EmojiReactionButton.svelte';

	// sit above the music bar on app pages, low on the immersive player
	const immersive = $derived(
		(page.route.id?.includes('/watch/') || page.route.id?.includes('/couch/')) ?? false
	);
</script>

<div
	class="fixed right-4 z-40 flex items-end gap-2 {immersive ? 'bottom-4' : 'bottom-24'}"
	transition:fly={{ y: 20, duration: 250 }}
>
	<CouchPopover triggerClass="block drop-shadow-lg transition-transform hover:scale-[1.03]">
		{#snippet trigger()}
			<Couch height="h-16" avatar="size-8" />
		{/snippet}
	</CouchPopover>
	<EmojiReactionButton
		triggerClass="flex size-11 items-center justify-center rounded-full border border-edge
			bg-surface-2/90 text-text shadow-lg backdrop-blur transition-colors hover:bg-surface-2 hover:text-accent"
	/>
</div>
