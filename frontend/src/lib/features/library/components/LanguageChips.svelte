<script lang="ts">
	import { Check, Plus } from 'lucide-svelte';
	import { CONTENT_LANGS, langLabel } from '$lib/i18n/content-langs';
	import * as m from '$lib/paraglide/messages';

	// content-language picker: the common languages as toggle chips, plus any
	// custom ISO code the admin types. Selected custom codes show as chips too.
	let { selected = $bindable<string[]>([]) }: { selected?: string[] } = $props();

	let adding = $state(false);
	let custom = $state('');

	const codes = $derived([
		...CONTENT_LANGS.map((l) => l.code),
		...selected.filter((c) => !CONTENT_LANGS.some((l) => l.code === c))
	]);

	function toggle(code: string) {
		selected = selected.includes(code) ? selected.filter((c) => c !== code) : [...selected, code];
	}
	function addCustom() {
		const code = custom.trim().toLowerCase();
		if (code && !selected.includes(code)) selected = [...selected, code];
		custom = '';
		adding = false;
	}
</script>

<div class="flex flex-wrap items-center gap-1.5">
	{#each codes as code (code)}
		<button
			type="button"
			onclick={() => toggle(code)}
			class="flex items-center gap-1 rounded-full border px-2.5 py-1 text-xs transition-colors
				{selected.includes(code)
				? 'border-accent bg-accent/15 text-text'
				: 'border-edge text-muted hover:border-faint'}"
		>
			{#if selected.includes(code)}<Check class="size-3" />{/if}
			{langLabel(code)}
		</button>
	{/each}
	{#if adding}
		<input
			bind:value={custom}
			placeholder={m.library_language_code_placeholder()}
			class="h-7 w-28 rounded-full border border-accent bg-surface px-2.5 text-xs text-text
				placeholder:text-faint focus:outline-none"
			onkeydown={(e) => {
				if (e.key === 'Enter') {
					e.preventDefault();
					addCustom();
				}
			}}
			onblur={addCustom}
		/>
	{:else}
		<button
			type="button"
			onclick={() => (adding = true)}
			class="flex items-center gap-1 rounded-full border border-dashed border-edge px-2.5 py-1
				text-xs text-muted transition-colors hover:border-faint hover:text-text"
		>
			<Plus class="size-3" />
			{m.library_add_language()}
		</button>
	{/if}
</div>
