<script lang="ts">
	let { children } = $props();
</script>

<!-- pages in this group own their container so heroes can bleed full width -->
{@render children()}
