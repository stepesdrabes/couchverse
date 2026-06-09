<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { fade } from 'svelte/transition';
	import { ApiError } from '$lib/api/client';
	import GlowBackdrop from '$lib/components/layout/GlowBackdrop.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import { session } from '$lib/state/session.svelte';

	let username = $state('');
	let password = $state('');
	let error = $state('');
	let busy = $state(false);

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		if (busy) return;
		busy = true;
		error = '';
		try {
			await session.login(username, password);
			goto(page.url.searchParams.get('next') ?? '/');
		} catch (err) {
			error = err instanceof ApiError ? err.message : 'something went wrong, try again';
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>Sign in — Couchverse</title>
</svelte:head>

<div class="relative flex min-h-dvh items-center justify-center overflow-hidden px-4">
	<GlowBackdrop />

	<div class="relative w-full max-w-sm animate-slide-up">
		<div class="mb-8 text-center">
			<h1 class="text-3xl font-extrabold tracking-tight">
				couch<span class="text-accent">verse</span>
			</h1>
			<p class="mt-2 text-sm text-muted">Sign in to your library</p>
		</div>

		<form
			onsubmit={submit}
			class="space-y-4 rounded-card border border-edge bg-surface/70 p-6 backdrop-blur"
		>
			<Input
				label="Username"
				name="username"
				autocomplete="username"
				bind:value={username}
				required
			/>
			<Input
				label="Password"
				name="password"
				type="password"
				autocomplete="current-password"
				bind:value={password}
				required
			/>
			{#if error}
				<p transition:fade={{ duration: 150 }} class="text-sm text-danger">{error}</p>
			{/if}
			<Button type="submit" loading={busy} class="w-full">Sign in</Button>
		</form>

		<p class="mt-6 text-center text-xs text-faint">Accounts are created by the server admin.</p>
	</div>
</div>
