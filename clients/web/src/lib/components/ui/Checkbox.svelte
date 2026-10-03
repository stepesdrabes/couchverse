<script lang="ts">
	import { Checkbox } from 'bits-ui';
	import { Check } from 'lucide-svelte';

	let {
		checked = $bindable(false),
		label,
		onCheckedChange
	}: {
		checked?: boolean;
		/** what it turns on, for screen readers: the control has no text of its own */
		label: string;
		onCheckedChange?: (checked: boolean) => void;
	} = $props();
</script>

<Checkbox.Root
	bind:checked
	{onCheckedChange}
	aria-label={label}
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
