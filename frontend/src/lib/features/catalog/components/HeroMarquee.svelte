<script lang="ts">
	import { goto } from '$app/navigation';
	import { Check, Play, Plus } from 'lucide-svelte';
	import { fly } from 'svelte/transition';
	import { toast } from 'svelte-sonner';
	import * as catalog from '$lib/features/catalog/api';
	import type { FeaturedItem } from '$lib/features/catalog/types';
	import GlowBackdrop from '$lib/components/layout/GlowBackdrop.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import { accentVars } from '$lib/theme';

	let { items }: { items: FeaturedItem[] } = $props();

	const SLIDE_MS = 8000;
	const STEP_MS = 50;

	let index = $state(0);
	let progress = $state(0); // 0..1 within the current slide
	let paused = $state(false);
	// slight scroll parallax: the banner drifts slower than the page
	let scrollY = $state(0);
	const parallax = $derived(`translate3d(0, ${scrollY * 0.18}px, 0) scale(1.12)`);
	// optimistic My List state per slide, keyed by title id
	let listedOverrides = $state<Record<string, boolean>>({});

	// clamp the index if the featured set shrinks between loads
	$effect(() => {
		if (index >= items.length) index = 0;
	});

	const active = $derived(items[index] ?? items[0]);
	const listed = $derived(listedOverrides[active.id] ?? active.inList);
	const eyebrow = $derived(['Featured', ...active.genres.slice(0, 2)].join(' · '));

	// single ticking timer drives both the progress indicator and auto-advance,
	// so they never drift apart
	$effect(() => {
		if (items.length <= 1) return;
		const timer = setInterval(() => {
			if (paused) return;
			progress += STEP_MS / SLIDE_MS;
			if (progress >= 1) {
				progress = 0;
				index = (index + 1) % items.length;
			}
		}, STEP_MS);
		return () => clearInterval(timer);
	});

	function goTo(i: number) {
		index = (i + items.length) % items.length;
		progress = 0;
	}

	// accent the hero subtree from the active banner's server-extracted colour,
	// so the eyebrow and buttons echo the featured artwork
	const accentStyle = $derived(active.backdropAccent ? accentVars(active.backdropAccent) : '');

	function play() {
		if (active.kind === 'movie') goto(`/watch/movie/${active.id}`);
		else goto(`/title/${active.slug}`);
	}

	async function toggleList() {
		const id = active.id;
		const next = !listed;
		try {
			if (next) await catalog.addToList(id);
			else await catalog.removeFromList(id);
			listedOverrides = { ...listedOverrides, [id]: next };
		} catch {
			toast.error('Failed to update My List');
		}
	}
</script>

<svelte:window bind:scrollY />

<div
	class="relative flex min-h-[72vh] items-center justify-center overflow-hidden md:min-h-[82vh]"
	style={accentStyle}
	role="region"
	aria-roledescription="carousel"
	aria-label="Featured titles"
	onpointerenter={() => (paused = true)}
	onpointerleave={() => (paused = false)}
>
	<!-- backdrops crossfade between slides -->
	{#if items.some((it) => it.backdropId)}
		{#each items as item, i (item.id)}
			{#if item.backdropId}
				<img
					src={catalog.artworkUrl(item.backdropId, item.backdropVer)}
					alt=""
					class="absolute inset-0 size-full object-cover transition-opacity duration-700
						{i === index ? 'opacity-100' : 'opacity-0'}"
					style="transform: {parallax}"
				/>
			{/if}
		{/each}
		<!-- legibility scrims: the hero text is white, so even bright banners
		     get a bottom fade into the page plus a dark vignette behind the text -->
		<div class="absolute inset-0 bg-gradient-to-t from-bg via-bg/45 to-black/25"></div>
		<div
			class="absolute inset-0"
			style="background: radial-gradient(ellipse 60% 55% at 50% 55%, rgb(0 0 0 / 0.55), transparent 75%)"
		></div>
	{:else}
		<GlowBackdrop />
	{/if}

	{#key active.id}
		<div
			class="relative mx-auto max-w-3xl px-6 pt-24 pb-20 text-center
				{active.backdropId ? '[text-shadow:0_2px_18px_rgb(0_0_0/0.55)]' : ''}"
		>
			<p in:fly={{ y: 12, duration: 400, delay: 100 }} class="eyebrow mb-6">{eyebrow}</p>
			<h1
				in:fly={{ y: 16, duration: 450, delay: 200 }}
				class="text-[clamp(2.2rem,7vw,5.5rem)] leading-[1.05] font-extrabold tracking-tight"
			>
				{active.name}
			</h1>
			{#if active.overview}
				<p
					in:fly={{ y: 14, duration: 450, delay: 320 }}
					class="mx-auto mt-5 line-clamp-2 max-w-xl text-[15px] text-muted italic"
				>
					“{active.overview}”
				</p>
			{/if}

			<div
				in:fly={{ y: 14, duration: 450, delay: 420 }}
				class="mt-6 flex items-center justify-center gap-2 text-xs text-muted"
			>
				{#if active.year}<span class="rounded-md border border-edge bg-surface/60 px-2 py-1"
						>{active.year}</span
					>{/if}
				{#if active.contentRating}<span
						class="rounded-md border border-edge bg-surface/60 px-2 py-1"
						>{active.contentRating}</span
					>{/if}
				<span class="rounded-md border border-edge bg-surface/60 px-2 py-1 capitalize"
					>{active.kind}</span
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
	{/key}

	{#if items.length > 1}
		<!-- prev/next + story-style segmented progress; the controls sit above the
		     content and have a tall hit area so the thin bars stay easy to click -->
		<div class="absolute inset-x-0 bottom-7 z-20 flex items-center justify-center gap-4">
			<button
				type="button"
				class="flex size-8 items-center justify-center rounded-full bg-black/40 text-white/80
					backdrop-blur transition-colors hover:bg-black/60 hover:text-white"
				aria-label="Previous featured"
				onclick={() => goTo(index - 1)}
			>
				<Play class="size-3.5 rotate-180 fill-current" />
			</button>

			<div class="flex items-center gap-2">
				{#each items as item, i (item.id)}
					<button
						type="button"
						class="group/seg flex h-6 items-center"
						aria-label="Show featured {i + 1}: {item.name}"
						aria-current={i === index}
						onclick={() => goTo(i)}
					>
						<span
							class="h-1.5 w-10 overflow-hidden rounded-full bg-white/25 transition-all
								group-hover/seg:bg-white/40 {i === index ? 'w-14' : ''}"
						>
							<span
								class="block h-full rounded-full bg-accent"
								style="width: {i === index ? progress * 100 : i < index ? 100 : 0}%"
							></span>
						</span>
					</button>
				{/each}
			</div>

			<button
				type="button"
				class="flex size-8 items-center justify-center rounded-full bg-black/40 text-white/80
					backdrop-blur transition-colors hover:bg-black/60 hover:text-white"
				aria-label="Next featured"
				onclick={() => goTo(index + 1)}
			>
				<Play class="size-3.5 fill-current" />
			</button>
		</div>
	{/if}
</div>
