<script lang="ts">
	import { Checkbox } from 'bits-ui';
	import { Check } from 'lucide-svelte';

	let {
		checked = $bindable(false),
		onCheckedChange
	}: {
		checked?: boolean;
		onCheckedChange?: (checked: boolean) => void;
	} = $props();
</script>

<Checkbox.Root
	bind:checked
	{onCheckedChange}
	class="flex size-4.5 items-center justify-center rounded border transition-colors
		data-[state=checked]:border-accent data-[state=checked]:bg-accent
		data-[state=unchecked]:border-edge data-[state=unchecked]:bg-surface
		data-[state=unchecked]:hover:border-faint"
>
	{#snippet children({ checked })}
		{#if checked}
			<Check class="size-3 text-white" strokeWidth={3} />
		{/if}
	{/snippet}
</Checkbox.Root>
