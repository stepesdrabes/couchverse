<script lang="ts">
	import { goto } from '$app/navigation';
	import GlowBackdrop from '$lib/components/layout/GlowBackdrop.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import { session } from '$lib/features/auth/session.svelte';
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
		<div class="mt-6 flex items-center justify-center gap-2">
			{#if session.isAdmin}
				<Button variant="primary" size="sm" onclick={() => goto('/admin')}>Server admin</Button>
			{/if}
			<Button variant="secondary" size="sm" onclick={() => session.logout()}>Sign out</Button>
		</div>
	</div>
</div>
