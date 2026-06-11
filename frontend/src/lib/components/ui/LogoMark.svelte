<script lang="ts">
	// The app mark: a rounded square in the accent colour with the logo centred
	// in the contrast colour. Tracks the live accent via CSS variables (so it
	// also follows the appearance preview).
	let { class: cls = 'size-8 rounded-lg' }: { class?: string } = $props();
</script>

<span class="flex shrink-0 items-center justify-center bg-accent {cls}" aria-hidden="true">
	<span
		class="block bg-[var(--color-on-accent)]"
		style="width: 66%; height: 66%;
			mask: url(/logo.svg) center / contain no-repeat;
			-webkit-mask: url(/logo.svg) center / contain no-repeat"
	></span>
</span>
