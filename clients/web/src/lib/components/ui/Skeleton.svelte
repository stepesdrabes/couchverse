<script lang="ts">
	let { class: cls = '' }: { class?: string } = $props();
</script>

<div
	class="animate-shimmer rounded-lg bg-gradient-to-r from-surface via-surface-2 to-surface bg-[length:200%_100%] {cls}"
></div>
