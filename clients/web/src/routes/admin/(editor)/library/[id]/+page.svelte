<script lang="ts">
	import type { PageProps } from './$types';
	import AdminTitleEditor from '$lib/features/library/pages/AdminTitleEditor.svelte';

	let { data }: PageProps = $props();
</script>

<AdminTitleEditor {data} />
