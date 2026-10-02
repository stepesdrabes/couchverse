<script lang="ts">
	import { DropdownMenu } from 'bits-ui';
	import { Check, ChevronDown } from 'lucide-svelte';
	import Flag from '$lib/components/ui/Flag.svelte';
	import { putPreferences } from '$lib/features/preferences/api';
	import {
		currentLang,
		displayLangs,
		displayLangLabel,
		setDisplayLang,
		type DisplayLang
	} from '$lib/i18n/locale.svelte';
	import * as m from '$lib/paraglide/messages';

	// `class` lets a host stretch the trigger (e.g. w-full in the admin sidebar);
	// the header leaves it content-sized.
	let { class: cls = '' }: { class?: string } = $props();

	const active = currentLang();

	async function choose(next: DisplayLang) {
		if (next === currentLang()) return;
		try {
			await putPreferences({ language: next });
		} catch {
			// offline/unauthenticated: localStorage still carries the choice
		}
		setDisplayLang(next); // persists + reloads
	}
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger
		class="inline-flex h-9 items-center justify-between gap-2 rounded-full border border-edge
			bg-surface px-3 text-xs font-medium text-text transition-colors hover:border-faint {cls}"
		aria-label={m.language_label()}
	>
		<span class="flex items-center gap-2">
			<Flag code={active} />
			{displayLangLabel(active)}
		</span>
		<ChevronDown class="size-3.5 text-muted" />
	</DropdownMenu.Trigger>
	<DropdownMenu.Portal>
		<DropdownMenu.Content
			align="end"
			sideOffset={6}
			class="z-50 min-w-[8rem] animate-pop-in rounded-card border border-edge bg-surface-2 p-1
				shadow-xl shadow-black/40"
		>
			{#each displayLangs as lang (lang)}
				<DropdownMenu.Item
					class="flex cursor-pointer items-center gap-2 rounded-lg px-3 py-1.5 text-xs text-muted
						outline-none data-highlighted:bg-surface data-highlighted:text-text"
					onSelect={() => choose(lang)}
				>
					<Flag code={lang} />
					<span class="flex-1">{displayLangLabel(lang)}</span>
					{#if lang === active}<Check class="size-3.5 text-accent" />{/if}
				</DropdownMenu.Item>
			{/each}
		</DropdownMenu.Content>
	</DropdownMenu.Portal>
</DropdownMenu.Root>
