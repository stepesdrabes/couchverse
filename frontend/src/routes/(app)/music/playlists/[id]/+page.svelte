<script lang="ts">
	import { goto, invalidateAll } from '$app/navigation';
	import { ArrowDown, ArrowUp, ListMusic, Pencil, Play, Trash2, X } from 'lucide-svelte';
	import { flip } from 'svelte/animate';
	import { toast } from 'svelte-sonner';
	import Button from '$lib/components/ui/Button.svelte';
	import Confirm from '$lib/components/ui/Confirm.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Modal from '$lib/components/ui/Modal.svelte';
	import * as musicApi from '$lib/features/music/api';
	import { musicPlayer as player } from '$lib/features/music/player.svelte';
	import { formatClock } from '$lib/utils/format';

	let { data } = $props();

	// local copy so reorder/remove can be optimistic; rewritten on navigation
	let entries = $derived(data.entries);

	let renameOpen = $state(false);
	let newName = $state(data.playlist.name);
	let confirmDelete = $state(false);

	const tracks = $derived(entries.map((e) => e.track));

	function playAll() {
		player.playQueue(tracks);
	}

	async function move(index: number, dir: -1 | 1) {
		const target = index + dir;
		if (target < 0 || target >= entries.length) return;
		const next = [...entries];
		[next[index], next[target]] = [next[target], next[index]];
		entries = next;
		try {
			await musicApi.reorderPlaylist(
				data.playlist.id,
				next.map((e) => e.entryId)
			);
		} catch {
			toast.error('Failed to reorder');
			invalidateAll();
		}
	}

	async function removeEntry(entryId: number) {
		entries = entries.filter((e) => e.entryId !== entryId);
		try {
			await musicApi.removePlaylistEntry(data.playlist.id, entryId);
		} catch {
			toast.error('Failed to remove track');
			invalidateAll();
		}
	}

	async function rename(e: SubmitEvent) {
		e.preventDefault();
		try {
			await musicApi.renamePlaylist(data.playlist.id, newName);
			renameOpen = false;
			invalidateAll();
		} catch {
			toast.error('Failed to rename');
		}
	}

	async function deletePlaylist() {
		try {
			await musicApi.deletePlaylist(data.playlist.id);
			goto('/music');
		} catch {
			toast.error('Failed to delete');
		}
	}

	const isPlaying = (trackId: number) => player.current?.id === trackId;
</script>

<svelte:head>
	<title>{data.playlist.name} — Couchverse</title>
</svelte:head>

<div class="mx-auto max-w-4xl px-6 pt-28 pb-16">
	<div class="mb-8 flex items-end justify-between gap-4">
		<div class="min-w-0">
			<p class="eyebrow mb-2">Playlist</p>
			<h1 class="truncate text-3xl font-extrabold tracking-tight sm:text-4xl">
				{data.playlist.name}
			</h1>
			<p class="mt-2 text-sm text-faint tnum">{entries.length} tracks</p>
		</div>
		<div class="flex shrink-0 gap-2">
			<Button onclick={playAll} disabled={entries.length === 0}>
				<Play class="size-4 fill-current" />
				Play
			</Button>
			<Button variant="ghost" size="md" onclick={() => (renameOpen = true)} aria-label="Rename">
				<Pencil class="size-4" />
			</Button>
			<Button
				variant="ghost"
				size="md"
				onclick={() => (confirmDelete = true)}
				aria-label="Delete playlist"
			>
				<Trash2 class="size-4" />
			</Button>
		</div>
	</div>

	{#if entries.length === 0}
		<EmptyState title="Empty playlist" message="Add tracks from any album with the ⋮ menu.">
			<ListMusic class="size-5 text-faint" />
		</EmptyState>
	{:else}
		<ul class="overflow-hidden rounded-card border border-edge bg-surface/40">
			{#each entries as entry, i (entry.entryId)}
				<li
					animate:flip={{ duration: 250 }}
					class="group flex items-center gap-3 border-b border-edge/40 px-4 py-2.5 transition-colors
						last:border-0 hover:bg-surface-2/50"
				>
					<button
						class="w-6 text-left text-xs text-faint tnum"
						onclick={() => player.playQueue(tracks, i)}
						aria-label="Play from here"
					>
						{i + 1}
					</button>
					<div class="min-w-0 flex-1">
						<p
							class="truncate text-sm font-medium {isPlaying(entry.track.id) ? 'text-accent' : ''}"
						>
							{entry.track.name}
						</p>
						<p class="truncate text-xs text-faint">
							{entry.track.trackArtist ?? entry.track.artistName} · {entry.track.albumName}
						</p>
					</div>
					<span class="text-xs text-faint tnum">{formatClock(entry.track.durationSeconds)}</span>
					<span class="flex gap-0.5 opacity-0 transition-opacity group-hover:opacity-100">
						<button
							class="rounded-full p-1.5 text-faint hover:text-text disabled:opacity-30"
							disabled={i === 0}
							onclick={() => move(i, -1)}
							aria-label="Move up"
						>
							<ArrowUp class="size-3.5" />
						</button>
						<button
							class="rounded-full p-1.5 text-faint hover:text-text disabled:opacity-30"
							disabled={i === entries.length - 1}
							onclick={() => move(i, 1)}
							aria-label="Move down"
						>
							<ArrowDown class="size-3.5" />
						</button>
						<button
							class="rounded-full p-1.5 text-faint hover:text-danger"
							onclick={() => removeEntry(entry.entryId)}
							aria-label="Remove"
						>
							<X class="size-3.5" />
						</button>
					</span>
				</li>
			{/each}
		</ul>
	{/if}
</div>

<Modal bind:open={renameOpen} title="Rename playlist">
	<form onsubmit={rename} class="space-y-4">
		<Input label="Name" bind:value={newName} required />
		<div class="flex justify-end">
			<Button type="submit">Save</Button>
		</div>
	</form>
</Modal>

<Confirm
	bind:open={confirmDelete}
	title="Delete “{data.playlist.name}”?"
	message="The playlist is removed for good. Tracks stay in the library."
	onconfirm={deletePlaylist}
/>
