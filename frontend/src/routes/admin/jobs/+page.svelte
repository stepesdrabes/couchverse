<script lang="ts">
	import { Database } from 'lucide-svelte';
	import EmptyState from '$lib/components/ui/EmptyState.svelte';
</script>

<svelte:head>
	<title>Jobs & Storage — Couchverse admin</title>
</svelte:head>

<h1 class="mb-6 text-2xl font-bold">Jobs & Storage</h1>

<div class="rounded-card border border-edge bg-surface/40">
	<EmptyState
		title="The job queue arrives with milestone 4"
		message="Library scans, ffprobe analysis and transcodes will report progress here."
	>
		<Database class="size-5 text-faint" />
	</EmptyState>
</div>
