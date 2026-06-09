<script lang="ts">
	import { goto } from '$app/navigation';
	import { Check, Play, Plus } from 'lucide-svelte';
	import { fly } from 'svelte/transition';
	import { toast } from 'svelte-sonner';
	import * as catalog from '$lib/features/catalog/api';
	import type { ArtworkRef, Title } from '$lib/features/catalog/types';
	import GlowBackdrop from '$lib/components/layout/GlowBackdrop.svelte';
	import Button from '$lib/components/ui/Button.svelte';

	let {
		featured,
		backdrop = null,
		inList = false
	}: { featured: Title; backdrop?: ArtworkRef | null; inList?: boolean } = $props();

	let listed = $state(inList);

	const eyebrow = $derived(['Featured', ...featured.genres.slice(0, 2)].join(' · ').toUpperCase());

	function play() {
		if (featured.kind === 'movie') goto(`/watch/movie/${featured.id}`);
		else goto(`/title/${featured.id}`);
	}

	async function toggleList() {
		try {
			if (listed) await catalog.removeFromList(featured.id);
			else await catalog.addToList(featured.id);
			listed = !listed;
		} catch {
			toast.error('Failed to update My List');
		}
	}
</script>

<div class="relative flex min-h-[72vh] items-center justify-center overflow-hidden md:min-h-[82vh]">
	{#if backdrop}
		<img
			src={catalog.artworkUrl(backdrop.id)}
			alt=""
			class="absolute inset-0 size-full object-cover opacity-40"
		/>
		<div class="absolute inset-0 bg-gradient-to-t from-bg via-bg/40 to-bg/70"></div>
	{:else}
		<GlowBackdrop />
	{/if}

	<div class="relative mx-auto max-w-3xl px-6 pt-24 pb-16 text-center">
		<p in:fly={{ y: 12, duration: 400, delay: 100 }} class="eyebrow mb-6">{eyebrow}</p>
		<h1
			in:fly={{ y: 16, duration: 450, delay: 200 }}
			class="text-[clamp(2.2rem,7vw,5.5rem)] leading-none font-extrabold tracking-[0.18em]
				uppercase"
		>
			{featured.name}
		</h1>
		{#if featured.overview}
			<p
				in:fly={{ y: 14, duration: 450, delay: 320 }}
				class="mx-auto mt-5 line-clamp-2 max-w-xl text-[15px] text-muted italic"
			>
				“{featured.overview}”
			</p>
		{/if}

		<div
			in:fly={{ y: 14, duration: 450, delay: 420 }}
			class="mt-6 flex items-center justify-center gap-2 text-xs text-muted"
		>
			{#if featured.year}<span class="rounded-md border border-edge bg-surface/60 px-2 py-1"
					>{featured.year}</span
				>{/if}
			{#if featured.contentRating}<span
					class="rounded-md border border-edge bg-surface/60 px-2 py-1"
					>{featured.contentRating}</span
				>{/if}
			<span class="rounded-md border border-edge bg-surface/60 px-2 py-1 capitalize"
				>{featured.kind}</span
			>
		</div>

		<div
			in:fly={{ y: 14, duration: 450, delay: 520 }}
			class="mt-8 flex items-center justify-center gap-3"
		>
			<Button size="lg" onclick={play}>
				<Play class="size-4 fill-current" />
				Play
			</Button>
			<Button variant="secondary" size="lg" onclick={toggleList}>
				{#if listed}
					<Check class="size-4" />
				{:else}
					<Plus class="size-4" />
				{/if}
				My List
			</Button>
		</div>
	</div>
</div>
