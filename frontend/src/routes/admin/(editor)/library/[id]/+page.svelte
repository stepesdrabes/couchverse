<script lang="ts">
	import type { PageProps } from './$types';
	import AdminTitleEditorPage from '$lib/features/library/pages/AdminTitleEditorPage.svelte';

	let { data }: PageProps = $props();
</script>

<AdminTitleEditorPage {data} />
