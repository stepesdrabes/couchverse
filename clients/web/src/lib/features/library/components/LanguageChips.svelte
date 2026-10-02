<script lang="ts">
	import { Plus, X } from 'lucide-svelte';
	import Flag from '$lib/components/ui/Flag.svelte';
	import { langLabel } from '$lib/i18n/content-langs';
	import AddLanguageModal from './AddLanguageModal.svelte';
	import * as m from '$lib/paraglide/messages';

	// content-language picker: the selected languages as removable chips plus an
	// "Add language" button opening a searchable modal. A parent can intercept
	// removals via onremove (the title editor purges translations + files); without
	// it, removal just updates the bound selection (e.g. when creating a title).
	let {
		selected = $bindable<string[]>([]),
		onremove
	}: { selected?: string[]; onremove?: (code: string) => void } = $props();

	let addOpen = $state(false);

	function remove(code: string) {
		if (selected.length <= 1) return; // a title needs at least one content language
		if (onremove) onremove(code);
		else selected = selected.filter((c) => c !== code);
	}
	function add(code: string) {
		if (!selected.includes(code)) selected = [...selected, code];
	}
</script>

<div class="flex flex-wrap items-center gap-1.5">
	{#each selected as code (code)}
		<span
			class="flex items-center gap-1.5 rounded-full border border-edge bg-surface py-1 pr-1.5 pl-2.5
				text-xs text-text"
		>
			<Flag {code} />
			{langLabel(code)}
			{#if selected.length > 1}
				<button
					type="button"
					onclick={() => remove(code)}
					class="rounded-full p-0.5 text-faint transition-colors hover:bg-danger/15 hover:text-danger"
					title={m.library_remove_language()}
				>
					<X class="size-3" />
				</button>
			{/if}
		</span>
	{/each}
	<button
		type="button"
		onclick={() => (addOpen = true)}
		class="flex items-center gap-1 rounded-full border border-dashed border-edge px-2.5 py-1
			text-xs text-muted transition-colors hover:border-faint hover:text-text"
	>
		<Plus class="size-3" />
		{m.library_add_language()}
	</button>
</div>

<AddLanguageModal bind:open={addOpen} {selected} onpick={add} />
