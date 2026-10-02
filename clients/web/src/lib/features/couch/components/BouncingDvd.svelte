<script lang="ts">
	import { onMount } from 'svelte';
	import DvdLogo from './icons/DvdLogo.svelte';

	// the classic bouncing-DVD screensaver: it ricochets off the screen edges and
	// flips to a random colour on every bounce (everyone waits for the corner hit)
	let w = $state(0);
	let h = $state(0);
	let dvdW = $state(0);
	let dvdH = $state(0);
	let x = $state(0);
	let y = $state(0);
	let color = $state('hsl(280, 85%, 62%)');

	let vx = 0;
	let vy = 0;

	const randomColor = () => `hsl(${Math.floor(Math.random() * 360)}, 85%, 62%)`;

	onMount(() => {
		const speed = 130; // px/s
		// a diagonal-ish start so it actually travels toward corners
		const angle = (0.25 + Math.random() * 0.5) * (Math.PI / 2) + Math.PI / 8;
		vx = Math.cos(angle) * speed * (Math.random() < 0.5 ? 1 : -1);
		vy = Math.sin(angle) * speed * (Math.random() < 0.5 ? 1 : -1);
		color = randomColor();

		let raf = 0;
		let last = 0;
		const frame = (t: number) => {
			if (last && w && h && dvdW && dvdH) {
				const dt = Math.min(0.05, (t - last) / 1000);
				const maxX = Math.max(0, w - dvdW);
				const maxY = Math.max(0, h - dvdH);
				let nx = x + vx * dt;
				let ny = y + vy * dt;
				if (nx <= 0) {
					nx = 0;
					vx = Math.abs(vx);
					color = randomColor();
				} else if (nx >= maxX) {
					nx = maxX;
					vx = -Math.abs(vx);
					color = randomColor();
				}
				if (ny <= 0) {
					ny = 0;
					vy = Math.abs(vy);
					color = randomColor();
				} else if (ny >= maxY) {
					ny = maxY;
					vy = -Math.abs(vy);
					color = randomColor();
				}
				x = nx;
				y = ny;
			}
			last = t;
			raf = requestAnimationFrame(frame);
		};
		raf = requestAnimationFrame(frame);
		return () => cancelAnimationFrame(raf);
	});
</script>

<div
	bind:clientWidth={w}
	bind:clientHeight={h}
	class="pointer-events-none absolute inset-0 overflow-hidden"
>
	<div
		bind:clientWidth={dvdW}
		bind:clientHeight={dvdH}
		class="absolute top-0 left-0 w-28 will-change-transform sm:w-36"
		style="transform: translate({x}px, {y}px); color: {color};"
	>
		<DvdLogo class="w-full opacity-80" />
	</div>
</div>
