<script lang="ts">
	let { children } = $props();
</script>

<main class="min-h-dvh">
	<div class="mx-auto max-w-7xl px-8 pb-12">
		{@render children()}
	</div>
</main>
