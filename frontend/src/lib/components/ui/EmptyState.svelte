<script lang="ts">
	import type { Snippet } from 'svelte';

	let {
		title,
		message = '',
		children
	}: { title: string; message?: string; children?: Snippet } = $props();
</script>

<div class="flex flex-col items-center justify-center gap-2 py-20 text-center">
	<p class="text-sm font-semibold text-muted">{title}</p>
	{#if message}
		<p class="max-w-sm text-xs text-faint">{message}</p>
	{/if}
	{#if children}
		<div class="mt-3">{@render children()}</div>
	{/if}
</div>
