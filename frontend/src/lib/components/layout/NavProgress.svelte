<script lang="ts">
	import { navigating } from '$app/state';
</script>

{#if navigating.to}
	<div class="nav-progress" role="progressbar" aria-label="Loading" aria-busy="true"></div>
{/if}

<style lang="scss">
	.nav-progress {
		position: fixed;
		inset: 0 0 auto 0;
		height: 2.5px;
		z-index: 100;
		overflow: hidden;
		/* stays put during the View Transitions cross-fade */
		view-transition-name: nav-progress;
		background: color-mix(in srgb, var(--color-accent) 14%, transparent);

		&::before {
			content: '';
			position: absolute;
			inset: 0;
			width: 45%;
			border-radius: 999px;
			background: linear-gradient(
				90deg,
				transparent,
				var(--color-accent) 45%,
				var(--color-accent) 55%,
				transparent
			);
			box-shadow: 0 0 10px color-mix(in srgb, var(--color-accent) 65%, transparent);
			animation: nav-progress-slide 1.05s cubic-bezier(0.65, 0, 0.35, 1) infinite;
		}
	}

	@keyframes nav-progress-slide {
		from {
			transform: translateX(-110%);
		}
		to {
			transform: translateX(322%);
		}
	}

	@media (prefers-reduced-motion: reduce) {
		.nav-progress::before {
			animation: none;
			width: 100%;
			background: var(--color-accent);
			opacity: 0.8;
		}
	}
</style>
