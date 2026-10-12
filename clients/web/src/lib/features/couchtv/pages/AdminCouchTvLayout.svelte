<script lang="ts">
	import type { Snippet } from 'svelte';
	import { page } from '$app/state';
	import { features } from '$lib/features/settings/features.svelte';
	import * as m from '$lib/paraglide/messages';

	let { children }: { children: Snippet } = $props();

	const tabs = $derived([
		{ href: '/admin/couch-tv', label: m.admin_couchtv_tab_schedule() },
		{ href: '/admin/couch-tv/template', label: m.admin_couchtv_tab_template() },
		{ href: '/admin/couch-tv/render', label: m.admin_couchtv_tab_render() },
		{ href: '/admin/couch-tv/preview', label: m.admin_couchtv_tab_preview() }
	]);

	const current = (href: string) =>
		href === '/admin/couch-tv' ? page.url.pathname === href : page.url.pathname.startsWith(href);
</script>

<div class="mb-5 flex flex-wrap items-center justify-between gap-3">
	<h1 class="text-2xl font-bold">{m.admin_nav_couch_tv()}</h1>
	{#if !features.couchTvEnabled}
		<a
			href="/admin/settings"
			class="rounded-full border border-edge bg-surface px-3 py-1 text-xs font-medium text-muted transition-colors hover:border-faint hover:text-text"
		>
			{m.admin_couchtv_off_air()}
		</a>
	{/if}
</div>

<nav
	aria-label={m.admin_couchtv_sections()}
	class="mb-6 inline-flex max-w-full flex-wrap items-center gap-1 rounded-full border border-edge bg-surface p-1"
>
	{#each tabs as tab (tab.href)}
		<a
			href={tab.href}
			aria-current={current(tab.href) ? 'page' : undefined}
			class="rounded-full px-4 py-1.5 text-xs font-semibold transition-colors
				{current(tab.href) ? 'bg-accent-soft text-text' : 'text-muted hover:text-text'}"
		>
			{tab.label}
		</a>
	{/each}
</nav>

{@render children()}
