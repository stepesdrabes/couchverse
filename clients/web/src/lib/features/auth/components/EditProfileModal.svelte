<script lang="ts">
	import { untrack } from 'svelte';
	import { Camera, ImageUp, Trash2 } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Modal from '$lib/components/ui/Modal.svelte';
	import Switch from '$lib/components/ui/Switch.svelte';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import MarkdownEditor from '$lib/components/ui/MarkdownEditor.svelte';
	import { artworkUrl } from '$lib/features/catalog/api';
	import { features } from '$lib/features/settings/features.svelte';
	import { preferences } from '$lib/features/preferences/preferences.svelte';
	import { FormState } from '$lib/utils/form-state.svelte';
	import * as authApi from '../api';
	import { session } from '../session.svelte';
	import * as m from '$lib/paraglide/messages';

	const MAX_BIO = 2000;

	let { open = $bindable(false), onsaved }: { open?: boolean; onsaved?: () => void } = $props();

	let displayName = $state('');
	let bio = $state('');
	let saving = $state(false);
	let avatarInput = $state<HTMLInputElement>();
	let bannerInput = $state<HTMLInputElement>();
	const form = new FormState(() => ({ displayName, bio }));

	// images and the privacy switch persist the moment they change, so only the
	// two text fields are dirty-tracked behind Save
	let publicProfile = $state(true);

	// Seed once per opening, inside untrack: form.reset() reads the very fields
	// this effect writes, so without it the effect would re-run on every
	// keystroke and revert what the user just typed.
	let seeded = false;
	$effect(() => {
		if (!open) {
			seeded = false;
			return;
		}
		if (seeded) return;
		seeded = true;
		untrack(() => {
			displayName = session.user?.displayName ?? '';
			bio = session.user?.bio ?? '';
			publicProfile = preferences.publicProfile;
			form.reset();
		});
	});

	async function pickImage(kind: 'avatar' | 'banner', files: FileList | null) {
		const file = files?.[0];
		if (!file) return;
		try {
			session.user =
				kind === 'avatar' ? await authApi.uploadAvatar(file) : await authApi.uploadBanner(file);
			toast.success(m.profile_picture_updated());
			onsaved?.();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : m.profile_avatar_upload_failed());
		}
	}

	async function removeImage(kind: 'avatar' | 'banner') {
		try {
			session.user =
				kind === 'avatar' ? await authApi.deleteAvatar() : await authApi.deleteBanner();
			onsaved?.();
		} catch {
			toast.error(m.profile_avatar_remove_failed());
		}
	}

	async function savePrivacy(next: boolean) {
		const previous = publicProfile;
		publicProfile = next;
		try {
			await preferences.savePublicProfile(next);
			toast.success(m.profiles_privacy_saved());
			onsaved?.();
		} catch {
			publicProfile = previous;
			toast.error(m.profiles_privacy_failed());
		}
	}

	async function save(e: SubmitEvent) {
		e.preventDefault();
		if (bio.length > MAX_BIO) return;
		saving = true;
		try {
			session.user = await authApi.updateProfile(displayName.trim(), bio);
			form.reset();
			toast.success(m.profile_saved());
			onsaved?.();
			open = false;
		} catch (err) {
			toast.error(err instanceof Error ? err.message : m.profile_save_failed());
		} finally {
			saving = false;
		}
	}
</script>

<Modal bind:open title={m.profiles_edit_profile()} size="lg">
	<form id="edit-profile" onsubmit={save} class="space-y-5">
		<!-- banner first: it is the backdrop the avatar sits on, so editing reads
		     in the same order as the profile renders -->
		<div>
			<span class="mb-1.5 block text-xs font-medium text-muted">{m.profile_banner()}</span>
			<div
				class="group relative h-28 overflow-hidden rounded-input border border-edge bg-surface-2"
			>
				{#if session.user?.bannerId}
					<img
						src="{artworkUrl(session.user.bannerId)}?size=w780"
						alt=""
						class="size-full object-cover"
					/>
				{/if}
				<div
					class="absolute inset-0 flex items-center justify-center gap-2 bg-black/50 opacity-0
						transition-opacity group-hover:opacity-100 {session.user?.bannerId ? '' : 'opacity-100'}"
				>
					<Button type="button" variant="secondary" size="sm" onclick={() => bannerInput?.click()}>
						<ImageUp class="size-4" />
						{session.user?.bannerId ? m.profile_banner_replace() : m.profile_banner_upload()}
					</Button>
					{#if session.user?.bannerId}
						<Button
							type="button"
							variant="ghost"
							size="sm"
							onclick={() => removeImage('banner')}
							aria-label={m.profile_banner_remove()}
						>
							<Trash2 class="size-4" />
						</Button>
					{/if}
				</div>
			</div>
			<p class="mt-1 text-[11px] text-faint">{m.profile_banner_hint()}</p>
		</div>

		<div class="flex items-center gap-5">
			<button
				type="button"
				class="group relative shrink-0 overflow-hidden rounded-2xl"
				onclick={() => avatarInput?.click()}
				title={m.profile_change_picture()}
			>
				<UserAvatar
					name={session.user?.displayName ?? '?'}
					avatarId={session.user?.avatarId}
					seed={session.user?.username}
					class="size-20 rounded-2xl text-2xl"
				/>
				<span
					class="absolute inset-0 flex items-center justify-center bg-black/55 opacity-0
						transition-opacity group-hover:opacity-100"
				>
					<Camera class="size-5 text-white" />
				</span>
			</button>
			<div class="min-w-0 flex-1">
				<Input label={m.profile_display_name()} bind:value={displayName} required maxlength={60} />
				{#if session.user?.avatarId}
					<button
						type="button"
						class="mt-2 inline-flex items-center gap-1 text-xs text-faint transition-colors
							hover:text-danger"
						onclick={() => removeImage('avatar')}
					>
						<Trash2 class="size-3" />
						{m.profile_remove_picture()}
					</button>
				{/if}
			</div>
		</div>

		<div>
			<span class="mb-1.5 block text-xs font-medium text-muted">{m.profile_bio()}</span>
			<MarkdownEditor
				bind:value={bio}
				maxlength={MAX_BIO}
				placeholder={m.profile_bio_placeholder()}
			/>
		</div>

		{#if features.rankingsEnabled}
			<label
				class="flex items-center justify-between gap-4 rounded-input border border-edge px-3.5 py-2.5"
			>
				<span>
					<span class="block text-sm">{m.profiles_public_label()}</span>
					<span class="block text-[11px] text-faint">{m.profiles_public_hint()}</span>
				</span>
				<Switch bind:checked={publicProfile} onCheckedChange={savePrivacy} />
			</label>
		{/if}
	</form>

	{#snippet footer()}
		<Button type="button" variant="ghost" onclick={() => (open = false)}>{m.common_cancel()}</Button
		>
		<Button
			type="submit"
			form="edit-profile"
			loading={saving}
			disabled={!form.dirty || bio.length > MAX_BIO}
		>
			{m.common_save()}
		</Button>
	{/snippet}
</Modal>

<input
	bind:this={avatarInput}
	type="file"
	accept=".jpg,.jpeg,.png,.webp"
	class="hidden"
	onchange={(e) => {
		pickImage('avatar', e.currentTarget.files);
		e.currentTarget.value = '';
	}}
/>
<input
	bind:this={bannerInput}
	type="file"
	accept=".jpg,.jpeg,.png,.webp"
	class="hidden"
	onchange={(e) => {
		pickImage('banner', e.currentTarget.files);
		e.currentTarget.value = '';
	}}
/>
