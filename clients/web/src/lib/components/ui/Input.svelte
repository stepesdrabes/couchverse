<script lang="ts">
	import type { HTMLInputAttributes } from 'svelte/elements';

	type Props = HTMLInputAttributes & {
		label?: string;
		error?: string;
		value?: string;
	};

	let { label, error, value = $bindable(''), class: cls = '', ...rest }: Props = $props();
</script>

<label class="block {cls}">
	{#if label}
		<span class="mb-1.5 block text-xs font-medium text-muted">{label}</span>
	{/if}
	<input
		bind:value
		class="h-10 w-full rounded-input border bg-surface px-3.5 text-sm transition-colors
			placeholder:text-faint focus:outline-none
			{error ? 'border-danger' : 'border-edge focus:border-accent'}"
		{...rest}
	/>
	{#if error}
		<span class="mt-1.5 block text-xs text-danger">{error}</span>
	{/if}
</label>
