<script lang="ts">
	import GlowBackdrop from '$lib/components/layout/GlowBackdrop.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import { session } from '$lib/state/session.svelte';
</script>

<div class="relative flex min-h-dvh items-center justify-center overflow-hidden">
	<GlowBackdrop />
	<div class="relative animate-slide-up text-center">
		<p class="eyebrow mb-4">Self-hosted streaming</p>
		<h1 class="text-4xl font-extrabold tracking-tight">
			couch<span class="text-accent">verse</span>
		</h1>
		<p class="mt-3 text-sm text-muted">
			Signed in as {session.user?.displayName} · the library arrives in the next milestone
		</p>
		<Button variant="secondary" size="sm" class="mt-6" onclick={() => session.logout()}>
			Sign out
		</Button>
	</div>
</div>
