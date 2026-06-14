<script lang="ts">
	import { Camera, Trash2 } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import * as authApi from '$lib/features/auth/api';
	import { session } from '$lib/features/auth/session.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import { FormState } from '$lib/utils/form-state.svelte';
	import * as m from '$lib/paraglide/messages';

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
			toast.success(m.profile_saved());
		} catch (err) {
			toast.error(err instanceof Error ? err.message : m.profile_save_failed());
		} finally {
			saving = false;
		}
	}

	async function uploadAvatar(files: FileList | null) {
		const file = files?.[0];
		if (!file) return;
		try {
			session.user = await authApi.uploadAvatar(file);
			toast.success(m.profile_picture_updated());
		} catch (err) {
			toast.error(err instanceof Error ? err.message : m.profile_avatar_upload_failed());
		}
	}

	async function removeAvatar() {
		try {
			session.user = await authApi.deleteAvatar();
		} catch {
			toast.error(m.profile_avatar_remove_failed());
		}
	}
</script>

<svelte:head>
	<title>{m.profile_page_title()}</title>
</svelte:head>

<div class="mx-auto max-w-lg px-6 pt-28 pb-16">
	<h1 class="mb-8 text-2xl font-bold">{m.profile_heading()}</h1>

	<div class="rounded-card border border-edge bg-surface/40 p-6">
		<div class="mb-6 flex items-center gap-5">
			<button
				type="button"
				class="group relative shrink-0 overflow-hidden rounded-2xl"
				onclick={() => fileInput?.click()}
				title={m.profile_change_picture()}
			>
				<UserAvatar
					name={session.user?.displayName ?? '?'}
					avatarId={session.user?.avatarId}
					seed={session.user?.username}
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
					@{session.user?.username} · {session.user?.role === 'admin'
						? m.profile_role_admin()
						: m.profile_role_member()}
				</p>
				{#if session.user?.avatarId}
					<button
						class="mt-2 inline-flex items-center gap-1 text-xs text-faint transition-colors hover:text-danger"
						onclick={removeAvatar}
					>
						<Trash2 class="size-3" />
						{m.profile_remove_picture()}
					</button>
				{/if}
			</div>
		</div>

		<form onsubmit={save} class="space-y-4">
			<Input label={m.profile_display_name()} bind:value={displayName} required maxlength={60} />
			<div class="flex justify-end">
				<Button type="submit" loading={saving} disabled={!form.dirty}>{m.common_save()}</Button>
			</div>
		</form>
	</div>

	<p class="mt-4 text-xs text-faint">
		{m.profile_help_text()}
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
