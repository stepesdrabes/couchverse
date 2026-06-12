<script lang="ts">
	import AdminSidebar from '$lib/features/admin/components/AdminSidebar.svelte';

	let { children } = $props();
</script>

<!-- The sidebar lives here (not in the (shell)/(editor) groups) so it persists
     across all admin navigation and only animates in when you enter admin. -->
<div class="flex min-h-dvh">
	<AdminSidebar />
	<main class="min-w-0 flex-1 overflow-x-hidden">
		{@render children()}
	</main>
</div>
