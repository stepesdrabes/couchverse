<script lang="ts">
	import { Camera, Trash2 } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import * as authApi from '$lib/features/auth/api';
	import { session } from '$lib/features/auth/session.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import { FormState } from '$lib/utils/form-state.svelte';

	let displayName = $state(session.user?.displayName ?? '');
	let saving = $state(false);
	let fileInput = $state<HTMLInputElement>();
	const form = new FormState(() => ({ displayName }));
	form.reset();

	async function save(e: SubmitEvent) {
		e.preventDefault();
		saving = true;
		try {
			session.user = await authApi.updateProfile(displayName.trim());
			form.reset();
			toast.success('Profile saved');
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to save profile');
		} finally {
			saving = false;
		}
	}

	async function uploadAvatar(files: FileList | null) {
		const file = files?.[0];
		if (!file) return;
		try {
			session.user = await authApi.uploadAvatar(file);
			toast.success('Profile picture updated');
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Avatar upload failed');
		}
	}

	async function removeAvatar() {
		try {
			session.user = await authApi.deleteAvatar();
		} catch {
			toast.error('Failed to remove avatar');
		}
	}
</script>

<svelte:head>
	<title>Profile - Couchverse</title>
</svelte:head>

<div class="mx-auto max-w-lg px-6 pt-28 pb-16">
	<h1 class="mb-8 text-2xl font-bold">Your profile</h1>

	<div class="rounded-card border border-edge bg-surface/40 p-6">
		<div class="mb-6 flex items-center gap-5">
			<button
				type="button"
				class="group relative shrink-0 overflow-hidden rounded-2xl"
				onclick={() => fileInput?.click()}
				title="Change profile picture"
			>
				<UserAvatar
					name={session.user?.displayName ?? '?'}
					avatarId={session.user?.avatarId}
					class="size-24 rounded-2xl text-3xl"
				/>
				<span
					class="absolute inset-0 flex items-center justify-center bg-black/55 opacity-0
						transition-opacity group-hover:opacity-100"
				>
					<Camera class="size-6 text-white" />
				</span>
			</button>
			<div class="min-w-0">
				<p class="truncate text-lg font-semibold">{session.user?.displayName}</p>
				<p class="text-sm text-faint">
					@{session.user?.username} · {session.user?.role === 'admin' ? 'Admin' : 'Member'}
				</p>
				{#if session.user?.avatarId}
					<button
						class="mt-2 inline-flex items-center gap-1 text-xs text-faint transition-colors hover:text-danger"
						onclick={removeAvatar}
					>
						<Trash2 class="size-3" />
						Remove picture
					</button>
				{/if}
			</div>
		</div>

		<form onsubmit={save} class="space-y-4">
			<Input label="Display name" bind:value={displayName} required maxlength={60} />
			<div class="flex justify-end">
				<Button type="submit" loading={saving} disabled={!form.dirty}>Save</Button>
			</div>
		</form>
	</div>

	<p class="mt-4 text-xs text-faint">
		Your name and picture show up in the top bar and on the admin's user list. Passwords are managed
		by the server admin.
	</p>
</div>

<input
	bind:this={fileInput}
	type="file"
	accept=".jpg,.jpeg,.png,.webp"
	class="hidden"
	onchange={(e) => {
		uploadAvatar(e.currentTarget.files);
		e.currentTarget.value = '';
	}}
/>
