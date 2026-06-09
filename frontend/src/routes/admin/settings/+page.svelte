<script lang="ts">
	import { toast } from 'svelte-sonner';
	import * as admin from '$lib/api/admin';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';

	let tmdbKey = $state('');
	let saving = $state(false);

	$effect(() => {
		admin.getSettings().then((s) => {
			tmdbKey = typeof s['tmdb.api_key'] === 'string' ? (s['tmdb.api_key'] as string) : '';
		});
	});

	async function save(e: SubmitEvent) {
		e.preventDefault();
		saving = true;
		try {
			await admin.putSettings({ 'tmdb.api_key': tmdbKey });
			toast.success('Settings saved');
		} catch {
			toast.error('Failed to save settings');
		} finally {
			saving = false;
		}
	}
</script>

<svelte:head>
	<title>Settings — Couchverse admin</title>
</svelte:head>

<h1 class="mb-6 text-2xl font-bold">Settings</h1>

<form onsubmit={save} class="max-w-lg space-y-4 rounded-card border border-edge bg-surface/40 p-6">
	<h2 class="text-sm font-semibold text-muted">Metadata</h2>
	<Input
		label="TMDB API key"
		bind:value={tmdbKey}
		placeholder="paste your themoviedb.org API key"
		autocomplete="off"
	/>
	<p class="text-xs leading-relaxed text-faint">
		With a key set, the title editor can search TMDB and fill in posters, overviews and genres
		automatically.
	</p>
	<div class="flex justify-end">
		<Button type="submit" loading={saving}>Save</Button>
	</div>
</form>

<p class="mt-6 text-xs text-faint">Transcoding preferences arrive with the HLS milestone.</p>
