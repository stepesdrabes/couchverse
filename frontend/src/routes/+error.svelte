<script lang="ts">
	import { page } from '$app/state';
	import { House } from 'lucide-svelte';
	import * as m from '$lib/paraglide/messages';

	const status = $derived(page.status);
	const heading = $derived(status === 404 ? m.error_not_found() : m.error_page_title());
	// for non-404s surface the server message; for 404 the heading already says it
	const detail = $derived(status !== 404 ? (page.error?.message ?? '') : '');
</script>

<svelte:head>
	<title>{status} · Couchverse</title>
</svelte:head>

<div
	class="relative flex min-h-dvh flex-col items-center justify-center overflow-hidden px-6 text-center"
>
	<div
		class="pointer-events-none absolute top-1/2 left-1/2 -z-10 size-[34rem] max-w-[90vw] -translate-x-1/2
			-translate-y-1/2 rounded-full bg-accent/15 blur-[120px]"
	></div>

	<p class="text-[7rem] leading-none font-black text-accent tnum sm:text-[10rem]">{status}</p>
	<h1 class="mt-1 text-xl font-bold">{heading}</h1>
	{#if detail}
		<p class="mt-2 max-w-md text-sm text-muted">{detail}</p>
	{/if}

	<a
		href="/"
		class="mt-8 inline-flex h-11 items-center gap-2 rounded-full bg-accent px-6 text-sm font-semibold
			text-[var(--color-on-accent)] shadow-lg transition-colors hover:bg-accent-strong"
	>
		<House class="size-4" />
		{m.error_go_home()}
	</a>
</div>
