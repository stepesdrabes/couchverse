<script lang="ts">
	import { Search } from 'lucide-svelte';
	import Flag from '$lib/components/ui/Flag.svelte';
	import Modal from '$lib/components/ui/Modal.svelte';
	import { langLabel, searchLangs } from '$lib/i18n/content-langs';
	import * as m from '$lib/paraglide/messages';

	let {
		open = $bindable(false),
		selected = [],
		onpick
	}: { open?: boolean; selected?: string[]; onpick: (code: string) => void } = $props();

	let query = $state('');

	// all languages matching the search, minus the ones already chosen
	const results = $derived(searchLangs(query).filter((c) => !selected.includes(c)));

	function pick(code: string) {
		onpick(code);
		query = '';
		open = false;
	}
</script>

<Modal bind:open title={m.library_add_language_title()} size="md">
	<div class="relative mb-3">
		<Search class="absolute top-1/2 left-3 size-4 -translate-y-1/2 text-faint" />
		<input
			bind:value={query}
			placeholder={m.library_search_language_placeholder()}
			class="h-9 w-full rounded-full border border-edge bg-surface pr-4 pl-9 text-sm
				placeholder:text-faint focus:border-accent focus:outline-none"
		/>
	</div>
	<ul class="max-h-80 space-y-0.5 overflow-y-auto">
		{#each results as code (code)}
			<li>
				<button
					type="button"
					onclick={() => pick(code)}
					class="flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-left text-sm text-muted
						transition-colors hover:bg-surface hover:text-text"
				>
					<Flag {code} />
					<span class="flex-1 truncate">{langLabel(code)}</span>
					<span class="text-[11px] tracking-wide text-faint uppercase">{code}</span>
				</button>
			</li>
		{:else}
			<li class="px-3 py-6 text-center text-xs text-faint">{m.library_no_languages_found()}</li>
		{/each}
	</ul>
</Modal>
