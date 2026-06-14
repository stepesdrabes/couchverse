<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { fade } from 'svelte/transition';
	import { ApiError } from '$lib/api/client';
	import GlowBackdrop from '$lib/components/layout/GlowBackdrop.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import LogoMark from '$lib/components/ui/LogoMark.svelte';
	import * as m from '$lib/paraglide/messages';
	import { session } from '$lib/features/auth/session.svelte';

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
			error = err instanceof ApiError ? err.message : m.login_error_generic();
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>{m.login_page_title()}</title>
</svelte:head>

<div class="relative flex min-h-dvh items-center justify-center overflow-hidden px-4">
	<GlowBackdrop />

	<div class="relative w-full max-w-sm animate-slide-up">
		<div class="mb-8 flex flex-col items-center text-center">
			<LogoMark class="mb-5 size-16 rounded-2xl shadow-xl shadow-accent/25" />
			<h1 class="text-3xl font-extrabold tracking-tight">
				couch<span class="text-accent">verse</span>
			</h1>
			<p class="mt-2 text-sm text-muted">{m.login_subtitle()}</p>
		</div>

		<form
			onsubmit={submit}
			class="space-y-4 rounded-card border border-edge bg-surface/70 p-6 backdrop-blur"
		>
			<Input
				label={m.login_username()}
				name="username"
				autocomplete="username"
				bind:value={username}
				required
			/>
			<Input
				label={m.login_password()}
				name="password"
				type="password"
				autocomplete="current-password"
				bind:value={password}
				required
			/>
			{#if error}
				<p transition:fade={{ duration: 150 }} class="text-sm text-danger">{error}</p>
			{/if}
			<Button type="submit" loading={busy} class="w-full">{m.login_submit()}</Button>
		</form>

		<p class="mt-6 text-center text-xs text-faint">{m.login_accounts_note()}</p>
	</div>
</div>
