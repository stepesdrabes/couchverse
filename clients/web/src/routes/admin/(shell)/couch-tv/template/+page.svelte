<script lang="ts">
	import type { PageProps } from './$types';
	import AdminTemplatePage from '$lib/features/couchtv/pages/AdminTemplatePage.svelte';

	let { data }: PageProps = $props();
</script>

<AdminTemplatePage {data} />
