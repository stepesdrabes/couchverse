<div class="relative flex min-h-dvh items-center justify-center overflow-hidden">
	<div class="glow glow-violet"></div>
	<div class="glow glow-teal"></div>
	<div class="relative animate-slide-up text-center">
		<p class="eyebrow mb-4">Self-hosted streaming</p>
		<h1 class="text-4xl font-extrabold tracking-tight">
			couch<span class="text-accent">verse</span>
		</h1>
		<p class="mt-3 text-sm text-muted">Setting up the stage…</p>
	</div>
</div>

<style lang="scss">
	.glow {
		position: absolute;
		width: 60rem;
		height: 60rem;
		border-radius: 50%;
		filter: blur(120px);
		opacity: 0.14;
		pointer-events: none;

		&-violet {
			background: radial-gradient(circle, #8b7cf0 0%, transparent 65%);
			top: -20rem;
			left: -15rem;
		}

		&-teal {
			background: radial-gradient(circle, #3dd6c4 0%, transparent 65%);
			bottom: -25rem;
			right: -18rem;
		}
	}
</style>
