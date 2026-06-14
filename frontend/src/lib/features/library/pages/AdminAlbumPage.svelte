<script lang="ts">
	import { untrack } from 'svelte';
	import { goto, invalidateAll } from '$app/navigation';
	import { ArrowLeft, Check, ImagePlus, Pencil, Trash2, X } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import { artworkUrl } from '$lib/features/catalog/api';
	import * as libraryApi from '$lib/features/library/api';
	import EditorUploadCard from '$lib/features/uploads/components/EditorUploadCard.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Confirm from '$lib/components/ui/Confirm.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import StatusPill from '$lib/components/ui/StatusPill.svelte';
	import { formatClock } from '$lib/utils/format';
	import { FormState } from '$lib/utils/form-state.svelte';
	import * as m from '$lib/paraglide/messages';

	let { data }: { data: Awaited<ReturnType<typeof libraryApi.getAdminAlbum>> } = $props();

	let name = $state(data.album.name);
	let artistName = $state(data.album.artistName);
	let year = $state(data.album.year?.toString() ?? '');
	let status = $state<string>(data.album.status);
	let saving = $state(false);
	let confirmDelete = $state(false);
	const form = new FormState(() => ({ name, artistName, year, status }));

	// re-sync after invalidateAll - $state initializers only run once
	$effect(() => {
		const album = data.album;
		untrack(() => {
			if (form.dirty) return;
			name = album.name;
			artistName = album.artistName;
			year = album.year?.toString() ?? '';
			status = album.status;
			form.reset();
		});
	});

	let editingTrack = $state<string | null>(null);
	let trackName = $state('');
	let coverInput = $state<HTMLInputElement>();

	async function save(e: SubmitEvent) {
		e.preventDefault();
		saving = true;
		try {
			await libraryApi.updateAlbum(data.album.id, {
				name,
				artistName,
				year: year ? Number(year) : null,
				status
			});
			form.reset();
			toast.success(m.common_saved());
			invalidateAll();
		} catch {
			toast.error(m.library_album_save_failed());
		} finally {
			saving = false;
		}
	}

	async function uploadCover(files: FileList | null) {
		const file = files?.[0];
		if (!file) return;
		try {
			await libraryApi.uploadArtwork('album', data.album.id, 'album_cover', file);
			toast.success(m.library_cover_updated());
			invalidateAll();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : m.library_cover_upload_failed());
		}
	}

	async function saveTrack(id: string) {
		const next = trackName.trim();
		editingTrack = null;
		if (!next) return;
		try {
			await libraryApi.renameTrack(id, next);
			invalidateAll();
		} catch {
			toast.error(m.library_track_rename_failed());
		}
	}

	async function removeTrack(id: string) {
		try {
			await libraryApi.deleteTrack(id);
			invalidateAll();
		} catch {
			toast.error(m.library_track_delete_failed());
		}
	}

	async function removeAlbum() {
		try {
			await libraryApi.deleteAlbum(data.album.id);
			toast.success(m.library_album_deleted());
			goto('/admin/library');
		} catch {
			toast.error(m.library_album_delete_failed());
		}
	}
</script>

<svelte:head>
	<title>{m.library_album_page_title({ name: data.album.name })}</title>
</svelte:head>

<a
	href="/admin/library"
	class="mb-4 inline-flex items-center gap-1.5 text-xs font-medium text-faint transition-colors hover:text-text"
>
	<ArrowLeft class="size-3.5" />
	{m.library_heading()}
</a>

<div class="mb-6 flex items-center gap-4">
	<h1 class="text-2xl font-bold">{data.album.name}</h1>
	<StatusPill status={data.album.status} />
	<span class="text-xs text-faint">{m.library_album()}</span>
</div>

<div class="grid gap-8 lg:grid-cols-[1fr_320px]">
	<div class="space-y-8">
		<form onsubmit={save} class="space-y-4 rounded-card border border-edge bg-surface/40 p-6">
			<h2 class="text-sm font-semibold text-muted">{m.library_metadata()}</h2>
			<div class="grid gap-4 sm:grid-cols-2">
				<Input label={m.library_album_name()} bind:value={name} required />
				<Input label={m.library_artist()} bind:value={artistName} required />
				<Input label={m.library_year()} type="number" bind:value={year} />
			</div>
			<div class="flex items-center justify-between pt-2">
				<Select
					bind:value={status}
					label={m.common_status()}
					items={[
						{ value: 'draft', label: m.library_status_draft() },
						{ value: 'published', label: m.library_status_published() },
						{ value: 'hidden', label: m.library_status_hidden() }
					]}
				/>
				<Button type="submit" loading={saving} disabled={!form.dirty}
					>{m.library_save_changes()}</Button
				>
			</div>
		</form>

		<section class="rounded-card border border-edge bg-surface/40 p-6">
			<h2 class="mb-4 text-sm font-semibold text-muted">
				{m.library_tracks()}
				<span class="ml-1 font-normal text-faint tnum">({data.tracks.length})</span>
			</h2>
			<ul class="divide-y divide-edge/50 rounded-input border border-edge/70">
				{#each data.tracks as track (track.id)}
					<li class="flex items-center gap-3 px-3 py-2 text-sm">
						<span class="w-8 text-xs text-faint tnum">{track.trackNumber}</span>
						{#if editingTrack === track.id}
							<!-- svelte-ignore a11y_autofocus -->
							<input
								bind:value={trackName}
								autofocus
								class="h-8 flex-1 rounded-lg border border-edge bg-surface px-2 text-sm focus:border-accent focus:outline-none"
								onkeydown={(e) => {
									if (e.key === 'Enter') saveTrack(track.id);
									if (e.key === 'Escape') editingTrack = null;
								}}
							/>
							<button
								class="rounded-full p-1.5 text-success"
								onclick={() => saveTrack(track.id)}
								aria-label={m.library_save_name()}
							>
								<Check class="size-3.5" />
							</button>
							<button
								class="rounded-full p-1.5 text-faint"
								onclick={() => (editingTrack = null)}
								aria-label={m.common_cancel()}
							>
								<X class="size-3.5" />
							</button>
						{:else}
							<span class="flex-1 truncate">{track.name}</span>
							<span class="text-xs text-faint tnum">{formatClock(track.durationSeconds)}</span>
							<button
								class="rounded-full p-1.5 text-faint transition-colors hover:text-text"
								onclick={() => {
									editingTrack = track.id;
									trackName = track.name;
								}}
								aria-label={m.library_rename_track()}
							>
								<Pencil class="size-3" />
							</button>
							<button
								class="rounded-full p-1.5 text-faint transition-colors hover:bg-danger/15 hover:text-danger"
								onclick={() => removeTrack(track.id)}
								aria-label={m.library_delete_track()}
							>
								<Trash2 class="size-3" />
							</button>
						{/if}
					</li>
				{:else}
					<li class="px-3 py-4 text-xs text-faint">{m.library_no_tracks()}</li>
				{/each}
			</ul>
		</section>
	</div>

	<aside class="space-y-6">
		<EditorUploadCard
			kind="music"
			hint="Tagged audio files are sorted by their artist/album tags."
		/>

		<div class="rounded-card border border-edge bg-surface/40 p-6">
			<h2 class="mb-4 text-sm font-semibold text-muted">{m.library_cover()}</h2>
			<button
				type="button"
				class="group relative block size-36 overflow-hidden rounded-input border border-edge bg-surface-2
					transition-colors hover:border-accent"
				onclick={() => coverInput?.click()}
				title={m.library_upload_cover()}
			>
				{#if data.album.coverId}
					<img
						src="{artworkUrl(data.album.coverId)}?size=w342"
						alt={m.library_album_cover_alt()}
						class="size-full object-cover"
					/>
				{/if}
				<span
					class="absolute inset-0 flex items-center justify-center bg-black/50
						{data.album.coverId ? 'opacity-0 transition-opacity group-hover:opacity-100' : ''}"
				>
					<ImagePlus class="size-5 text-white" />
				</span>
			</button>
		</div>

		<div class="rounded-card border border-danger/30 bg-danger/5 p-6">
			<h2 class="mb-2 text-sm font-semibold text-danger">{m.library_danger_zone()}</h2>
			<p class="mb-4 text-xs text-faint">
				{m.library_album_danger_text()}
			</p>
			<Button variant="danger" size="sm" onclick={() => (confirmDelete = true)}>
				<Trash2 class="size-3.5" />
				{m.library_delete_album()}
			</Button>
		</div>
	</aside>
</div>

<Confirm
	bind:open={confirmDelete}
	title={m.library_delete_named_title({ name: data.album.name })}
	message={m.library_album_delete_message()}
	onconfirm={removeAlbum}
/>

<input
	bind:this={coverInput}
	type="file"
	accept=".jpg,.jpeg,.png,.webp"
	class="hidden"
	onchange={(e) => {
		uploadCover(e.currentTarget.files);
		e.currentTarget.value = '';
	}}
/>
