<script lang="ts">
	import { page } from '$app/state';
	import { DropdownMenu } from 'bits-ui';
	import { LogOut, Search, Shield, Trophy, UserRound } from 'lucide-svelte';
	import { session } from '$lib/features/auth/session.svelte';
	import { features } from '$lib/features/settings/features.svelte';
	import { rank } from '$lib/features/ranks/rank.svelte';
	import RankRing from '$lib/features/ranks/components/RankRing.svelte';
	import { rankColor } from '$lib/features/ranks/tiers';
	import { tierName } from '$lib/features/ranks/labels';
	import CouchButton from '$lib/features/couch/components/CouchButton.svelte';
	import LogoMark from '$lib/components/ui/LogoMark.svelte';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import LanguageSwitcher from '$lib/components/layout/LanguageSwitcher.svelte';
	import { readableTextOn } from '$lib/theme';
	import * as m from '$lib/paraglide/messages';

	const items = $derived(
		[
			{ href: '/', label: m.nav_home() },
			{ href: '/series', label: m.nav_series() },
			{ href: '/movies', label: m.nav_movies() },
			{ href: '/music', label: m.nav_music() },
			{ href: '/my-list', label: m.nav_my_list() },
			{ href: '/genres', label: m.nav_genres() }
		].filter((item) => item.href !== '/music' || features.musicEnabled)
	);

	let scrollY = $state(0);
	const scrolled = $derived(scrollY > 24);

	// A brand-new account has no ring at all, so the badge never reads as an
	// empty broken gauge on a fresh install.
	const showRank = $derived(features.rankingsEnabled && (rank.summary?.xp ?? 0) > 0);
	const tier = $derived(rank.summary?.tier);
	const myProfileHref = $derived(
		session.user ? `/u/${encodeURIComponent(session.user.username)}` : ''
	);

	const isActive = (href: string) =>
		href === '/' ? page.url.pathname === '/' : page.url.pathname.startsWith(href);
</script>

<svelte:window bind:scrollY />

<header
	class="fixed inset-x-0 top-0 z-40 border-b transition-all duration-300
		{scrolled
		? 'border-edge/60 bg-bg/85 shadow-lg shadow-black/20 backdrop-blur-md'
		: 'border-transparent bg-transparent shadow-none'}"
	style="view-transition-name: top-nav"
>
	<!-- legibility scrim for white nav over a hero; fades out once scrolled, so
	     the solid bar fades in smoothly without swapping a gradient for a colour -->
	<div
		class="pointer-events-none absolute inset-0 bg-gradient-to-b from-bg/85 to-transparent
			transition-opacity duration-300 {scrolled ? 'opacity-0' : 'opacity-100'}"
	></div>

	<div class="relative mx-auto flex h-20 max-w-[1700px] items-center gap-6 px-6 lg:px-8">
		<a href="/" class="flex items-center gap-2.5">
			<LogoMark class="size-10 rounded-lg" />
			<span class="text-xl font-extrabold tracking-tight">
				couch<span class="text-accent">verse</span>
			</span>
		</a>

		<nav
			class="absolute left-1/2 hidden -translate-x-1/2 items-center gap-1 rounded-full border
				border-edge/60 bg-surface/70 p-1.5 shadow-lg shadow-black/10 backdrop-blur md:flex"
		>
			{#each items as item (item.href)}
				<a
					href={item.href}
					class="rounded-full px-4 py-2 text-[13px] font-semibold transition-colors
						{isActive(item.href)
						? 'bg-accent text-white shadow-sm shadow-accent/30'
						: 'text-muted hover:bg-surface-2 hover:text-text'}"
				>
					{item.label}
				</a>
			{/each}
		</nav>

		<div class="ml-auto flex items-center gap-2.5">
			<a
				href="/search"
				class="rounded-full p-2.5 text-text/90 transition-colors hover:bg-surface-2 hover:text-text"
				title={m.nav_search()}
			>
				<Search class="size-5" />
			</a>

			{#if features.rankingsEnabled}
				<a
					href="/leaderboard"
					class="rounded-full p-2.5 text-text/90 transition-colors hover:bg-surface-2 hover:text-text"
					title={m.nav_leaderboard()}
				>
					<Trophy class="size-5" />
				</a>
			{/if}

			<CouchButton
				triggerClass="rounded-full p-2.5 text-text/90 transition-colors hover:bg-surface-2 hover:text-text"
			/>

			<LanguageSwitcher />

			<DropdownMenu.Root>
				<!-- no overflow-hidden here: it would clip the rank ring, and
				     UserAvatar already clips its own image -->
				<DropdownMenu.Trigger
					class="relative flex size-12 shrink-0 items-center justify-center rounded-full
						transition-transform hover:scale-105"
					aria-label={m.nav_account_menu()}
				>
					{#if showRank && tier}
						<RankRing
							class="absolute inset-0"
							tier={tier.code}
							percent={rank.summary!.percent}
							size={48}
							stroke={3}
							pulse={rank.levelUps}
							label={m.rank_level({ level: tier.level })}
						/>
					{/if}
					<UserAvatar
						name={session.user?.displayName ?? '?'}
						avatarId={session.user?.avatarId}
						seed={session.user?.username}
						class="size-9 rounded-lg text-xs"
					/>
					{#if showRank && tier}
						<span
							class="pointer-events-none absolute -right-0.5 -bottom-0.5 flex h-4 min-w-4
								items-center justify-center rounded-full px-1 text-[10px] font-bold ring-2
								ring-bg tnum"
							style="background: {rankColor(tier.code)}; color: {readableTextOn(
								rankColor(tier.code)
							)}"
						>
							{tier.level}
						</span>
					{/if}
				</DropdownMenu.Trigger>
				<DropdownMenu.Portal>
					<DropdownMenu.Content
						align="end"
						sideOffset={8}
						class="z-50 w-52 animate-pop-in rounded-card border border-edge bg-surface-2 p-1 shadow-xl shadow-black/40"
					>
						<div class="border-b border-edge/60 px-3 py-2.5">
							<p class="truncate text-sm font-semibold">{session.user?.displayName}</p>
							<p class="text-[11px] text-faint">@{session.user?.username}</p>
							{#if showRank && tier}
								<p class="mt-1 text-[11px] font-semibold" style="color: {rankColor(tier.code)}">
									{m.rank_level({ level: tier.level })} · {tierName(tier.code)}
								</p>
							{/if}
						</div>
						<div class="md:hidden">
							{#each items as item (item.href)}
								<DropdownMenu.Item
									class="block cursor-pointer rounded-lg px-3 py-2 text-sm text-muted outline-none
										data-highlighted:bg-surface data-highlighted:text-text"
								>
									{#snippet child({ props })}
										<a {...props} href={item.href}>{item.label}</a>
									{/snippet}
								</DropdownMenu.Item>
							{/each}
							<div class="my-1 border-t border-edge/60"></div>
						</div>
						<DropdownMenu.Item
							class="flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-sm text-muted
								outline-none data-highlighted:bg-surface data-highlighted:text-text"
						>
							{#snippet child({ props })}
								<a {...props} href="/profile">
									<UserRound class="size-4" />
									{m.nav_profile()}
								</a>
							{/snippet}
						</DropdownMenu.Item>
						{#if features.rankingsEnabled && myProfileHref}
							<DropdownMenu.Item
								class="flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-sm text-muted
									outline-none data-highlighted:bg-surface data-highlighted:text-text"
							>
								{#snippet child({ props })}
									<a {...props} href={myProfileHref}>
										<Trophy class="size-4" />
										{m.nav_public_profile()}
									</a>
								{/snippet}
							</DropdownMenu.Item>
						{/if}
						{#if session.isAdmin}
							<DropdownMenu.Item
								class="flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-sm text-muted
									outline-none data-highlighted:bg-surface data-highlighted:text-text"
							>
								{#snippet child({ props })}
									<a {...props} href="/admin">
										<Shield class="size-4" />
										{m.nav_server_admin()}
									</a>
								{/snippet}
							</DropdownMenu.Item>
						{/if}
						<DropdownMenu.Item
							class="flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-sm text-muted
								outline-none data-highlighted:bg-surface data-highlighted:text-text"
							onSelect={() => session.logout()}
						>
							<LogOut class="size-4" />
							{m.nav_sign_out()}
						</DropdownMenu.Item>
					</DropdownMenu.Content>
				</DropdownMenu.Portal>
			</DropdownMenu.Root>
		</div>
	</div>
</header>
