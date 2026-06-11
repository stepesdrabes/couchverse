<script lang="ts">
	import { Tooltip } from 'bits-ui';
	import type { Snippet } from 'svelte';

	let {
		label,
		side = 'top',
		portalTo = undefined,
		trigger
	}: {
		label: string;
		side?: 'top' | 'bottom' | 'left' | 'right';
		// render the tooltip inside this element (e.g. the fullscreen wrapper)
		portalTo?: HTMLElement | undefined;
		// receives the trigger props to spread onto your own element
		trigger: Snippet<[Record<string, unknown>]>;
	} = $props();
</script>

<Tooltip.Provider delayDuration={250}>
	<Tooltip.Root>
		<Tooltip.Trigger>
			{#snippet child({ props })}
				{@render trigger(props)}
			{/snippet}
		</Tooltip.Trigger>
		<Tooltip.Portal to={portalTo}>
			<Tooltip.Content
				{side}
				sideOffset={8}
				class="z-50 animate-pop-in rounded-md border border-edge bg-surface-2 px-2 py-1 text-xs
					font-medium text-text shadow-lg shadow-black/40"
			>
				{label}
			</Tooltip.Content>
		</Tooltip.Portal>
	</Tooltip.Root>
</Tooltip.Provider>
