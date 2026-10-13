<script lang="ts">
	import { BRAND } from '@couchverse/broadcast/brand/wordmark';
	import type { Snippet } from 'svelte';
	import Lockup from './Lockup.svelte';

	// The channel's full-screen states without a picture: off air, a link that ended, a channel
	// that would not play, on the brand's flat night field (D42). The actions are the page's.
	let {
		title,
		message = '',
		children
	}: { title: string; message?: string; children?: Snippet } = $props();
</script>

<div
	class="flex min-h-dvh w-full items-center justify-center px-6"
	style="background: {BRAND.night}"
>
	<div class="flex max-w-md flex-col items-center gap-5 text-center">
		<Lockup size={40} layout="stack" />
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
