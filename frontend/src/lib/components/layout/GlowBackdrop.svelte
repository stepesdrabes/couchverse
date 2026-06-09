<!-- ambient radial glows behind hero/login surfaces, as in the mockups -->
<div class="glow glow-violet" aria-hidden="true"></div>
<div class="glow glow-teal" aria-hidden="true"></div>

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
