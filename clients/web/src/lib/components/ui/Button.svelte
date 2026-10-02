<script lang="ts">
	import type { Snippet } from 'svelte';
	import type { HTMLButtonAttributes } from 'svelte/elements';

	type Props = HTMLButtonAttributes & {
		variant?: 'primary' | 'secondary' | 'ghost' | 'danger';
		size?: 'sm' | 'md' | 'lg';
		loading?: boolean;
		children: Snippet;
	};

	let {
		variant = 'primary',
		size = 'md',
		loading = false,
		disabled,
		class: cls = '',
		children,
		...rest
	}: Props = $props();

	const variants = {
		primary: 'bg-accent text-[var(--color-on-accent)] hover:bg-accent-strong',
		secondary: 'border border-edge bg-surface/60 text-text hover:border-faint hover:bg-surface-2',
		ghost: 'text-muted hover:bg-surface-2 hover:text-text',
		danger: 'bg-danger/15 text-danger hover:bg-danger/25'
	};
	const sizes = {
		sm: 'h-8 gap-1.5 px-4 text-xs',
		md: 'h-10 gap-2 px-5 text-sm',
		lg: 'h-11 gap-2 px-6 text-[15px]'
	};
</script>

<button
	class="relative inline-flex items-center justify-center rounded-full font-semibold transition-all
		duration-200 select-none active:scale-[0.97] cursor-pointer disabled:pointer-events-none disabled:opacity-50
		{variants[variant]} {sizes[size]} {cls}"
	disabled={disabled || loading}
	{...rest}
>
	{#if loading}
		<svg class="absolute size-4 animate-spin" viewBox="0 0 24 24" fill="none" aria-hidden="true">
			<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
			<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v4a4 4 0 00-4 4H4z" />
		</svg>
	{/if}
	<span class="contents {loading ? 'invisible' : ''}">{@render children()}</span>
</button>
