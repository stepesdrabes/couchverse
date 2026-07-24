<script lang="ts">
	import { fly, scale } from 'svelte/transition';
	import { backOut } from 'svelte/easing';
	import { KeyRound, Pencil } from 'lucide-svelte';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import Markdown from '$lib/components/ui/Markdown.svelte';
	import { artworkUrl } from '$lib/features/catalog/api';
	import { accentVars } from '$lib/theme';
	import { formatYearDate } from '$lib/utils/format';
	import RankRing from './RankRing.svelte';
	import XpBar from './XpBar.svelte';
	import CountUp from './CountUp.svelte';
	import { tierName } from '../labels';
	import { rankColor } from '../tiers';
	import * as m from '$lib/paraglide/messages';
	import type { Profile } from '../types';

	let {
		profile,
		leaderboardRank = null,
		onedit = undefined,
		onpassword = undefined
	}: {
		profile: Profile;
		leaderboardRank?: number | null;
		// owner affordances; absent on someone else's profile
		onedit?: () => void;
		onpassword?: () => void;
	} = $props();

	const tier = $derived(profile.rank.tier);
	// The banner's own extracted accent wins when there is one, so the card takes
	// the colour of the image the member chose; otherwise it falls back to the
	// tier colour and the hero still means something.
	const color = $derived(profile.user.bannerAccent || rankColor(tier.code));
	const banner = $derived(profile.user.bannerId);
</script>

<!-- accentVars scopes --color-accent* (and the contrast-aware --color-on-accent)
     to this card only, so the eyebrow, the xp bar and the buttons inside all take
     the hero colour for free and nothing outside is touched -->
<div
	class="group relative overflow-hidden rounded-card border border-edge bg-surface/40"
	style={accentVars(color)}
>
	{#if banner}
		<div class="absolute inset-0" aria-hidden="true">
			<img
				src="{artworkUrl(banner)}?size=w780"
				alt=""
				class="size-full scale-105 object-cover opacity-40 blur-[1px]"
			/>
			<!-- the text sits on the lower half, so the scrim is strongest there -->
			<div
				class="absolute inset-0 bg-gradient-to-t from-surface via-surface/85 to-surface/40"
			></div>
		</div>
	{:else}
		<div
			class="pointer-events-none absolute -top-28 -left-20 size-80 rounded-full opacity-[0.16] blur-[90px]"
			style="background: {color}"
			aria-hidden="true"
		></div>
	{/if}

	<div class="relative p-6 sm:p-8">
		<div
			class="flex flex-col items-center gap-6 text-center sm:flex-row sm:items-center sm:gap-8
				sm:text-left"
		>
			<span
				class="relative flex size-32 shrink-0 items-center justify-center"
				in:scale|global={{ duration: 480, start: 0.86, easing: backOut }}
			>
				<RankRing
					class="absolute inset-0"
					tier={tier.code}
					percent={profile.rank.percent}
					size={128}
					stroke={7}
					orbit
					label={m.rank_xp_progress({ into: profile.rank.intoTier, need: profile.rank.tierSpan })}
				/>
				<UserAvatar
					name={profile.user.displayName}
					avatarId={profile.user.avatarId}
					seed={profile.user.username}
					class="size-24 rounded-2xl text-3xl"
				/>
			</span>

			<div class="min-w-0 flex-1">
				<div in:fly|global={{ y: 12, duration: 400, delay: 80 }}>
					<p class="eyebrow mb-1">{tierName(tier.code)}</p>
					<h1 class="text-3xl font-extrabold tracking-tight sm:text-4xl">
						{profile.user.displayName}
					</h1>
					<p class="mt-1 text-sm text-faint">
						@{profile.user.username} · {m.profiles_joined({
							date: formatYearDate(profile.user.memberSince)
						})}
					</p>
				</div>

				<div in:fly|global={{ y: 12, duration: 400, delay: 160 }}>
					<div class="mt-4 flex items-baseline gap-3">
						<span class="text-3xl font-extrabold tracking-tight">
							<CountUp value={profile.xp.total} />
						</span>
						<span class="text-sm font-semibold text-muted">{m.rank_xp()}</span>
						<span class="ml-auto text-sm font-bold tnum">
							{m.rank_level({ level: tier.level })}
						</span>
					</div>
					<XpBar rank={profile.rank} class="mt-2" />
				</div>

				<div
					class="mt-4 flex flex-wrap items-center justify-center gap-2 sm:justify-start"
					in:fly|global={{ y: 12, duration: 400, delay: 240 }}
				>
					{#if leaderboardRank}
						<a
							href="/leaderboard"
							class="rounded-full border border-edge bg-surface px-2.5 py-0.5 text-xs font-semibold
								text-muted transition-colors hover:border-accent/50 hover:text-text tnum"
						>
							{m.profiles_leaderboard_rank({ rank: leaderboardRank })}
						</a>
					{/if}
					{#if onedit}
						<button
							type="button"
							onclick={onedit}
							class="flex items-center gap-1.5 rounded-full border border-edge bg-surface px-2.5
								py-0.5 text-xs font-semibold text-muted transition-colors hover:border-accent/50
								hover:text-text"
						>
							<Pencil class="size-3" />
							{m.profiles_edit_profile()}
						</button>
					{/if}
					{#if onpassword}
						<button
							type="button"
							onclick={onpassword}
							class="flex items-center gap-1.5 rounded-full border border-edge bg-surface px-2.5
								py-0.5 text-xs font-semibold text-muted transition-colors hover:border-accent/50
								hover:text-text"
						>
							<KeyRound class="size-3" />
							{m.profile_password_heading()}
						</button>
					{/if}
				</div>
			</div>
		</div>

		{#if profile.user.bio.trim()}
			<div
				class="mt-6 border-t border-edge/60 pt-5"
				in:fly|global={{ y: 12, duration: 400, delay: 320 }}
			>
				<Markdown source={profile.user.bio} />
			</div>
		{/if}
	</div>
</div>
