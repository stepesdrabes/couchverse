<script lang="ts">
	import { artworkUrl } from '$lib/features/catalog/api';

	let {
		artworkId,
		name,
		v = null,
		class: cls = ''
	}: { artworkId: string | null; name: string; v?: number | null; class?: string } = $props();
</script>

{#if artworkId}
	<img
		src={artworkUrl(artworkId, v)}
		alt={name}
		loading="lazy"
		class="size-full object-cover {cls}"
	/>
{:else}
	<div
		class="flex size-full items-center justify-center bg-gradient-to-br from-accent-soft
			to-surface-2 {cls}"
	>
		<span class="px-3 text-center text-lg font-bold text-accent/70">{name[0] ?? '?'}</span>
	</div>
{/if}
