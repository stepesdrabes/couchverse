<script lang="ts">
	import { page } from '$app/state';
	import { DropdownMenu } from 'bits-ui';
	import { LogOut, Search, Shield, UserRound } from 'lucide-svelte';
	import { session } from '$lib/features/auth/session.svelte';
	import { features } from '$lib/features/settings/features.svelte';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';

	const items = $derived(
		[
			{ href: '/', label: 'Home' },
			{ href: '/series', label: 'Series' },
			{ href: '/movies', label: 'Movies' },
			{ href: '/music', label: 'Music' },
			{ href: '/my-list', label: 'My List' },
			{ href: '/genres', label: 'Genres' }
		].filter((item) => item.href !== '/music' || features.musicEnabled)
	);

	let scrollY = $state(0);
	const scrolled = $derived(scrollY > 24);

	const isActive = (href: string) =>
		href === '/' ? page.url.pathname === '/' : page.url.pathname.startsWith(href);
</script>

<svelte:window bind:scrollY />

<header
	class="fixed inset-x-0 top-0 z-40 transition-all duration-300
		{scrolled ? 'bg-bg/85 backdrop-blur-md' : 'bg-gradient-to-b from-bg/80 to-transparent'}"
	style="view-transition-name: top-nav"
>
	<div class="mx-auto flex h-16 max-w-[1700px] items-center gap-6 px-6">
		<a href="/" class="text-lg font-extrabold tracking-tight">
			couch<span class="text-accent">verse</span>
		</a>

		<nav
			class="absolute left-1/2 hidden -translate-x-1/2 items-center gap-1 rounded-full border
				border-edge/60 bg-surface/60 p-1 backdrop-blur md:flex"
		>
			{#each items as item (item.href)}
				<a
					href={item.href}
					class="rounded-full px-4 py-1.5 text-xs font-semibold transition-colors
						{isActive(item.href) ? 'bg-accent text-white' : 'text-muted hover:text-text'}"
				>
					{item.label}
				</a>
			{/each}
		</nav>

		<div class="ml-auto flex items-center gap-2">
			<a
				href="/search"
				class="rounded-full p-2 text-muted transition-colors hover:bg-surface-2 hover:text-text"
				title="Search"
			>
				<Search class="size-4.5" />
			</a>

			<DropdownMenu.Root>
				<DropdownMenu.Trigger
					class="overflow-hidden rounded-lg transition-transform hover:scale-105"
					aria-label="Account menu"
				>
					<UserAvatar
						name={session.user?.displayName ?? '?'}
						avatarId={session.user?.avatarId}
						class="size-8 rounded-lg text-xs"
					/>
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
									Profile
								</a>
							{/snippet}
						</DropdownMenu.Item>
						{#if session.isAdmin}
							<DropdownMenu.Item
								class="flex cursor-pointer items-center gap-2 rounded-lg px-3 py-2 text-sm text-muted
									outline-none data-highlighted:bg-surface data-highlighted:text-text"
							>
								{#snippet child({ props })}
									<a {...props} href="/admin">
										<Shield class="size-4" />
										Server admin
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
							Sign out
						</DropdownMenu.Item>
					</DropdownMenu.Content>
				</DropdownMenu.Portal>
			</DropdownMenu.Root>
		</div>
	</div>
</header>
