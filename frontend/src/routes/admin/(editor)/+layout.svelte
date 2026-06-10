<script lang="ts">
	import AdminSidebar from '$lib/components/admin/AdminSidebar.svelte';

	let { children } = $props();
</script>

<div class="flex min-h-dvh">
	<AdminSidebar />
	<main class="min-w-0 flex-1 overflow-x-hidden">
		<div class="mx-auto max-w-7xl px-8 pb-12">
			{@render children()}
		</div>
	</main>
</div>
