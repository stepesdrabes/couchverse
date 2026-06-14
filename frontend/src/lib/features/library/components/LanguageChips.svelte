<script lang="ts">
	import { Check } from 'lucide-svelte';

	// content-language picker: which languages TMDB metadata is fetched in for a
	// title. Labels are endonyms (same in any UI locale), so no i18n needed here.
	let { selected = $bindable<string[]>([]) }: { selected?: string[] } = $props();

	const OPTIONS = [
		{ code: 'en', label: 'English' },
		{ code: 'cs', label: 'Čeština' },
		{ code: 'sk', label: 'Slovenčina' },
		{ code: 'de', label: 'Deutsch' },
		{ code: 'es', label: 'Español' },
		{ code: 'fr', label: 'Français' },
		{ code: 'it', label: 'Italiano' },
		{ code: 'pl', label: 'Polski' },
		{ code: 'ko', label: '한국어' },
		{ code: 'ja', label: '日本語' }
	];

	function toggle(code: string) {
		selected = selected.includes(code) ? selected.filter((c) => c !== code) : [...selected, code];
	}
</script>

<div class="flex flex-wrap gap-1.5">
	{#each OPTIONS as opt (opt.code)}
		<button
			type="button"
			onclick={() => toggle(opt.code)}
			class="flex items-center gap-1 rounded-full border px-2.5 py-1 text-xs transition-colors
				{selected.includes(opt.code)
				? 'border-accent bg-accent/15 text-text'
				: 'border-edge text-muted hover:border-faint'}"
		>
			{#if selected.includes(opt.code)}<Check class="size-3" />{/if}
			{opt.label}
		</button>
	{/each}
</div>
