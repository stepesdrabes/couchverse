<script lang="ts">
	import AdminSidebar from '$lib/features/admin/components/AdminSidebar.svelte';

	let { children } = $props();
</script>

<div class="flex min-h-dvh">
	<AdminSidebar />
	<!-- pages in this group own their container so heroes can bleed full width -->
	<main class="min-w-0 flex-1 overflow-x-hidden">
		{@render children()}
	</main>
</div>
