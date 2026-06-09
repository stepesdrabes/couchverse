<script lang="ts">
	import { Clapperboard, Tv, Users } from 'lucide-svelte';
	import * as libraryApi from '$lib/features/library/api';
	import * as usersApi from '$lib/features/users/api';

	let movieCount = $state<number | null>(null);
	let seriesCount = $state<number | null>(null);
	let userCount = $state<number | null>(null);

	$effect(() => {
		libraryApi.listLibrary({ type: 'movie' }).then((r) => (movieCount = r.total));
		libraryApi.listLibrary({ type: 'series' }).then((r) => (seriesCount = r.total));
		usersApi.listUsers().then((u) => (userCount = u.length));
	});

	const cards = $derived([
		{ label: 'Movies', value: movieCount, icon: Clapperboard, href: '/admin/library?type=movie' },
		{ label: 'Series', value: seriesCount, icon: Tv, href: '/admin/library?type=series' },
		{ label: 'Users', value: userCount, icon: Users, href: '/admin/users' }
	]);
</script>

<svelte:head>
	<title>Overview — Couchverse admin</title>
</svelte:head>

<h1 class="mb-6 text-2xl font-bold">Overview</h1>

<div class="grid gap-4 sm:grid-cols-3">
	{#each cards as card (card.label)}
		<a
			href={card.href}
			class="group rounded-card border border-edge bg-surface/40 p-5 transition-colors hover:border-faint"
		>
			<card.icon class="mb-3 size-5 text-accent" />
			<p class="text-2xl font-bold tnum">{card.value ?? '—'}</p>
			<p class="text-xs font-medium text-muted group-hover:text-text">{card.label}</p>
		</a>
	{/each}
</div>

<p class="mt-8 text-xs text-faint">
	Storage stats, job queue and recent activity land here with the upcoming milestones.
</p>
