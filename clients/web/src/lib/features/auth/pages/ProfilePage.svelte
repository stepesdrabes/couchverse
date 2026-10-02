<script lang="ts">
	import { session } from '$lib/features/auth/session.svelte';
	import { currentLang } from '$lib/i18n/locale.svelte';
	import ProfileContent from '$lib/features/ranks/components/ProfileContent.svelte';
	import ProfileSkeleton from '$lib/features/ranks/components/ProfileSkeleton.svelte';
	import { profileCache } from '$lib/features/ranks/cache.svelte';
	import { getMyStats } from '$lib/features/ranks/api';
	import { rank } from '$lib/features/ranks/rank.svelte';
	import { features } from '$lib/features/settings/features.svelte';
	import EditProfileModal from '../components/EditProfileModal.svelte';
	import ChangePasswordModal from '../components/ChangePasswordModal.svelte';
	import AccountOnlyProfile from '../components/AccountOnlyProfile.svelte';
	import * as m from '$lib/paraglide/messages';
	import type { Profile } from '$lib/features/ranks/types';

	// Your own profile *is* the public one, just with edit affordances - same
	// component, same cache entry as /u/you, so the hop between them is free.
	const key = $derived(`${session.user?.username ?? ''}|${currentLang()}`);
	const profile = $derived<Profile | undefined>(profileCache.get(key));

	let editOpen = $state(false);
	let passwordOpen = $state(false);

	function refresh() {
		profileCache
			.revalidate(key, getMyStats)
			.then((p) => rank.seed(p.rank))
			.catch(() => {});
	}

	$effect(() => {
		if (!profile && session.user) refresh();
	});
</script>

<svelte:head>
	<title>{m.profile_page_title()}</title>
</svelte:head>

{#if !features.rankingsEnabled}
	<!-- with progression off there is no profile to show, so this falls back to
	     the plain account surface -->
	<AccountOnlyProfile onedit={() => (editOpen = true)} onpassword={() => (passwordOpen = true)} />
{:else if profile}
	<ProfileContent
		{profile}
		onedit={() => (editOpen = true)}
		onpassword={() => (passwordOpen = true)}
	/>
{:else}
	<ProfileSkeleton />
{/if}

<EditProfileModal bind:open={editOpen} onsaved={refresh} />
<ChangePasswordModal bind:open={passwordOpen} />
