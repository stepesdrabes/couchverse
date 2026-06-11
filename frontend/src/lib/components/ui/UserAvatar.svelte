<script lang="ts">
	import { minidenticon } from 'minidenticons';
	import { artworkUrl } from '$lib/features/catalog/api';

	let {
		name,
		avatarId,
		seed,
		class: cls = 'size-8 rounded-lg text-xs'
	}: {
		name: string;
		avatarId: string | null | undefined;
		// stable per-user value (id/username) the placeholder identicon hashes from
		seed?: string | number | null;
		class?: string;
	} = $props();

	const identicon = $derived(minidenticon(String(seed ?? name ?? '?')));
</script>

{#if avatarId}
	<span class="flex items-center justify-center overflow-hidden {cls}">
		<img src="{artworkUrl(avatarId)}?size=w342" alt={name} class="size-full object-cover" />
	</span>
{:else}
	<span class="identicon flex items-center justify-center overflow-hidden bg-surface-2 {cls}">
		<!-- minidenticon returns a GitHub-style pixel SVG; hue derives from the seed -->
		{@html identicon}
	</span>
{/if}

<style>
	.identicon :global(svg) {
		width: 100%;
		height: 100%;
	}
</style>
