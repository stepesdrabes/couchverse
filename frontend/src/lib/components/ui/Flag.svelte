<script lang="ts">
	import { langToCountry } from '$lib/i18n/flags';

	// Renders a language's flag via the bundled flag-icons CSS. Sizes itself from
	// the surrounding font-size (flag-icons draws a 4:3 box of 1em height); pass a
	// text-size class to scale. Nothing renders for languages without a flag.
	let { code, class: cls = '' }: { code: string; class?: string } = $props();

	const cc = $derived(langToCountry(code));
</script>

{#if cc}
	<span class="fi fi-{cc} shrink-0 rounded-[2px] {cls}" aria-hidden="true"></span>
{/if}
