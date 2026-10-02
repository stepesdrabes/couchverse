<script lang="ts">
	import { Tabs } from 'bits-ui';

	interface Item {
		value: string;
		label: string;
	}

	let {
		items,
		value = $bindable(''),
		onchange
	}: { items: Item[]; value?: string; onchange?: (value: string) => void } = $props();
</script>

<Tabs.Root bind:value onValueChange={(v) => onchange?.(v)}>
	<Tabs.List class="inline-flex items-center gap-1 rounded-full border border-edge bg-surface p-1">
		{#each items as item (item.value)}
			<Tabs.Trigger
				value={item.value}
				class="rounded-full px-4 py-1.5 text-xs font-semibold text-muted transition-colors
					hover:text-text data-[state=active]:bg-accent-soft data-[state=active]:text-text"
			>
				{item.label}
			</Tabs.Trigger>
		{/each}
	</Tabs.List>
</Tabs.Root>
