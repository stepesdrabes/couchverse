<script lang="ts">
	import type { PageProps } from './$types';
	import AdminAlbumEditor from '$lib/features/library/pages/AdminAlbumEditor.svelte';

	let { data }: PageProps = $props();
</script>

<AdminAlbumEditor {data} />
