<script lang="ts">
	import { Globe } from 'lucide-svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import { putPreferences } from '$lib/features/preferences/api';
	import {
		currentLang,
		displayLangs,
		displayLangLabel,
		setDisplayLang,
		type DisplayLang
	} from '$lib/i18n/locale.svelte';
	import * as m from '$lib/paraglide/messages';

	let value = $state<string>(currentLang());
	const items = displayLangs.map((l) => ({ value: l, label: displayLangLabel(l) }));

	async function choose(next: string) {
		if (next === currentLang()) return;
		try {
			await putPreferences({ language: next });
		} catch {
			// offline/unauthenticated: localStorage still carries the choice
		}
		setDisplayLang(next as DisplayLang); // persists + reloads
	}
</script>

<div class="flex items-center gap-1.5" title={m.language_label()}>
	<Globe class="size-4 shrink-0 text-muted" />
	<Select {items} bind:value onchange={choose} />
</div>
