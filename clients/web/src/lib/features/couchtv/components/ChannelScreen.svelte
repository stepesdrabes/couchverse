<script lang="ts">
	import type { Snippet } from 'svelte';
	import { colors } from '$lib/generated/tokens';
	import Lockup from './Lockup.svelte';

	// The channel's full-screen states without a picture: off air, a link that ended, a channel
	// that would not play. The actions are the page's.
	let {
		title,
		message = '',
		children
	}: { title: string; message?: string; children?: Snippet } = $props();
</script>

<div
	class="relative flex min-h-dvh w-full items-center justify-center overflow-hidden bg-black px-6"
>
	<!-- a glow of the brand's red behind the lockup -->
	<div
		class="pointer-events-none absolute inset-0"
		style="background: radial-gradient(ellipse at center, {colors.accent}29, transparent 65%)"
	></div>
	<div class="relative z-10 flex max-w-md flex-col items-center gap-5 text-center">
		<Lockup size={44} />
		<div class="space-y-2">
			<h1 class="text-xl font-bold text-white">{title}</h1>
			{#if message}
				<p class="text-sm text-muted">{message}</p>
			{/if}
		</div>
		{#if children}
			<div class="flex flex-wrap items-center justify-center gap-2">
				{@render children()}
			</div>
		{/if}
	</div>
</div>
