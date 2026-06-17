<script lang="ts">
	import { fly } from 'svelte/transition';
	import Couch from './Couch.svelte';
	import CouchPopover from './CouchPopover.svelte';
	import EmojiReactionButton from './EmojiReactionButton.svelte';
</script>

<!-- positioning is owned by the root layout's corner stack -->
<div class="flex items-end gap-2" transition:fly={{ y: 20, duration: 250 }}>
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
