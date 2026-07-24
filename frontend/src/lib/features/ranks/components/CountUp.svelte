<script lang="ts">
	import { Tween } from 'svelte/motion';
	import { cubicOut } from 'svelte/easing';
	import { prefersReducedMotion } from 'svelte/motion';

	// A number that rolls up to its value. Svelte's Tween shares one frame loop
	// across every instance, so a handful of these costs nothing on a Pi. The
	// global reduced-motion clamp in app.css only covers CSS animations, so this
	// is the one place that has to check the preference itself.
	let {
		value,
		format = (n: number) => String(Math.round(n)),
		duration = 900,
		class: cls = ''
	}: {
		value: number;
		format?: (n: number) => string;
		duration?: number;
		class?: string;
	} = $props();

	const tween = Tween.of(() => value, { duration, easing: cubicOut });
	const shown = $derived(prefersReducedMotion.current ? value : tween.current);
</script>

<span class="tnum {cls}">{format(shown)}</span>
