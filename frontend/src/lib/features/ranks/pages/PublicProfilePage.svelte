<script lang="ts">
	import CachedView from '$lib/components/CachedView.svelte';
	import NotFound from '$lib/components/NotFound.svelte';
	import ProfileContent from '../components/ProfileContent.svelte';
	import ProfileSkeleton from '../components/ProfileSkeleton.svelte';
	import { profileCache } from '../cache.svelte';
	import * as m from '$lib/paraglide/messages';
	import type { Profile } from '../types';

	let { data }: { data: { key: string; fresh: Promise<Profile> } } = $props();
</script>

<CachedView value={profileCache.get(data.key)} fresh={data.fresh}>
	{#snippet content(profile)}
		<ProfileContent {profile} />
	{/snippet}
	{#snippet skeleton()}
		<ProfileSkeleton />
	{/snippet}
	{#snippet notFound()}
		<!-- an opted-out member 404s exactly like a missing one, which is what
		     makes the privacy switch an honest promise -->
		<NotFound title={m.profiles_not_found()} />
	{/snippet}
</CachedView>
