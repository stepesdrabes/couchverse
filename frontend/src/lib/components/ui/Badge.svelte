<script lang="ts">
	import type { Snippet } from 'svelte';

	let { children }: { children: Snippet } = $props();
</script>

<span
	class="inline-flex items-center rounded border border-edge bg-surface-2 px-1.5 py-0.5 text-[10px]
		font-semibold tracking-wider text-muted uppercase"
>
	{@render children()}
</span>
