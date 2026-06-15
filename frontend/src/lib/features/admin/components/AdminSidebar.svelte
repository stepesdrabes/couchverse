<script lang="ts">
	import { page } from '$app/state';
	import { ArrowLeft, Database, LayoutDashboard, Library, Settings, Users } from 'lucide-svelte';
	import { fly } from 'svelte/transition';
	import { session } from '$lib/features/auth/session.svelte';
	import LanguageSwitcher from '$lib/components/layout/LanguageSwitcher.svelte';
	import LogoMark from '$lib/components/ui/LogoMark.svelte';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import StorageMeter from './StorageMeter.svelte';
	import SystemMeter from './SystemMeter.svelte';
	import * as m from '$lib/paraglide/messages';

	const items = $derived([
		{ href: '/admin', label: m.admin_nav_overview(), icon: LayoutDashboard },
		{ href: '/admin/library', label: m.admin_nav_library(), icon: Library },
		{ href: '/admin/users', label: m.admin_nav_users(), icon: Users },
		{ href: '/admin/jobs', label: m.admin_nav_jobs(), icon: Database },
		{ href: '/admin/settings', label: m.admin_nav_settings(), icon: Settings }
	]);

	const isActive = (href: string) =>
		href === '/admin' ? page.url.pathname === '/admin' : page.url.pathname.startsWith(href);
</script>

<aside
	in:fly={{ x: -32, duration: 380, opacity: 0 }}
	class="sticky top-0 flex h-dvh w-60 shrink-0 flex-col self-start border-r border-edge bg-surface/40"
	style="view-transition-name: admin-sidebar"
>
	<a href="/admin" class="flex items-center gap-2.5 px-6 pt-6 pb-4">
		<LogoMark class="size-8 rounded-md" />
		<span class="text-lg font-extrabold tracking-tight">
			couch<span class="text-accent">verse</span>
		</span>
	</a>

	<a
		href="/"
		class="mx-3 mb-3 flex items-center gap-2 rounded-full px-4 py-2 text-xs font-medium text-faint
			transition-colors hover:bg-surface-2 hover:text-text"
	>
		<ArrowLeft class="size-3.5" />
		{m.admin_back_to_app()}
	</a>

	<nav class="flex flex-1 flex-col gap-1 px-3">
		{#each items as item, i (item.href)}
			<a
				in:fly|global={{ x: -16, duration: 320, delay: 120 + i * 45 }}
				href={item.href}
				class="flex items-center gap-3 rounded-full px-4 py-2 text-[13px] font-medium transition-colors
					{isActive(item.href)
					? 'bg-accent-soft text-text'
					: 'text-muted hover:bg-surface-2 hover:text-text'}"
			>
				<item.icon class="size-4" />
				{item.label}
			</a>
		{/each}
	</nav>

	<div class="space-y-3 p-4">
		<LanguageSwitcher />
		<StorageMeter />
		<SystemMeter />
		<a
			href="/profile"
			class="flex items-center gap-3 rounded-card border border-edge bg-surface p-3 transition-colors hover:border-faint"
		>
			<UserAvatar
				name={session.user?.displayName ?? '?'}
				avatarId={session.user?.avatarId}
				seed={session.user?.username}
				class="size-8 rounded-lg text-xs"
			/>
			<div class="min-w-0">
				<p class="truncate text-xs font-semibold">{session.user?.displayName}</p>
				<p class="text-[10px] text-faint">{m.admin_owner()}</p>
			</div>
		</a>
	</div>
</aside>
