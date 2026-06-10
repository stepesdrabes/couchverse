<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { ArrowLeft, Check, ImagePlus, Pencil, Trash2, X } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import { artworkUrl } from '$lib/features/catalog/api';
	import * as libraryApi from '$lib/features/library/api';
	import EditorUploadCard from '$lib/components/admin/EditorUploadCard.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Confirm from '$lib/components/ui/Confirm.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import StatusPill from '$lib/components/ui/StatusPill.svelte';
	import { formatClock } from '$lib/utils/format';

	let { data } = $props();

	let name = $state(data.album.name);
	let artistName = $state(data.album.artistName);
	let year = $state(data.album.year?.toString() ?? '');
	let status = $state<string>(data.album.status);
	let saving = $state(false);
	let confirmDelete = $state(false);

	let editingTrack = $state<number | null>(null);
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
			toast.success('Saved');
			invalidateAll();
		} catch {
			toast.error('Failed to save');
		} finally {
			saving = false;
		}
	}

	async function uploadCover(files: FileList | null) {
		const file = files?.[0];
		if (!file) return;
		try {
			await libraryApi.uploadArtwork('album', data.album.id, 'album_cover', file);
			toast.success('Cover updated');
			invalidateAll();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Cover upload failed');
		}
	}

	async function saveTrack(id: number) {
		const next = trackName.trim();
		editingTrack = null;
		if (!next) return;
		try {
			await libraryApi.renameTrack(id, next);
			invalidateAll();
		} catch {
			toast.error('Failed to rename track');
		}
	}

	async function removeTrack(id: number) {
		try {
			await libraryApi.deleteTrack(id);
			invalidateAll();
		} catch {
			toast.error('Failed to delete track');
		}
	}

	async function removeAlbum() {
		try {
			await libraryApi.deleteAlbum(data.album.id);
			toast.success('Album deleted');
			goto('/admin/library');
		} catch {
			toast.error('Failed to delete album');
		}
	}
</script>

<svelte:head>
	<title>{data.album.name} — Couchverse admin</title>
</svelte:head>

<a
	href="/admin/library"
	class="mb-4 inline-flex items-center gap-1.5 text-xs font-medium text-faint transition-colors hover:text-text"
>
	<ArrowLeft class="size-3.5" />
	Library
</a>

<div class="mb-6 flex items-center gap-4">
	<h1 class="text-2xl font-bold">{data.album.name}</h1>
	<StatusPill status={data.album.status} />
	<span class="text-xs text-faint">Album</span>
</div>

<div class="grid gap-8 lg:grid-cols-[1fr_320px]">
	<div class="space-y-8">
		<form onsubmit={save} class="space-y-4 rounded-card border border-edge bg-surface/40 p-6">
			<h2 class="text-sm font-semibold text-muted">Metadata</h2>
			<div class="grid gap-4 sm:grid-cols-2">
				<Input label="Album name" bind:value={name} required />
				<Input label="Artist" bind:value={artistName} required />
				<Input label="Year" type="number" bind:value={year} />
			</div>
			<div class="flex items-center justify-between pt-2">
				<Select
					bind:value={status}
					label="Status"
					items={[
						{ value: 'draft', label: 'Draft' },
						{ value: 'published', label: 'Published' },
						{ value: 'hidden', label: 'Hidden' }
					]}
				/>
				<Button type="submit" loading={saving}>Save changes</Button>
			</div>
		</form>

		<section class="rounded-card border border-edge bg-surface/40 p-6">
			<h2 class="mb-4 text-sm font-semibold text-muted">
				Tracks
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
								aria-label="Save name"
							>
								<Check class="size-3.5" />
							</button>
							<button
								class="rounded-full p-1.5 text-faint"
								onclick={() => (editingTrack = null)}
								aria-label="Cancel"
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
								aria-label="Rename track"
							>
								<Pencil class="size-3" />
							</button>
							<button
								class="rounded-full p-1.5 text-faint transition-colors hover:bg-danger/15 hover:text-danger"
								onclick={() => removeTrack(track.id)}
								aria-label="Delete track"
							>
								<Trash2 class="size-3" />
							</button>
						{/if}
					</li>
				{:else}
					<li class="px-3 py-4 text-xs text-faint">No tracks — upload audio files.</li>
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
			<h2 class="mb-4 text-sm font-semibold text-muted">Cover</h2>
			<button
				type="button"
				class="group relative block size-36 overflow-hidden rounded-input border border-edge bg-surface-2
					transition-colors hover:border-accent"
				onclick={() => coverInput?.click()}
				title="Upload cover"
			>
				{#if data.album.coverId}
					<img
						src="{artworkUrl(data.album.coverId)}?size=w342"
						alt="Album cover"
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
			<h2 class="mb-2 text-sm font-semibold text-danger">Danger zone</h2>
			<p class="mb-4 text-xs text-faint">
				Removes the album, its tracks and playlist entries. Files on disk are kept.
			</p>
			<Button variant="danger" size="sm" onclick={() => (confirmDelete = true)}>
				<Trash2 class="size-3.5" />
				Delete album
			</Button>
		</div>
	</aside>
</div>

<Confirm
	bind:open={confirmDelete}
	title="Delete “{data.album.name}”?"
	message="The album and its tracks are removed from the catalog. Audio files on disk stay."
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
