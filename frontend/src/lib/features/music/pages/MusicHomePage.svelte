<script lang="ts">
	import { invalidateAll } from '$app/navigation';
	import { ListMusic, Mic2, Plus } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import AlbumCard from '$lib/features/music/components/AlbumCard.svelte';
	import MediaRow from '$lib/features/catalog/components/MediaRow.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Modal from '$lib/components/ui/Modal.svelte';
	import { artworkUrl } from '$lib/features/catalog/api';
	import * as musicApi from '$lib/features/music/api';

	let { data }: { data: Awaited<ReturnType<typeof musicApi.musicHome>> } = $props();

	let createOpen = $state(false);
	let newName = $state('');
	let creating = $state(false);

	async function createPlaylist(e: SubmitEvent) {
		e.preventDefault();
		creating = true;
		try {
			await musicApi.createPlaylist(newName);
			createOpen = false;
			newName = '';
			invalidateAll();
		} catch {
			toast.error('Failed to create playlist');
		} finally {
			creating = false;
		}
	}
</script>

<svelte:head>
	<title>Music - Couchverse</title>
</svelte:head>

<div class="pt-24 pb-16">
	<div class="mb-8 flex items-center justify-between px-6 lg:px-12">
		<h1 class="text-2xl font-bold">Music</h1>
		<Button variant="secondary" size="sm" onclick={() => (createOpen = true)}>
			<Plus class="size-3.5" />
			New playlist
		</Button>
	</div>

	{#if data.recentAlbums.length === 0}
		<EmptyState
			title="No music yet"
			message="Drop tagged audio files into the music library and scan."
		/>
	{:else}
		<div class="space-y-10">
			{#if data.recentlyPlayed.length > 0}
				<MediaRow label="Recently played">
					{#each data.recentlyPlayed as album (album.id)}
						<AlbumCard {album} />
					{/each}
				</MediaRow>
			{/if}

			<MediaRow label="Recently added">
				{#each data.recentAlbums as album (album.id)}
					<AlbumCard {album} />
				{/each}
			</MediaRow>

			{#if data.playlists.length > 0}
				<section class="px-6 lg:px-12">
					<h2 class="eyebrow mb-3">Your playlists</h2>
					<div class="grid grid-cols-2 gap-4 sm:grid-cols-3 lg:grid-cols-5">
						{#each data.playlists as playlist (playlist.id)}
							<a
								href="/music/playlists/{playlist.id}"
								class="group flex items-center gap-3 rounded-card border border-edge/60 bg-surface/40
									p-3 transition-colors hover:border-accent/60"
							>
								<span
									class="flex size-12 shrink-0 items-center justify-center overflow-hidden rounded-lg
										bg-accent-soft"
								>
									{#if playlist.coverId}
										<img src={artworkUrl(playlist.coverId)} alt="" class="size-full object-cover" />
									{:else}
										<ListMusic class="size-5 text-accent" />
									{/if}
								</span>
								<span class="min-w-0">
									<span class="block truncate text-sm font-semibold group-hover:text-accent">
										{playlist.name}
									</span>
									<span class="text-xs text-faint tnum">{playlist.trackCount} tracks</span>
								</span>
							</a>
						{/each}
					</div>
				</section>
			{/if}

			<section class="px-6 lg:px-12">
				<h2 class="eyebrow mb-3">Artists</h2>
				<div class="flex flex-wrap gap-2">
					{#each data.artists as artist (artist.id)}
						<a
							href="/music/artists/{artist.id}"
							class="inline-flex items-center gap-2 rounded-full border border-edge bg-surface/60
								px-4 py-2 text-sm font-medium transition-colors hover:border-accent hover:text-accent"
						>
							<Mic2 class="size-3.5 text-faint" />
							{artist.name}
						</a>
					{/each}
				</div>
			</section>
		</div>
	{/if}
</div>

<Modal bind:open={createOpen} title="New playlist">
	<form onsubmit={createPlaylist} class="space-y-4">
		<Input label="Name" bind:value={newName} required placeholder="Road trip" />
		<div class="flex justify-end">
			<Button type="submit" loading={creating}>Create</Button>
		</div>
	</form>
</Modal>
