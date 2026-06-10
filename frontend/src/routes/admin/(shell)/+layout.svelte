<script lang="ts">
	import AdminSidebar from '$lib/features/admin/components/AdminSidebar.svelte';

	let { children } = $props();
</script>

<div class="flex min-h-dvh">
	<AdminSidebar />
	<main class="min-w-0 flex-1 overflow-x-hidden">
		<div class="mx-auto max-w-6xl px-8 py-8">
			{@render children()}
		</div>
	</main>
</div>
