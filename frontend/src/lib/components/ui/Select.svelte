<script lang="ts">
	import { Select } from 'bits-ui';
	import { Check, ChevronDown } from 'lucide-svelte';

	interface Item {
		value: string;
		label: string;
	}

	let {
		items,
		value = $bindable(''),
		label = '',
		placeholder = 'Any',
		class: cls = '',
		onchange
	}: {
		items: Item[];
		value?: string;
		label?: string;
		placeholder?: string;
		class?: string;
		onchange?: (value: string) => void;
	} = $props();

	const selected = $derived(items.find((i) => i.value === value)?.label ?? placeholder);
</script>

<Select.Root type="single" bind:value onValueChange={(v) => onchange?.(v)}>
	<Select.Trigger
		class="inline-flex h-9 items-center gap-1.5 rounded-full border border-edge bg-surface px-4
			text-xs font-medium text-text transition-colors hover:border-faint {cls}"
	>
		{#if label}<span class="text-muted">{label}:</span>{/if}
		{selected}
		<ChevronDown class="size-3.5 text-muted" />
	</Select.Trigger>
	<Select.Portal>
		<Select.Content
			sideOffset={6}
			class="z-50 max-h-72 min-w-[var(--bits-select-anchor-width)] animate-pop-in overflow-y-auto
				rounded-card border border-edge bg-surface-2 p-1 shadow-xl shadow-black/40"
		>
			{#each items as item (item.value)}
				<Select.Item
					value={item.value}
					label={item.label}
					class="flex cursor-pointer items-center justify-between rounded-lg px-3 py-1.5 text-xs
						text-muted outline-none data-highlighted:bg-surface data-highlighted:text-text"
				>
					{#snippet children({ selected })}
						{item.label}
						{#if selected}<Check class="size-3.5 text-accent" />{/if}
					{/snippet}
				</Select.Item>
			{/each}
		</Select.Content>
	</Select.Portal>
</Select.Root>
