<script lang="ts">
	import { Eye, EyeOff, Pencil, Plus, Search, Trash2 } from 'lucide-svelte';
	import { SvelteSet } from 'svelte/reactivity';
	import { fly } from 'svelte/transition';
	import { toast } from 'svelte-sonner';
	import * as libraryApi from '$lib/features/library/api';
	import type { LibraryRow } from '$lib/features/library/api';
	import AdminMusicTable from '$lib/components/admin/AdminMusicTable.svelte';
	import NewTitleModal from '$lib/components/admin/NewTitleModal.svelte';
	import Artwork from '$lib/components/media/Artwork.svelte';
	import Badge from '$lib/components/ui/Badge.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Checkbox from '$lib/components/ui/Checkbox.svelte';
	import Confirm from '$lib/components/ui/Confirm.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import StatusPill from '$lib/components/ui/StatusPill.svelte';
	import Tabs from '$lib/components/ui/Tabs.svelte';
	import { formatBytes, formatDate, qualityLabel } from '$lib/utils/format';

	let items = $state<LibraryRow[]>([]);
	let total = $state(0);
	let loading = $state(true);

	let kind = $state('');
	let status = $state('');
	let sort = $state('added');
	let query = $state('');
	const selected = new SvelteSet<number>();

	let createOpen = $state(false);
	let confirmDelete = $state(false);

	let searchTimer: ReturnType<typeof setTimeout>;
	// debounced copy of the search box, shared with the music table
	let searchQuery = $state('');

	const musicTab = $derived(kind === 'music');

	async function refresh() {
		if (musicTab) return; // the music table fetches its own data
		loading = true;
		try {
			const res = await libraryApi.listLibrary({ type: kind, status, sort, q: query });
			items = res.items;
			total = res.total;
			for (const id of [...selected]) {
				if (!items.some((i) => i.id === id)) selected.delete(id);
			}
		} catch {
			toast.error('Failed to load library');
		} finally {
			loading = false;
		}
	}

	$effect(() => {
		// re-fetch when any filter changes
		void kind;
		void status;
		void sort;
		refresh();
	});

	function onSearchInput() {
		clearTimeout(searchTimer);
		searchTimer = setTimeout(() => {
			searchQuery = query;
			refresh();
		}, 250);
	}

	function toggle(id: number, on: boolean) {
		if (on) selected.add(id);
		else selected.delete(id);
	}

	function toggleAll(on: boolean) {
		selected.clear();
		if (on) items.forEach((i) => selected.add(i.id));
	}

	async function bulk(action: 'publish' | 'hide' | 'delete' | 'rescan') {
		const labels = {
			publish: 'published',
			hide: 'hidden',
			delete: 'deleted',
			rescan: 'queued for re-scan'
		};
		try {
			await libraryApi.bulkTitles([...selected], action);
			toast.success(`${selected.size} title${selected.size > 1 ? 's' : ''} ${labels[action]}`);
			selected.clear();
			refresh();
		} catch {
			toast.error('Bulk action failed');
		}
	}

	async function quickToggleVisibility(row: LibraryRow) {
		const next = row.status === 'published' ? 'hidden' : 'published';
		try {
			await libraryApi.updateTitle(row.id, { status: next });
			refresh();
		} catch {
			toast.error('Failed to update status');
		}
	}

	async function deleteOne(row: LibraryRow) {
		try {
			await libraryApi.deleteTitle(row.id);
			toast.success(`Deleted “${row.name}”`);
			refresh();
		} catch {
			toast.error('Failed to delete');
		}
	}

	let rowPendingDelete = $state<LibraryRow | null>(null);
	let confirmRowDelete = $state(false);

	const subtitle = (row: LibraryRow) =>
		row.kind === 'series'
			? `${row.seasonCount} season${row.seasonCount === 1 ? '' : 's'} · ${row.episodeCount} eps`
			: 'Movie';
</script>

<svelte:head>
	<title>Library — Couchverse admin</title>
</svelte:head>

<div class="mb-6 flex items-center gap-4">
	<h1 class="text-2xl font-bold">Library</h1>
	<span
		class="rounded-full border border-edge bg-surface px-2.5 py-0.5 text-xs font-semibold text-muted tnum"
	>
		{total} titles
	</span>
	<div class="ml-auto flex items-center gap-3">
		<div class="relative">
			<Search class="absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-faint" />
			<input
				bind:value={query}
				oninput={onSearchInput}
				placeholder="Search library..."
				class="h-9 w-64 rounded-full border border-edge bg-surface pr-4 pl-10 text-sm transition-colors
					placeholder:text-faint focus:border-accent focus:outline-none"
			/>
		</div>
		{#if !musicTab}
			<Button size="sm" onclick={() => (createOpen = true)}>
				<Plus class="size-4" />
				New title
			</Button>
		{/if}
	</div>
</div>

<div class="mb-4 flex flex-wrap items-center gap-3">
	<Tabs
		bind:value={kind}
		items={[
			{ value: '', label: 'All' },
			{ value: 'series', label: 'Series' },
			{ value: 'movie', label: 'Movies' },
			{ value: 'music', label: 'Music' }
		]}
	/>
	{#if !musicTab}
		<Select
			bind:value={status}
			label="Status"
			items={[
				{ value: '', label: 'Any' },
				{ value: 'draft', label: 'Draft' },
				{ value: 'processing', label: 'Processing' },
				{ value: 'published', label: 'Published' },
				{ value: 'hidden', label: 'Hidden' }
			]}
		/>
	{/if}
	<Select
		bind:value={sort}
		label="Sort"
		items={[
			{ value: 'added', label: 'Recently added' },
			{ value: 'name', label: 'Name' },
			{ value: 'year', label: 'Year' },
			{ value: 'size', label: 'Size' }
		]}
	/>
</div>

{#if musicTab}
	<AdminMusicTable query={searchQuery} {sort} />
{:else}
	<div class="overflow-hidden rounded-card border border-edge bg-surface/40">
		<table class="w-full text-left text-sm">
			<thead>
				<tr class="border-b border-edge text-[11px] tracking-wider text-faint uppercase">
					<th class="w-12 px-4 py-3">
						<Checkbox
							checked={items.length > 0 && selected.size === items.length}
							onCheckedChange={toggleAll}
						/>
					</th>
					<th class="py-3 pr-4 font-semibold">Title</th>
					<th class="py-3 pr-4 font-semibold">Type</th>
					<th class="py-3 pr-4 font-semibold">Quality</th>
					<th class="py-3 pr-4 font-semibold">Size</th>
					<th class="py-3 pr-4 font-semibold">Added</th>
					<th class="py-3 pr-4 font-semibold">Status</th>
					<th class="w-28 py-3 pr-4"></th>
				</tr>
			</thead>
			<tbody>
				{#each items as row (row.id)}
					<tr
						class="border-b border-edge/50 transition-colors last:border-0
						{selected.has(row.id) ? 'bg-accent-soft/30' : 'hover:bg-surface-2/40'}"
					>
						<td class="px-4 py-3">
							<Checkbox
								checked={selected.has(row.id)}
								onCheckedChange={(on) => toggle(row.id, on)}
							/>
						</td>
						<td class="py-3 pr-4">
							<a href="/admin/library/{row.id}" class="group flex items-center gap-3">
								<span class="block h-10 w-16 shrink-0 overflow-hidden rounded-md">
									<Artwork artworkId={row.backdropId ?? row.posterId} name={row.name} />
								</span>
								<span class="min-w-0">
									<span class="block truncate font-semibold group-hover:text-accent">
										{row.name}
									</span>
									<span class="block text-xs text-faint">{subtitle(row)}</span>
								</span>
							</a>
						</td>
						<td class="py-3 pr-4 text-muted capitalize">{row.kind}</td>
						<td class="py-3 pr-4">
							<span class="flex gap-1">
								{#if qualityLabel(row.maxHeight)}
									<Badge>{qualityLabel(row.maxHeight)}</Badge>
								{/if}
								{#if row.hdr}
									<Badge>HDR</Badge>
								{/if}
								{#if !qualityLabel(row.maxHeight)}
									<span class="text-xs text-faint">no files</span>
								{/if}
							</span>
						</td>
						<td class="py-3 pr-4 text-muted tnum">{formatBytes(row.sizeBytes)}</td>
						<td class="py-3 pr-4 text-muted tnum">{formatDate(row.addedAt)}</td>
						<td class="py-3 pr-4"><StatusPill status={row.status} /></td>
						<td class="py-3 pr-4">
							<span class="flex justify-end gap-1">
								<a
									href="/admin/library/{row.id}"
									class="rounded-full p-2 text-muted transition-colors hover:bg-surface-2 hover:text-text"
									title="Edit"
								>
									<Pencil class="size-3.5" />
								</a>
								<button
									class="rounded-full p-2 text-muted transition-colors hover:bg-surface-2 hover:text-text"
									title={row.status === 'published' ? 'Hide' : 'Publish'}
									onclick={() => quickToggleVisibility(row)}
								>
									{#if row.status === 'published'}
										<EyeOff class="size-3.5" />
									{:else}
										<Eye class="size-3.5" />
									{/if}
								</button>
								<button
									class="rounded-full p-2 text-muted transition-colors hover:bg-danger/15 hover:text-danger"
									title="Delete"
									onclick={() => {
										rowPendingDelete = row;
										confirmRowDelete = true;
									}}
								>
									<Trash2 class="size-3.5" />
								</button>
							</span>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>

		{#if !loading && items.length === 0}
			<EmptyState
				title="Nothing here yet"
				message="Create a title or scan a media folder to fill the library."
			>
				<Button size="sm" onclick={() => (createOpen = true)}>
					<Plus class="size-4" />
					New title
				</Button>
			</EmptyState>
		{/if}
	</div>
{/if}

{#if selected.size > 0 && !musicTab}
	<div
		transition:fly={{ y: 24, duration: 200 }}
		class="fixed bottom-6 left-1/2 z-40 flex -translate-x-1/2 items-center gap-1 rounded-full
			border border-edge bg-surface-2/95 px-2 py-1.5 shadow-2xl shadow-black/50 backdrop-blur"
	>
		<span class="px-3 text-xs font-semibold text-accent tnum">{selected.size} selected</span>
		<span class="h-5 w-px bg-edge"></span>
		<Button variant="ghost" size="sm" onclick={() => bulk('publish')}>Publish</Button>
		<Button variant="ghost" size="sm" onclick={() => bulk('hide')}>Hide</Button>
		<Button variant="ghost" size="sm" onclick={() => bulk('rescan')}>Re-scan</Button>
		<Button variant="danger" size="sm" onclick={() => (confirmDelete = true)}>
			<Trash2 class="size-3.5" />
			Delete
		</Button>
	</div>
{/if}

<NewTitleModal bind:open={createOpen} />

<Confirm
	bind:open={confirmDelete}
	title="Delete {selected.size} title{selected.size > 1 ? 's' : ''}?"
	message="This removes the titles and their metadata from the library. Media files on disk are not touched."
	onconfirm={() => bulk('delete')}
/>

<Confirm
	bind:open={confirmRowDelete}
	title="Delete “{rowPendingDelete?.name}”?"
	message="This removes the title and its metadata from the library. Media files on disk are not touched."
	onconfirm={() => {
		if (rowPendingDelete) deleteOne(rowPendingDelete);
		rowPendingDelete = null;
	}}
/>
