<script lang="ts">
	import { goto } from '$app/navigation';
	import { Play } from 'lucide-svelte';
	import { onMount } from 'svelte';
	import { core } from '$lib/core';
	import { problemMessage } from '$lib/api/problem';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import { session } from '$lib/features/auth/session.svelte';
	import type { CouchTvView } from '$lib/generated/core';
	import * as m from '$lib/paraglide/messages';
	import { COUCH_TV, joinAsGuest } from '../channel';
	import ChannelScreen from '../components/ChannelScreen.svelte';

	// The channel link's page: a viewer without an account types the name everyone watching
	// sees, the core joins as a guest (the channel cookie remembers them) and the live channel
	// takes over. A member watches as themselves.
	let { code }: { code: string } = $props();

	$effect(() => core.watch(COUCH_TV));
	const view = $derived(core.view<CouchTvView>(COUCH_TV));

	onMount(() => {
		if (session.user) goto('/tv', { replaceState: true });
	});

	let name = $state('');
	let tried = $state(false);
	const joining = $derived(view?.joining ?? false);
	const error = $derived(
		tried && !joining && view?.problem ? problemMessage(view.problem, m.couchtv_join_failed()) : ''
	);

	async function join(e: SubmitEvent) {
		e.preventDefault();
		tried = true;
		await joinAsGuest(code, name);
		if (!view?.problem) goto('/tv', { replaceState: true });
	}
</script>

<svelte:head>
	<title>{m.couchtv_join_page_title()}</title>
</svelte:head>

<ChannelScreen title={m.couchtv_join_title()} message={m.couchtv_join_hint()}>
	<form class="flex w-full max-w-xs flex-col gap-3 text-left" onsubmit={join}>
		<Input
			label={m.couchtv_join_name()}
			bind:value={name}
			maxlength={32}
			required
			autocomplete="nickname"
			error={error || undefined}
			data-tv-autofocus
		/>
		<Button type="submit" size="lg" loading={joining} disabled={!name.trim()}>
			<Play class="size-4 fill-current" />
			{m.couchtv_join_submit()}
		</Button>
	</form>
</ChannelScreen>
