<script lang="ts">
	import { Music, Pencil, Trash2 } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import { artworkUrl } from '$lib/features/catalog/api';
	import * as libraryApi from '$lib/features/library/api';
	import type { AdminAlbumRow } from '$lib/features/library/api';
	import Confirm from '$lib/components/ui/Confirm.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import StatusPill from '$lib/components/ui/StatusPill.svelte';
	import { formatBytes, formatDate } from '$lib/utils/format';
	import * as m from '$lib/paraglide/messages';

	let { query = '', sort = 'added' }: { query?: string; sort?: string } = $props();

	let items = $state<AdminAlbumRow[]>([]);
	let total = $state(0);
	let loaded = $state(false);
	let pendingDelete = $state<AdminAlbumRow | null>(null);
	let confirmDelete = $state(false);

	export async function refresh() {
		try {
			const res = await libraryApi.listAdminMusic({ q: query, sort });
			items = res.items;
			total = res.total;
		} catch {
			toast.error(m.library_albums_load_failed());
		} finally {
			loaded = true;
		}
	}

	$effect(() => {
		void query;
		void sort;
		refresh();
	});

	async function deleteOne() {
		if (!pendingDelete) return;
		try {
			await libraryApi.deleteAlbum(pendingDelete.id);
			toast.success(m.library_deleted_named({ name: pendingDelete.name }));
			refresh();
		} catch {
			toast.error(m.library_album_delete_failed());
		}
		pendingDelete = null;
	}
</script>

<p class="mb-2 text-xs text-faint tnum">{m.library_album_count({ count: total })}</p>
<div class="overflow-hidden rounded-card border border-edge bg-surface/40">
	<table class="w-full text-left text-sm">
		<thead>
			<tr class="border-b border-edge text-[11px] tracking-wider text-faint uppercase">
				<th class="px-4 py-3 font-semibold">{m.library_album()}</th>
				<th class="py-3 pr-4 font-semibold">{m.library_artist()}</th>
				<th class="py-3 pr-4 font-semibold">{m.library_tracks()}</th>
				<th class="py-3 pr-4 font-semibold">{m.library_col_size()}</th>
				<th class="py-3 pr-4 font-semibold">{m.library_col_added()}</th>
				<th class="py-3 pr-4 font-semibold">{m.common_status()}</th>
				<th class="w-24 py-3 pr-4"></th>
			</tr>
		</thead>
		<tbody>
			{#each items as album (album.id)}
				<tr class="border-b border-edge/50 transition-colors last:border-0 hover:bg-surface-2/40">
					<td class="px-4 py-3">
						<a href="/admin/music/{album.id}" class="group flex items-center gap-3">
							<span class="block size-10 shrink-0 overflow-hidden rounded-md bg-surface-2">
								{#if album.coverId}
									<img
										src="{artworkUrl(album.coverId)}?size=w342"
										alt=""
										loading="lazy"
										class="size-full object-cover"
									/>
								{:else}
									<span class="flex size-full items-center justify-center">
										<Music class="size-4 text-accent/60" />
									</span>
								{/if}
							</span>
							<span class="min-w-0">
								<span class="block truncate font-semibold group-hover:text-accent">
									{album.name}
								</span>
								{#if album.year}
									<span class="block text-xs text-faint tnum">{album.year}</span>
								{/if}
							</span>
						</a>
					</td>
					<td class="py-3 pr-4 text-muted">{album.artistName}</td>
					<td class="py-3 pr-4 text-muted tnum">{album.trackCount}</td>
					<td class="py-3 pr-4 text-muted tnum">{formatBytes(album.sizeBytes)}</td>
					<td class="py-3 pr-4 text-muted tnum">{formatDate(album.addedAt)}</td>
					<td class="py-3 pr-4"><StatusPill status={album.status} /></td>
					<td class="py-3 pr-4">
						<span class="flex justify-end gap-1">
							<a
								href="/admin/music/{album.id}"
								class="rounded-full p-2 text-muted transition-colors hover:bg-surface-2 hover:text-text"
								title={m.common_edit()}
							>
								<Pencil class="size-3.5" />
							</a>
							<button
								class="rounded-full p-2 text-muted transition-colors hover:bg-danger/15 hover:text-danger"
								title={m.common_delete()}
								onclick={() => {
									pendingDelete = album;
									confirmDelete = true;
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

	{#if loaded && items.length === 0}
		<EmptyState title={m.library_albums_empty_title()} message={m.library_albums_empty_message()} />
	{/if}
</div>

<Confirm
	bind:open={confirmDelete}
	title={m.library_delete_named_title({ name: pendingDelete?.name ?? '' })}
	message={m.library_album_delete_message()}
	onconfirm={deleteOne}
/>
