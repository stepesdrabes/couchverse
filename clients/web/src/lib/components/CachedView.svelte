<script lang="ts" generics="T">
	import type { Snippet } from 'svelte';
	import { LoadStatus } from '$lib/generated/core';

	// Three-state view over a core view: a cached value paints at once (and keeps showing even
	// if a later reload fails - stale beats blank), a cold visit shows the skeleton, and a
	// first-load failure shows notFound (or `failed` for anything but a 404, when given).
	let {
		value,
		status,
		content,
		skeleton,
		notFound,
		failed
	}: {
		value: T | undefined;
		status?: LoadStatus;
		content: Snippet<[T]>;
		skeleton: Snippet;
		notFound?: Snippet;
		failed?: Snippet;
	} = $props();

	const missing = $derived(status === LoadStatus.NotFound);
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
