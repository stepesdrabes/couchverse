<script lang="ts">
	import type { MediaFile } from '$lib/features/catalog/types';
	import type { SubtitleInfo } from '$lib/features/library/api';
	import Modal from '$lib/components/ui/Modal.svelte';
	import SubtitleManager from './SubtitleManager.svelte';

	let {
		open = $bindable(false),
		mediaFile,
		subtitles
	}: {
		open?: boolean;
		mediaFile: MediaFile | null;
		subtitles: SubtitleInfo[];
	} = $props();
</script>

{#if mediaFile}
	<Modal
		bind:open
		title="Subtitles"
		description={mediaFile.path.split('/').pop() ?? mediaFile.path}
	>
		<SubtitleManager {mediaFile} {subtitles} />
	</Modal>
{/if}
