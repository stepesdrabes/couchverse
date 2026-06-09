<script lang="ts">
	import { ArrowLeft } from 'lucide-svelte';
	import VideoPlayer from '$lib/components/player/VideoPlayer.svelte';

	let { data } = $props();
</script>

<svelte:head>
	<title>{data.info.display.title} — Couchverse</title>
</svelte:head>

{#if data.info.mode === 'direct'}
	{#key data.id}
		<VideoPlayer
			info={data.info}
			titleId={data.kind === 'movie' ? data.id : null}
			episodeId={data.kind === 'episode' ? data.id : null}
		/>
	{/key}
{:else}
	<div class="flex min-h-dvh flex-col items-center justify-center gap-4 px-6 text-center">
		<h1 class="text-xl font-bold">This file needs transcoding</h1>
		<p class="max-w-md text-sm text-muted">
			The source format isn’t playable in a browser yet. Server-side transcoding arrives with the
			HLS milestone — once it lands, this title will play here automatically.
		</p>
		<a
			href="/title/{data.info.display.titleId}"
			class="inline-flex h-10 items-center gap-2 rounded-full border border-edge bg-surface/60
				px-5 text-sm font-semibold transition-colors hover:border-faint hover:bg-surface-2"
		>
			<ArrowLeft class="size-4" />
			Back to title
		</a>
	</div>
{/if}
