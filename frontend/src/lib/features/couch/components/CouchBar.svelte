<script lang="ts">
	import { backOut } from 'svelte/easing';
	import { fly, scale } from 'svelte/transition';
	import Couch from './Couch.svelte';
	import CouchPopover from './CouchPopover.svelte';
	import EmojiReactionButton from './EmojiReactionButton.svelte';

	// portalTo: the fullscreen element to portal popovers into (set by the player).
	// showEmoji: hidden while the player controls are hidden, so undisturbed
	// watching stays clean; the couch itself stays visible.
	let { portalTo, showEmoji = true }: { portalTo?: HTMLElement; showEmoji?: boolean } = $props();
</script>

<!-- positioning is owned by the parent (root layout stack or the player wrapper) -->
<div class="flex items-center gap-2" transition:fly={{ y: 90, duration: 450, easing: backOut }}>
	<CouchPopover
		{portalTo}
		triggerClass="block drop-shadow-lg transition-transform hover:scale-[1.03]"
	>
		{#snippet trigger()}
			<Couch height="h-16" avatar="size-8" />
		{/snippet}
	</CouchPopover>
	{#if showEmoji}
		<div transition:scale={{ duration: 200, start: 0.7 }}>
			<EmojiReactionButton
				{portalTo}
				triggerClass="flex size-11 items-center justify-center rounded-full border border-edge
					bg-surface-2/90 text-text shadow-lg backdrop-blur transition-colors hover:bg-surface-2 hover:text-accent"
			/>
		</div>
	{/if}
</div>
