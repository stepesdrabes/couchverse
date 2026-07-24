<script lang="ts">
	import { fly } from 'svelte/transition';
	import {
		CalendarDays,
		CircleCheckBig,
		Clock,
		EyeOff,
		Flame,
		Headphones,
		ListVideo,
		Sofa,
		Tag
	} from 'lucide-svelte';
	import StatTile from '$lib/components/ui/StatTile.svelte';
	import RankedList from '$lib/components/ui/RankedList.svelte';
	import { features } from '$lib/features/settings/features.svelte';
	import { formatUptime } from '$lib/utils/format';
	import ProfileHero from './ProfileHero.svelte';
	import ActivityHeatmap from './ActivityHeatmap.svelte';
	import ActivityClock from './ActivityClock.svelte';
	import XpSourceCard from './XpSourceCard.svelte';
	import AchievementGrid from './AchievementGrid.svelte';
	import * as m from '$lib/paraglide/messages';
	import type { Profile } from '../types';

	let {
		profile,
		onedit = undefined,
		onpassword = undefined
	}: {
		profile: Profile;
		// owner affordances, threaded to the hero; absent on someone else's profile
		onedit?: () => void;
		onpassword?: () => void;
	} = $props();

	const t = $derived(profile.totals);

	// Music and couch tiles are gated on their flags, so a server with music off
	// never shows a permanent zero.
	const tiles = $derived(
		[
			{
				key: 'watch',
				icon: Clock,
				value: formatUptime(t.videoSeconds),
				label: m.profiles_stat_watch_time()
			},
			{
				key: 'titles',
				icon: CircleCheckBig,
				value: t.moviesCompleted + t.seriesCompleted,
				label: m.profiles_stat_titles()
			},
			{
				key: 'episodes',
				icon: ListVideo,
				value: t.episodesCompleted,
				label: m.profiles_stat_episodes()
			},
			{
				key: 'listening',
				icon: Headphones,
				value: formatUptime(t.musicSeconds),
				label: m.profiles_stat_listening(),
				hidden: !features.musicEnabled
			},
			{
				key: 'longest',
				icon: Flame,
				value: m.profiles_streak_days({ count: t.longestStreak }),
				label: m.profiles_stat_longest_streak()
			},
			{
				key: 'current',
				icon: CalendarDays,
				value: m.profiles_streak_days({ count: t.currentStreak }),
				label: m.profiles_stat_current_streak()
			},
			{
				key: 'genre',
				icon: Tag,
				value: profile.favouriteGenre || '-',
				label: m.profiles_stat_favourite_genre()
			},
			{
				key: 'couch',
				icon: Sofa,
				value: t.couchHosted,
				label: m.profiles_stat_couch(),
				hidden: !features.couchEnabled
			}
		].filter((tile) => !tile.hidden)
	);

	const topTitles = $derived(
		profile.topTitles.map((title) => ({
			key: title.slug,
			label: title.name,
			href: `/title/${title.slug}`,
			value: title.seconds,
			display: formatUptime(title.seconds)
		}))
	);
</script>

<svelte:head>
	<title>{m.profiles_page_title({ name: profile.user.displayName })}</title>
</svelte:head>

<div class="mx-auto max-w-5xl px-6 pt-28 pb-16">
	{#if profile.isSelf && !profile.public}
		<div
			class="mb-4 flex flex-wrap items-center gap-3 rounded-card border border-edge bg-surface-2/60
				px-4 py-3 text-xs text-muted"
		>
			<EyeOff class="size-4 shrink-0 text-faint" />
			<span>{m.profiles_private_self_notice()}</span>
			<a
				href="/profile?tab=privacy"
				class="ml-auto shrink-0 font-semibold text-accent hover:underline"
			>
				{m.profiles_make_public()}
			</a>
		</div>
	{/if}

	<ProfileHero {profile} {onedit} {onpassword} />

	<div class="mt-6 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
		{#each tiles as tile, i (tile.key)}
			<div in:fly|global={{ y: 16, duration: 350, delay: Math.min(i * 55, 300) }}>
				<StatTile icon={tile.icon} value={tile.value} label={tile.label} size="sm" />
			</div>
		{/each}
	</div>

	<div class="mt-6 grid gap-6 lg:grid-cols-3" in:fly|global={{ y: 20, duration: 400 }}>
		<div class="lg:col-span-2">
			<ActivityHeatmap activity={profile.activity} />
		</div>
		<XpSourceCard xp={profile.xp} />
	</div>

	<div class="mt-6 grid gap-6 lg:grid-cols-2" in:fly|global={{ y: 20, duration: 400, delay: 80 }}>
		<div class="rounded-card border border-edge bg-surface/40 p-6">
			<h2 class="mb-3 text-sm font-semibold text-muted">{m.profiles_top_titles_heading()}</h2>
			{#if topTitles.length === 0}
				<p class="py-6 text-center text-xs text-faint">{m.profiles_no_activity()}</p>
			{:else}
				<RankedList items={topTitles} />
			{/if}
		</div>
		<ActivityClock hours={profile.hours} />
	</div>

	<AchievementGrid achievements={profile.achievements} won={profile.achievementsWon} />
</div>
