<script lang="ts">
	import { toast } from 'svelte-sonner';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Modal from '$lib/components/ui/Modal.svelte';
	import * as authApi from '../api';
	import * as m from '$lib/paraglide/messages';

	const MIN_PASSWORD = 8;

	let { open = $bindable(false) }: { open?: boolean } = $props();

	let currentPassword = $state('');
	let newPassword = $state('');
	let confirmPassword = $state('');
	let busy = $state(false);

	// clear on close so a reopened dialog never shows a stale password
	$effect(() => {
		if (!open) currentPassword = newPassword = confirmPassword = '';
	});

	const passwordError = $derived(
		newPassword.length > 0 && newPassword.length < MIN_PASSWORD
			? m.profile_password_too_short({ count: MIN_PASSWORD })
			: ''
	);
	const confirmError = $derived(
		confirmPassword.length > 0 && newPassword !== confirmPassword
			? m.profile_password_mismatch()
			: ''
	);
	const canSubmit = $derived(
		currentPassword.length > 0 &&
			newPassword.length >= MIN_PASSWORD &&
			newPassword === confirmPassword
	);

	async function submit(e: SubmitEvent) {
		e.preventDefault();
		if (!canSubmit) return;
		busy = true;
		try {
			await authApi.changePassword(currentPassword, newPassword);
			toast.success(m.profile_password_changed());
			open = false;
		} catch (err) {
			toast.error(err instanceof Error ? err.message : m.profile_password_change_failed());
		} finally {
			busy = false;
		}
	}
</script>

<Modal bind:open title={m.profile_password_heading()}>
	<form id="change-password" onsubmit={submit} class="space-y-4">
		<Input
			label={m.profile_current_password()}
			type="password"
			autocomplete="current-password"
			bind:value={currentPassword}
			required
		/>
		<Input
			label={m.profile_new_password()}
			type="password"
			autocomplete="new-password"
			bind:value={newPassword}
			error={passwordError}
			required
		/>
		<Input
			label={m.profile_confirm_password()}
			type="password"
			autocomplete="new-password"
			bind:value={confirmPassword}
			error={confirmError}
			required
		/>
	</form>

	{#snippet footer()}
		<Button type="button" variant="ghost" onclick={() => (open = false)}>{m.common_cancel()}</Button
		>
		<Button type="submit" form="change-password" loading={busy} disabled={!canSubmit}>
			{m.profile_password_heading()}
		</Button>
	{/snippet}
</Modal>
