<script lang="ts">
	import { toast } from 'svelte-sonner';
	import type { MediaFile } from '$lib/features/catalog/types';
	import { setMediaFileAudio } from '$lib/features/library/api';
	import * as m from '$lib/paraglide/messages';

	// per-file audio-language tagging for model-B multi-language audio: mark a
	// file's language and whether it is the primary file or an alternate sibling.
	let { file }: { file: MediaFile } = $props();

	let lang = $state(file.audioLang ?? '');
	let role = $state(file.audioRole || 'primary');

	const LANGS = ['en', 'cs', 'sk', 'de', 'es', 'fr', 'it', 'pl', 'ko', 'ja'];

	async function persist() {
		try {
			await setMediaFileAudio(file.id, lang, role as 'primary' | 'audio_alt');
			toast.success(m.library_audio_saved());
		} catch {
			toast.error(m.common_save_failed());
		}
	}
</script>

<div class="mt-1.5 flex flex-wrap items-center gap-1.5 text-[11px] text-faint">
	<span>{m.library_audio_language()}</span>
	<select
		bind:value={lang}
		onchange={persist}
		class="rounded border border-edge bg-surface px-1.5 py-0.5 text-text"
	>
		<option value="">-</option>
		{#each LANGS as code (code)}<option value={code}>{code}</option>{/each}
	</select>
	<select
		bind:value={role}
		onchange={persist}
		class="rounded border border-edge bg-surface px-1.5 py-0.5 text-text"
	>
		<option value="primary">{m.library_audio_primary()}</option>
		<option value="audio_alt">{m.library_audio_alternate()}</option>
	</select>
</div>
