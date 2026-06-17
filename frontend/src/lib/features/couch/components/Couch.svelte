<script lang="ts">
	import { onMount } from 'svelte';
	import { Pause } from 'lucide-svelte';
	import { backOut, cubicOut } from 'svelte/easing';
	import { fade, fly, scale } from 'svelte/transition';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import Tooltip from '$lib/components/ui/Tooltip.svelte';
	import { couch } from '$lib/features/couch/couch.svelte';
	import CouchLeft from './icons/CouchLeft.svelte';
	import CouchMiddle from './icons/CouchMiddle.svelte';
	import CouchRight from './icons/CouchRight.svelte';
	import Remote from './icons/Remote.svelte';
	import * as m from '$lib/paraglide/messages';

	let { height = 'h-20', avatar = 'size-9' }: { height?: string; avatar?: string } = $props();

	// left + (n-1) middle + right (one participant => left + right, no middle)
	const middles = $derived(
		Array.from({ length: Math.max(0, couch.participants.length - 1) }, (_, i) => i)
	);

	const reactionsFor = (pid: string) => couch.reactions.filter((r) => r.participantId === pid);

	// suppress the per-seat intros on first paint so launch is just the bar fly;
	// later joins then animate (middle slides in, avatar pops down)
	let ready = $state(false);
	onMount(() => {
		ready = true;
	});
	const seatIn = $derived(ready ? { y: -28, duration: 350, easing: backOut } : { duration: 0 });

	// width-only grow used for couch sections: Svelte's slide also animates height
	// (which warped the couch), this touches width + margin only.
	function growX(node: Element, { duration = 350 }: { duration?: number } = {}) {
		const style = getComputedStyle(node);
		const width = parseFloat(style.width) || 0;
		const ml = parseFloat(style.marginLeft) || 0;
		return {
			duration,
			easing: cubicOut,
			css: (t: number) => `overflow: hidden; width: ${t * width}px; margin-left: ${t * ml}px;`
		};
	}
</script>

<div class="relative inline-flex text-accent" in:fly={{ y: -28, duration: 350, easing: backOut }}>
	<!-- seated avatars overlaid on the cushions -->
	<div
		class="absolute inset-x-0 top-1/2 z-10 flex -translate-y-[60%] items-end justify-center gap-6 px-5"
	>
		{#each couch.participants as p (p.id)}
			<div class="relative" data-couch-seat={p.id} in:fly={seatIn} out:fade={{ duration: 150 }}>
				<Tooltip label={p.displayName}>
					{#snippet trigger(props)}
						<span {...props} class="block">
							<UserAvatar
								name={p.displayName}
								avatarId={p.avatarId ?? null}
								seed={p.seed}
								class="{avatar} rounded-lg text-[10px] ring-2 ring-black/40 transition-[filter,transform]
									duration-300 {p.paused ? 'scale-90 grayscale' : 'scale-100 grayscale-0'}"
							/>
						</span>
					{/snippet}
				</Tooltip>

				{#if p.isHost}
					<Tooltip label={m.couch_has_remote({ name: p.displayName })} side="top">
						{#snippet trigger(props)}
							<span
								{...props}
								class="absolute -top-3 -right-2.5 flex size-5 rotate-12 items-center justify-center
									rounded-full bg-black/70 text-white shadow"
							>
								<Remote class="size-3" />
							</span>
						{/snippet}
					</Tooltip>
				{/if}

				{#if p.paused}
					<span
						transition:scale={{ duration: 200, start: 0.4 }}
						class="absolute -top-3 left-1/2 flex size-5 -translate-x-1/2 items-center justify-center
							rounded-full bg-black/75 text-white shadow"
					>
						<Pause class="size-3 fill-current" />
					</span>
				{/if}

				<!-- reactions rise up from the sender's seat, growing as they fade -->
				{#each reactionsFor(p.id) as r (r.id)}
					<span class="couch-reaction pointer-events-none absolute -top-1 left-1/2 z-20 text-xl">
						{r.emoji}
					</span>
				{/each}
			</div>
		{/each}
	</div>

	<!-- the assembled couch frame (sections overlap 1px to avoid sub-pixel seams) -->
	<div class="flex items-end {height}">
		<CouchLeft class="h-full w-auto" />
		{#each middles as i (i)}
			<div class="-ml-px flex h-full" transition:growX={{ duration: ready ? 350 : 0 }}>
				<CouchMiddle class="h-full w-auto" />
			</div>
		{/each}
		<CouchRight class="-ml-px h-full w-auto" />
	</div>
</div>

<style>
	.couch-reaction {
		transform: translate(-50%, 0) scale(0.5);
		animation: couch-reaction-rise 2s cubic-bezier(0.25, 0.6, 0.3, 1) forwards;
	}
	@keyframes couch-reaction-rise {
		0% {
			transform: translate(-50%, 6px) scale(0.5);
			opacity: 0;
		}
		15% {
			opacity: 1;
		}
		70% {
			opacity: 1;
		}
		100% {
			transform: translate(-50%, -95px) scale(1.7);
			opacity: 0;
		}
	}
</style>
