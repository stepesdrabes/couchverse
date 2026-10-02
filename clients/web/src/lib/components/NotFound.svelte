<script lang="ts">
	import { House } from 'lucide-svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import * as m from '$lib/paraglide/messages';

	let {
		title = m.error_not_found(),
		href = '/',
		label = m.error_go_home()
	}: { title?: string; href?: string; label?: string } = $props();
</script>

<div class="mx-auto max-w-5xl px-6 pt-40 pb-16">
	<EmptyState {title}>
		<a
			{href}
			class="inline-flex h-10 items-center gap-2 rounded-full bg-accent px-5 text-sm font-semibold
				text-[var(--color-on-accent)] transition-colors hover:bg-accent-strong"
		>
			<House class="size-4" />
			{label}
		</a>
	</EmptyState>
</div>
