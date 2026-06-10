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
			toast.success(`Created account “${newUsername}”`);
			createOpen = false;
			newUsername = newPassword = '';
			newRole = 'member';
			refresh();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to create user');
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
			toast.success('User updated');
			editOpen = false;
			refresh();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to update user');
		} finally {
			busy = false;
		}
	}

	async function remove() {
		if (!deleting) return;
		try {
			await usersApi.deleteUser(deleting.id);
			toast.success(`Deleted “${deleting.username}”`);
			refresh();
		} catch (err) {
			toast.error(err instanceof Error ? err.message : 'Failed to delete user');
		}
	}
</script>

<svelte:head>
	<title>Users — Couchverse admin</title>
</svelte:head>

<div class="mb-6 flex items-center gap-4">
	<h1 class="text-2xl font-bold">Users</h1>
	<span
		class="rounded-full border border-edge bg-surface px-2.5 py-0.5 text-xs font-semibold text-muted tnum"
	>
		{users.length}
	</span>
	<div class="ml-auto">
		<Button size="sm" onclick={() => (createOpen = true)}>
			<Plus class="size-4" />
			New user
		</Button>
	</div>
</div>

<div class="overflow-hidden rounded-card border border-edge bg-surface/40">
	<table class="w-full text-left text-sm">
		<thead>
			<tr class="border-b border-edge text-[11px] tracking-wider text-faint uppercase">
				<th class="px-4 py-3 font-semibold">User</th>
				<th class="py-3 pr-4 font-semibold">Role</th>
				<th class="py-3 pr-4 font-semibold">Status</th>
				<th class="py-3 pr-4 font-semibold">Created</th>
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
								class="size-8 rounded-lg text-xs"
							/>
							<span>
								<span class="block font-semibold">
									{user.displayName}
									{#if user.id === session.user?.id}
										<span class="ml-1 text-[10px] font-normal text-faint">(you)</span>
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
								Admin
							</span>
						{:else}
							<span class="text-xs text-muted">Member</span>
						{/if}
					</td>
					<td class="py-3 pr-4">
						<span class="text-xs {user.disabled ? 'text-danger' : 'text-success'}">
							{user.disabled ? 'Disabled' : 'Active'}
						</span>
					</td>
					<td class="py-3 pr-4 text-xs text-muted tnum">{formatYearDate(user.createdAt)}</td>
					<td class="py-3 pr-4 text-right">
						<Button variant="ghost" size="sm" onclick={() => openEdit(user)}>Edit</Button>
						{#if user.id !== session.user?.id}
							<Button
								variant="ghost"
								size="sm"
								onclick={() => {
									deleting = user;
									confirmDelete = true;
								}}
							>
								Delete
							</Button>
						{/if}
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>

<Modal
	bind:open={createOpen}
	title="New user"
	description="Create an account for someone in your household."
>
	<form onsubmit={create} class="space-y-4">
		<Input label="Username" bind:value={newUsername} required autocomplete="off" />
		<Input
			label="Password"
			type="password"
			bind:value={newPassword}
			required
			autocomplete="new-password"
		/>
		<Select
			bind:value={newRole}
			label="Role"
			items={[
				{ value: 'member', label: 'Member' },
				{ value: 'admin', label: 'Admin' }
			]}
		/>
		<div class="flex justify-end pt-2">
			<Button type="submit" loading={busy}>Create user</Button>
		</div>
	</form>
</Modal>

<Modal bind:open={editOpen} title="Edit {editing?.username}">
	<form onsubmit={saveEdit} class="space-y-4">
		<Input label="Display name" bind:value={editDisplayName} />
		<Select
			bind:value={editRole}
			label="Role"
			items={[
				{ value: 'member', label: 'Member' },
				{ value: 'admin', label: 'Admin' }
			]}
		/>
		<Input
			label="New password (leave empty to keep)"
			type="password"
			bind:value={editPassword}
			autocomplete="new-password"
		/>
		<label class="flex items-center justify-between rounded-input border border-edge px-3.5 py-2.5">
			<span class="text-sm">Disabled</span>
			<Switch bind:checked={editDisabled} />
		</label>
		<div class="flex justify-end pt-2">
			<Button type="submit" loading={busy}>Save</Button>
		</div>
	</form>
</Modal>

<Confirm
	bind:open={confirmDelete}
	title="Delete “{deleting?.username}”?"
	message="Their watch progress, lists and playlists are removed permanently."
	onconfirm={remove}
/>
