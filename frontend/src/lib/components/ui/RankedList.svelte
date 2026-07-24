<script lang="ts" module>
	export interface RankedItem {
		key: string;
		label: string;
		href?: string;
		value: number;
		display: string;
	}
</script>

<script lang="ts">
	import { onMount } from 'svelte';

	// A leaderboard-shaped list: label, value, and a bar proportional to the
	// leader. Bars scale rather than resize so a long list stays cheap to animate.
	let {
		items,
		animate = true
	}: {
		items: RankedItem[];
		animate?: boolean;
	} = $props();

	const top = $derived(items[0]?.value ?? 0);
	const share = (value: number) => (top > 0 ? value / top : 0);

	// bars start empty and fill on the first frame after mount, so they sweep in
	let ready = $state(!animate);
	onMount(() => {
		ready = true;
	});
</script>

<ul class="space-y-2.5 text-xs">
	{#each items as item, i (item.key)}
		<li>
			<div class="flex items-baseline justify-between gap-3">
				{#if item.href}
					<a href={item.href} class="truncate font-medium transition-colors hover:text-accent">
						{item.label}
					</a>
				{:else}
					<span class="truncate font-medium">{item.label}</span>
				{/if}
				<span class="shrink-0 text-faint tnum">{item.display}</span>
			</div>
			<div class="mt-1 h-1 overflow-hidden rounded-full bg-surface-2">
				<div
					class="h-full origin-left rounded-full bg-accent transition-transform duration-700 ease-out"
					style="transform: scaleX({ready ? share(item.value) : 0});
						transition-delay: {Math.min(i * 40, 300)}ms"
				></div>
			</div>
		</li>
	{/each}
</ul>
