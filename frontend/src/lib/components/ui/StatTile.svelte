<script lang="ts">
	import type { Icon as LucideIcon } from 'lucide-svelte';

	// The headline-number card: an accent icon chip, a big tabular value and a
	// label. `live` dims the whole tile at zero and pings the chip above it, so a
	// quiet server reads as quiet rather than broken.
	let {
		icon: Icon,
		value,
		label,
		sub = undefined,
		href = undefined,
		live = false,
		size = 'md'
	}: {
		icon: typeof LucideIcon;
		value: string | number;
		label: string;
		sub?: string;
		href?: string;
		live?: boolean;
		size?: 'sm' | 'md';
	} = $props();

	const hot = $derived(live && Number(value) > 0);
	const dim = $derived(live && Number(value) <= 0);
</script>

<svelte:element
	this={href ? 'a' : 'div'}
	{href}
	class="group flex items-center gap-4 rounded-card border bg-surface/40 transition-colors
		{size === 'sm' ? 'p-5' : 'p-6'}
		{hot ? 'border-accent/50' : 'border-edge'}
		{href ? 'hover:border-accent/50 hover:bg-surface/70' : ''}"
>
	<span
		class="relative flex shrink-0 items-center justify-center rounded-xl
			{size === 'sm' ? 'size-11' : 'size-12'}
			{dim ? 'bg-surface-2' : 'bg-accent-soft'}"
	>
		<Icon class="size-5 {dim ? 'text-faint' : 'text-accent'}" />
		{#if hot}
			<span class="absolute -top-0.5 -right-0.5 flex size-2.5">
				<span class="absolute inline-flex size-full animate-ping rounded-full bg-accent/70"></span>
				<span class="relative inline-flex size-2.5 rounded-full bg-accent"></span>
			</span>
		{/if}
	</span>
	<span class="min-w-0">
		<span
			class="block font-extrabold tracking-tight tnum
				{size === 'sm' ? 'text-2xl' : 'text-3xl'} {dim ? 'text-faint' : ''}"
		>
			{value}
		</span>
		<span class="block text-xs font-medium text-muted transition-colors group-hover:text-text">
			{label}
		</span>
		{#if sub}
			<span class="block text-[11px] text-faint tnum">{sub}</span>
		{/if}
	</span>
</svelte:element>
