<script lang="ts">
	import { onMount, untrack } from 'svelte';
	import { Pause, Play, StepBack, StepForward, Volume2, VolumeX } from 'lucide-svelte';
	import { Player, type ClipSpec, type Cue, type PlayerSnapshot } from '@couchverse/broadcast';
	import * as m from '$lib/paraglide/messages';

	// A cutscene played live from its spec: the same timeline the renderer seeks frame by
	// frame, here against the audio clock, with scrubbing and frame steps for review.
	let { spec }: { spec: ClipSpec | null } = $props();

	let host = $state<HTMLDivElement>();
	let player = $state<Player | null>(null);
	let snap = $state<PlayerSnapshot>({ state: 'empty', time: 0, duration: 0, cue: null });
	let cues = $state<Cue[]>([]);
	let muted = $state(false);
	let failed = $state(false);
	// the template and the exact spec on screen: an equal spec never reloads
	let shown = '';
	let shownSpec = '';

	onMount(() => {
		if (!host) return;
		const p = new Player(host);
		p.onUpdate = (s) => (snap = s);
		player = p;
		return () => p.destroy();
	});

	// a new spec reloads; the same template keeps its position (tuning an option mid-review)
	$effect(() => {
		const p = player;
		const next = spec ? ($state.snapshot(spec) as ClipSpec) : null;
		if (!p || !next) return;
		const key = JSON.stringify(next);
		if (key === shownSpec) return;
		shownSpec = key;
		const at = next.template === shown ? untrack(() => snap.time) : 0;
		shown = next.template;
		failed = false;
		p.load(next, at)
			.then(() => (cues = p.cues))
			.catch(() => (failed = true));
	});

	const playing = $derived(snap.state === 'playing');

	function toggle() {
		if (!player || snap.state === 'empty' || snap.state === 'loading') return;
		if (playing) player.pause();
		else void player.play();
	}

	function step(frames: number) {
		if (!player) return;
		if (playing) player.pause();
		void player.step(frames);
	}

	function toggleMute() {
		muted = !muted;
		player?.setMuted(muted);
	}

	let box = $state<HTMLDivElement>();

	// keys while a player control has focus; buttons keep Space and the slider its arrows
	function onkeydown(e: KeyboardEvent) {
		const target = e.target as HTMLElement | null;
		if (!box || !target || !box.contains(target)) return;
		if (e.key === ' ' && target.tagName !== 'BUTTON') {
			e.preventDefault();
			toggle();
		} else if ((e.key === 'ArrowRight' || e.key === 'ArrowLeft') && target.tagName !== 'INPUT') {
			e.preventDefault();
			step(e.key === 'ArrowRight' ? 1 : -1);
		}
	}
</script>

<svelte:window {onkeydown} />

<div bind:this={box} class="overflow-hidden rounded-card border border-edge bg-black">
	<div class="relative">
		<div bind:this={host} class="aspect-video w-full"></div>
		{#if snap.state === 'loading' || failed}
			<p
				class="pointer-events-none absolute inset-0 grid place-items-center bg-black/60 text-xs font-medium
					tracking-widest text-muted uppercase"
				role="status"
			>
				{failed ? m.admin_couchtv_load_failed() : m.common_loading()}
			</p>
		{/if}
	</div>
	<div class="flex items-center gap-3 border-t border-edge bg-surface/60 px-4 py-3">
		<button
			type="button"
			onclick={toggle}
			aria-label={playing ? m.common_pause() : m.common_play()}
			class="flex size-9 shrink-0 items-center justify-center rounded-full bg-accent
				text-[var(--color-on-accent)] transition-colors hover:bg-accent-strong"
		>
			{#if playing}<Pause class="size-4" />{:else}<Play class="size-4" />{/if}
		</button>
		<button
			type="button"
			onclick={() => step(-1)}
			aria-label={m.admin_couchtv_frame_previous()}
			class="flex size-8 shrink-0 items-center justify-center rounded-full text-muted transition-colors
				hover:bg-surface-2 hover:text-text"
		>
			<StepBack class="size-4" />
		</button>
		<button
			type="button"
			onclick={() => step(1)}
			aria-label={m.admin_couchtv_frame_next()}
			class="flex size-8 shrink-0 items-center justify-center rounded-full text-muted transition-colors
				hover:bg-surface-2 hover:text-text"
		>
			<StepForward class="size-4" />
		</button>
		<div class="relative min-w-0 flex-1">
			<div class="pointer-events-none absolute inset-x-0 -top-1.5 h-1.5" aria-hidden="true">
				{#each cues as cue (cue.name + cue.at)}
					<i
						class="absolute top-0 h-1.5 w-px bg-faint"
						style="left: {snap.duration ? (cue.at / snap.duration) * 100 : 0}%"
						title={cue.name}
					></i>
				{/each}
			</div>
			<input
				type="range"
				min="0"
				max={snap.duration || 1}
				step="0.04"
				value={snap.time}
				oninput={(e) => player?.seek(Number(e.currentTarget.value))}
				aria-label={m.admin_couchtv_scrub()}
				class="w-full accent-accent"
			/>
		</div>
		<span class="w-28 shrink-0 text-right text-xs text-muted tnum">
			{snap.time.toFixed(2)} / {snap.duration.toFixed(2)} s
		</span>
		<button
			type="button"
			onclick={toggleMute}
			aria-label={muted ? m.player_unmute() : m.player_mute()}
			aria-pressed={muted}
			class="flex size-8 shrink-0 items-center justify-center rounded-full text-muted transition-colors
				hover:bg-surface-2 hover:text-text"
		>
			{#if muted}<VolumeX class="size-4" />{:else}<Volume2 class="size-4" />{/if}
		</button>
	</div>
	<p class="truncate border-t border-edge/60 px-4 py-2 text-[11px] text-faint">
		<span class="font-medium text-muted">{m.admin_couchtv_cue()}:</span>
		{snap.cue?.name ?? '-'}
	</p>
</div>
