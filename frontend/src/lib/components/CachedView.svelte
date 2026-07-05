<script lang="ts" generics="T">
	import type { Snippet } from 'svelte';

	// Three-state view over the SWR cache: cached value paints at once (and keeps
	// showing even if a later revalidation fails - stale beats blank), a cold visit
	// shows the skeleton, and a first-load failure shows notFound (or the skeleton).
	let {
		value,
		fresh,
		content,
		skeleton,
		notFound
	}: {
		value: T | undefined;
		fresh: Promise<T>;
		content: Snippet<[T]>;
		skeleton: Snippet;
		notFound?: Snippet;
	} = $props();

	let failed = $state(false);
	$effect(() => {
		failed = false;
		const pending = fresh;
		pending.catch(() => {
			if (pending === fresh && value === undefined) failed = true;
		});
	});
</script>

{#if value !== undefined}
	{@render content(value)}
{:else if failed && notFound}
	{@render notFound()}
{:else}
	{@render skeleton()}
{/if}
