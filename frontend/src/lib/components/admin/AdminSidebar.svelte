<script lang="ts">
	import { page } from '$app/state';
	import {
		ArrowLeft,
		Database,
		LayoutDashboard,
		Library,
		Settings,
		Upload,
		Users
	} from 'lucide-svelte';
	import { session } from '$lib/state/session.svelte';

	const items = [
		{ href: '/admin', label: 'Overview', icon: LayoutDashboard },
		{ href: '/admin/library', label: 'Library', icon: Library },
		{ href: '/admin/uploads', label: 'Uploads', icon: Upload },
		{ href: '/admin/users', label: 'Users', icon: Users },
		{ href: '/admin/jobs', label: 'Jobs & Storage', icon: Database },
		{ href: '/admin/settings', label: 'Settings', icon: Settings }
	];

	const isActive = (href: string) =>
		href === '/admin' ? page.url.pathname === '/admin' : page.url.pathname.startsWith(href);
</script>

<aside class="flex h-dvh w-60 shrink-0 flex-col border-r border-edge bg-surface/40">
	<a href="/admin" class="px-6 pt-6 pb-7">
		<span class="text-lg font-extrabold tracking-tight">
			couch<span class="text-accent">verse</span>
		</span>
		<span class="mt-0.5 block text-[10px] font-semibold tracking-[0.25em] text-faint uppercase">
			Server admin
		</span>
	</a>

	<nav class="flex flex-1 flex-col gap-1 px-3">
		{#each items as item (item.href)}
			<a
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
		<a
			href="/"
			class="flex items-center gap-2 px-2 text-xs font-medium text-faint transition-colors hover:text-text"
		>
			<ArrowLeft class="size-3.5" />
			Back to app
		</a>
		<div class="flex items-center gap-3 rounded-card border border-edge bg-surface p-3">
			<span
				class="flex size-8 items-center justify-center rounded-lg bg-danger/80 text-xs font-bold text-white uppercase"
			>
				{session.user?.displayName?.[0] ?? '?'}
			</span>
			<div class="min-w-0">
				<p class="truncate text-xs font-semibold">{session.user?.displayName}</p>
				<p class="text-[10px] text-faint">Owner</p>
			</div>
		</div>
	</div>
</aside>
