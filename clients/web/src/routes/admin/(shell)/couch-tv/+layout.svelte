<script lang="ts">
	import AdminCouchTvLayout from '$lib/features/couchtv/pages/AdminCouchTvLayout.svelte';

	let { children } = $props();
</script>

<AdminCouchTvLayout>
	{@render children()}
</AdminCouchTvLayout>
