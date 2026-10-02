<script lang="ts">
	import { Plus, ShieldCheck } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import * as usersApi from '$lib/features/users/api';
	import type { User } from '$lib/features/auth/api';
	import Button from '$lib/components/ui/Button.svelte';
	import Confirm from '$lib/components/ui/Confirm.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import Modal from '$lib/components/ui/Modal.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import Switch from '$lib/components/ui/Switch.svelte';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import { session } from '$lib/features/auth/session.svelte';
	import { formatYearDate } from '$lib/utils/format';
	import { FormState } from '$lib/utils/form-state.svelte';
	import * as m from '$lib/paraglide/messages';

	let users = $state<User[]>([]);

	let createOpen = $state(false);
	let newUsername = $state('');
	let newPassword = $state('');
	let newRole = $state('member');
	let busy = $state(false);

	let editing = $state<User | null>(null);
	let editOpen = $state(false);
	let editDisplayName = $state('');
	let editRole = $state('member');
	let editDisabled = $state(false);
	let editPassword = $state('');
	const editForm = new FormState(() => ({ editDisplayName, editRole, editDisabled, editPassword }));

	let deleting = $state<User | null>(null);
	let confirmDelete = $state(false);

	async function refresh() {
		users = await usersApi.listUsers();
	}
	refresh();

	async function create(e: SubmitEvent) {
		e.preventDefault();
		busy = true;
		try {
			await usersApi.createUser({ username: newUsername, password: newPassword, role: newRole });
			toast.success(m.users_created_account({ username: newUsername }));
			createOpen = false;
			newUsername = newPassword = '';
			newRole = 'member';
			refresh();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : m.users_create_failed());
		} finally {
			busy = false;
		}
	}

	function openEdit(user: User) {
		editing = user;
		editDisplayName = user.displayName;
		editRole = user.role;
		editDisabled = user.disabled;
		editPassword = '';
		editForm.reset();
		editOpen = true;
	}

	async function saveEdit(e: SubmitEvent) {
		e.preventDefault();
		if (!editing) return;
		busy = true;
		try {
			await usersApi.updateUser(editing.id, {
				displayName: editDisplayName,
				role: editRole,
				disabled: editDisabled,
				...(editPassword ? { password: editPassword } : {})
			});
			toast.success(m.users_updated());
			editOpen = false;
			refresh();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : m.users_update_failed());
		} finally {
			busy = false;
		}
	}

	async function remove() {
		if (!deleting) return;
		try {
			await usersApi.deleteUser(deleting.id);
			toast.success(m.users_deleted({ username: deleting.username }));
			refresh();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : m.users_delete_failed());
		}
	}
</script>

<svelte:head>
	<title>{m.users_page_title()}</title>
</svelte:head>

<div class="mb-6 flex items-center gap-4">
	<h1 class="text-2xl font-bold">{m.users_heading()}</h1>
	<span
		class="rounded-full border border-edge bg-surface px-2.5 py-0.5 text-xs font-semibold text-muted tnum"
	>
		{users.length}
	</span>
	<div class="ml-auto">
		<Button size="sm" onclick={() => (createOpen = true)}>
			<Plus class="size-4" />
			{m.users_new_user()}
		</Button>
	</div>
</div>

<div class="overflow-hidden rounded-card border border-edge bg-surface/40">
	<table class="w-full text-left text-sm">
		<thead>
			<tr class="border-b border-edge text-[11px] tracking-wider text-faint uppercase">
				<th class="px-4 py-3 font-semibold">{m.users_col_user()}</th>
				<th class="py-3 pr-4 font-semibold">{m.users_col_role()}</th>
				<th class="py-3 pr-4 font-semibold">{m.common_status()}</th>
				<th class="py-3 pr-4 font-semibold">{m.users_col_created()}</th>
				<th class="w-32 py-3 pr-4"></th>
			</tr>
		</thead>
		<tbody>
			{#each users as user (user.id)}
				<tr class="border-b border-edge/50 transition-colors last:border-0 hover:bg-surface-2/40">
					<td class="px-4 py-3">
						<span class="flex items-center gap-3">
							<UserAvatar
								name={user.displayName}
								avatarId={user.avatarId}
								seed={user.username}
								class="size-8 rounded-lg text-xs"
							/>
							<span>
								<span class="block font-semibold">
									{user.displayName}
									{#if user.id === session.user?.id}
										<span class="ml-1 text-[10px] font-normal text-faint">{m.users_you()}</span>
									{/if}
								</span>
								<span class="block text-xs text-faint">@{user.username}</span>
							</span>
						</span>
					</td>
					<td class="py-3 pr-4">
						{#if user.role === 'admin'}
							<span class="inline-flex items-center gap-1 text-xs font-semibold text-accent">
								<ShieldCheck class="size-3.5" />
								{m.users_role_admin()}
							</span>
						{:else}
							<span class="text-xs text-muted">{m.users_role_member()}</span>
						{/if}
					</td>
					<td class="py-3 pr-4">
						<span class="text-xs {user.disabled ? 'text-danger' : 'text-success'}">
							{user.disabled ? m.users_status_disabled() : m.users_status_active()}
						</span>
					</td>
					<td class="py-3 pr-4 text-xs text-muted tnum">{formatYearDate(user.createdAt)}</td>
					<td class="py-3 pr-4 text-right">
						<Button variant="ghost" size="sm" onclick={() => openEdit(user)}
							>{m.common_edit()}</Button
						>
						{#if user.id !== session.user?.id}
							<Button
								variant="ghost"
								size="sm"
								onclick={() => {
									deleting = user;
									confirmDelete = true;
								}}
							>
								{m.common_delete()}
							</Button>
						{/if}
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>

<Modal bind:open={createOpen} title={m.users_new_user()} description={m.users_create_description()}>
	<form onsubmit={create} class="space-y-4">
		<Input label={m.users_username_label()} bind:value={newUsername} required autocomplete="off" />
		<Input
			label={m.users_password_label()}
			type="password"
			bind:value={newPassword}
			required
			autocomplete="new-password"
		/>
		<Select
			bind:value={newRole}
			label={m.users_col_role()}
			items={[
				{ value: 'member', label: m.users_role_member() },
				{ value: 'admin', label: m.users_role_admin() }
			]}
		/>
		<div class="flex justify-end pt-2">
			<Button type="submit" loading={busy} disabled={!newUsername.trim() || !newPassword}>
				{m.users_create_user()}
			</Button>
		</div>
	</form>
</Modal>

<Modal bind:open={editOpen} title={m.users_edit_title({ username: editing?.username ?? '' })}>
	<form onsubmit={saveEdit} class="space-y-4">
		<Input label={m.users_display_name_label()} bind:value={editDisplayName} />
		<Select
			bind:value={editRole}
			label={m.users_col_role()}
			items={[
				{ value: 'member', label: m.users_role_member() },
				{ value: 'admin', label: m.users_role_admin() }
			]}
		/>
		<Input
			label={m.users_new_password_label()}
			type="password"
			bind:value={editPassword}
			autocomplete="new-password"
		/>
		<label class="flex items-center justify-between rounded-input border border-edge px-3.5 py-2.5">
			<span class="text-sm">{m.users_status_disabled()}</span>
			<Switch bind:checked={editDisabled} />
		</label>
		<div class="flex justify-end pt-2">
			<Button type="submit" loading={busy} disabled={!editForm.dirty}>{m.common_save()}</Button>
		</div>
	</form>
</Modal>

<Confirm
	bind:open={confirmDelete}
	title={m.users_delete_confirm_title({ username: deleting?.username ?? '' })}
	message={m.users_delete_confirm_message()}
	onconfirm={remove}
/>
