<script lang="ts" generics="T">
	import type { Snippet } from 'svelte';

	// Streamed (uncached) view: resolves the load's promise into local state and
	// keeps the last value while a same-key revalidation (e.g. invalidateAll after a
	// save) is in flight, so it never flashes the skeleton mid-edit. A genuinely new
	// key clears back to the skeleton. Use where data must stay fresh per visit
	// (admin editors) rather than served from the SWR cache.
	let {
		key,
		data,
		content,
		skeleton,
		notFound
	}: {
		key: string;
		data: Promise<T>;
		content: Snippet<[T]>;
		skeleton: Snippet;
		notFound?: Snippet;
	} = $props();

	let resolved = $state<T | undefined>(undefined);
	let failed = $state(false);
	let shownKey = '';

	$effect(() => {
		const k = key;
		const promise = data;
		if (k !== shownKey) {
			resolved = undefined;
			failed = false;
			shownKey = k;
		}
		let live = true;
		promise.then(
			(v) => {
				if (live) {
					resolved = v;
					failed = false;
				}
			},
			() => {
				if (live && resolved === undefined) failed = true;
			}
		);
		return () => {
			live = false;
		};
	});
</script>

{#if resolved !== undefined}
	{@render content(resolved)}
{:else if failed && notFound}
	{@render notFound()}
{:else}
	{@render skeleton()}
{/if}
