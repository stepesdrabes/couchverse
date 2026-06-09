<script lang="ts">
	import { Switch } from 'bits-ui';

	let {
		checked = $bindable(false),
		onCheckedChange
	}: {
		checked?: boolean;
		onCheckedChange?: (checked: boolean) => void;
	} = $props();
</script>

<Switch.Root
	bind:checked
	{onCheckedChange}
	class="inline-flex h-5.5 w-9.5 shrink-0 items-center rounded-full border border-edge px-0.5
		transition-colors data-[state=checked]:bg-accent data-[state=unchecked]:bg-surface"
>
	<Switch.Thumb
		class="size-4 rounded-full bg-white shadow transition-transform
			data-[state=checked]:translate-x-4 data-[state=unchecked]:translate-x-0"
	/>
</Switch.Root>
