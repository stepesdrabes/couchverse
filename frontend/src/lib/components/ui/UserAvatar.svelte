<script lang="ts">
	import { artworkUrl } from '$lib/features/catalog/api';

	let {
		name,
		avatarId,
		class: cls = 'size-8 rounded-lg text-xs'
	}: { name: string; avatarId: number | null | undefined; class?: string } = $props();
</script>

<span
	class="flex items-center justify-center overflow-hidden bg-danger/80 font-bold text-white uppercase {cls}"
>
	{#if avatarId}
		<img src="{artworkUrl(avatarId)}?size=w342" alt={name} class="size-full object-cover" />
	{:else}
		{name[0] ?? '?'}
	{/if}
</span>
