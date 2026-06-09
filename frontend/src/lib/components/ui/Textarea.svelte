<script lang="ts">
	import type { HTMLTextareaAttributes } from 'svelte/elements';

	type Props = HTMLTextareaAttributes & {
		label?: string;
		value?: string;
	};

	let { label, value = $bindable(''), class: cls = '', ...rest }: Props = $props();
</script>

<label class="block {cls}">
	{#if label}
		<span class="mb-1.5 block text-xs font-medium text-muted">{label}</span>
	{/if}
	<textarea
		bind:value
		rows={4}
		class="w-full rounded-input border border-edge bg-surface px-3.5 py-2.5 text-sm transition-colors
			placeholder:text-faint focus:border-accent focus:outline-none"
		{...rest}
	></textarea>
</label>
