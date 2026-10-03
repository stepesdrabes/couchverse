<script lang="ts">
	import { RotateCw } from 'lucide-svelte';
	import type { Surface } from '$lib/generated/core';
	import { core } from '$lib/core';
	import Button from '$lib/components/ui/Button.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import * as m from '$lib/paraglide/messages';

	// a cold visit whose load failed (offline, the server down): nothing cached to show
	let { screen }: { screen: Surface } = $props();
</script>

<div class="mx-auto max-w-5xl px-6 pt-40 pb-16">
	<EmptyState title={m.error_page_title()}>
		<Button onclick={() => core.revalidate(screen)} data-tv-autofocus>
			<RotateCw class="size-4" />
			{m.common_retry()}
		</Button>
	</EmptyState>
</div>
