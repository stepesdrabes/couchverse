<script lang="ts">
	import { Upload } from 'lucide-svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
</script>

<svelte:head>
	<title>Uploads — Couchverse admin</title>
</svelte:head>

<h1 class="mb-6 text-2xl font-bold">Uploads</h1>

<div class="rounded-card border border-edge bg-surface/40">
	<EmptyState
		title="Uploads arrive with milestone 6"
		message="Resumable chunked uploads with drag & drop, pause and resume."
	>
		<Upload class="size-5 text-faint" />
	</EmptyState>
</div>
