<script lang="ts">
	import CachedView from '$lib/components/CachedView.svelte';
	import { useScreen } from '$lib/core/screen.svelte';
	import { session } from '$lib/features/auth/session.svelte';
	import LoadFailed from '$lib/features/catalog/components/LoadFailed.svelte';
	import ProfileContent from '$lib/features/ranks/components/ProfileContent.svelte';
	import ProfileSkeleton from '$lib/features/ranks/components/ProfileSkeleton.svelte';
	import { profileScreen } from '$lib/features/ranks/api';
	import { features } from '$lib/features/settings/features.svelte';
	import EditProfileModal from '../components/EditProfileModal.svelte';
	import ChangePasswordModal from '../components/ChangePasswordModal.svelte';
	import AccountOnlyProfile from '../components/AccountOnlyProfile.svelte';
	import DevicesSection from '../components/DevicesSection.svelte';
	import * as m from '$lib/paraglide/messages';
	import type { ProfileView } from '$lib/generated/core';

	// Your own profile *is* the public one, just with edit affordances: the same screen in the
	// core as /u/you, so the hop between them is free.
	const screen = $derived(
		features.rankingsEnabled && session.user ? profileScreen(session.user.username) : undefined
	);
	const profile = useScreen<ProfileView>(() => screen);

	let editOpen = $state(false);
	let passwordOpen = $state(false);
</script>

<svelte:head>
	<title>{m.profile_page_title()}</title>
</svelte:head>

{#if !screen}
	<!-- with progression off there is no profile to show, so this falls back to
	     the plain account surface -->
	<AccountOnlyProfile onedit={() => (editOpen = true)} onpassword={() => (passwordOpen = true)} />
{:else}
	<CachedView value={profile.view?.profile} status={profile.view?.status}>
		{#snippet content(detail)}
			<ProfileContent
				profile={detail}
				onedit={() => (editOpen = true)}
				onpassword={() => (passwordOpen = true)}
			>
				<DevicesSection />
			</ProfileContent>
		{/snippet}
		{#snippet skeleton()}
			<ProfileSkeleton />
		{/snippet}
		{#snippet failed()}
			<LoadFailed {screen} />
		{/snippet}
	</CachedView>
{/if}

<EditProfileModal bind:open={editOpen} />
<ChangePasswordModal bind:open={passwordOpen} />
