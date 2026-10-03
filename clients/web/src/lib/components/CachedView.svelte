<script lang="ts" generics="T">
	import type { Snippet } from 'svelte';
	import { LoadStatus } from '$lib/generated/core';

	// Three-state view over cached data: a cached value paints at once (and keeps showing even
	// if a later revalidation fails - stale beats blank), a cold visit shows the skeleton, and a
	// first-load failure shows notFound (or `failed` for anything but a 404, when given). The
	// failure comes from a core view's `status` or, over the web's SWR cache, a rejected `fresh`.
	let {
		value,
		fresh,
		status,
		content,
		skeleton,
		notFound,
		failed
	}: {
		value: T | undefined;
		fresh?: Promise<T>;
		status?: LoadStatus;
		content: Snippet<[T]>;
		skeleton: Snippet;
		notFound?: Snippet;
		failed?: Snippet;
	} = $props();

	let rejected = $state(false);
	$effect(() => {
		rejected = false;
		const pending = fresh;
		pending?.catch(() => {
			if (pending === fresh && value === undefined) rejected = true;
		});
	});

	const missing = $derived(rejected || status === LoadStatus.NotFound);
	const broken = $derived(status === LoadStatus.Failed);
</script>

{#if value !== undefined}
	{@render content(value)}
{:else if broken && failed}
	{@render failed()}
{:else if (missing || broken) && notFound}
	{@render notFound()}
{:else}
	{@render skeleton()}
{/if}
