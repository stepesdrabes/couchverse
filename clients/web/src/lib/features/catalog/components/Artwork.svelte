<script lang="ts">
	import { artworkUrl } from '$lib/features/catalog/api';

	// `src` is a ready URL from a core view; the admin and the player still pass artwork ids
	let {
		src = null,
		artworkId = null,
		name,
		v = null,
		class: cls = ''
	}: {
		src?: string | null;
		artworkId?: string | null;
		name: string;
		v?: number | null;
		class?: string;
	} = $props();

	const url = $derived(src ?? (artworkId ? artworkUrl(artworkId, v) : null));
</script>

{#if url}
	<img src={url} alt={name} loading="lazy" class="size-full object-cover {cls}" />
{:else}
	<div
		class="flex size-full items-center justify-center bg-gradient-to-br from-accent-soft
			to-surface-2 {cls}"
	>
		<span class="px-3 text-center text-lg font-bold text-accent-ink/70">{name[0] ?? '?'}</span>
	</div>
{/if}
