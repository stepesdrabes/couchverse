<script lang="ts">
	import CachedView from '$lib/components/CachedView.svelte';
	import NotFound from '$lib/components/NotFound.svelte';
	import { useScreen } from '$lib/core/screen.svelte';
	import LoadFailed from '$lib/features/catalog/components/LoadFailed.svelte';
	import ProfileContent from '../components/ProfileContent.svelte';
	import ProfileSkeleton from '../components/ProfileSkeleton.svelte';
	import * as m from '$lib/paraglide/messages';
	import type { ProfileView, Surface } from '$lib/generated/core';

	let { data }: { data: { screen: Surface } } = $props();

	const profile = useScreen<ProfileView>(() => data.screen);
</script>

<CachedView value={profile.view?.profile} status={profile.view?.status}>
	{#snippet content(detail)}
		<ProfileContent profile={detail} />
	{/snippet}
	{#snippet skeleton()}
		<ProfileSkeleton />
	{/snippet}
	{#snippet notFound()}
		<!-- an opted-out member 404s exactly like a missing one, which is what
		     makes the privacy switch an honest promise -->
		<NotFound title={m.profiles_not_found()} />
	{/snippet}
	{#snippet failed()}
		<LoadFailed screen={data.screen} />
	{/snippet}
</CachedView>
