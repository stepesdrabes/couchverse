<script lang="ts">
	import { Copy, Link } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import { problemMessage } from '$lib/api/problem';
	import Button from '$lib/components/ui/Button.svelte';
	import Confirm from '$lib/components/ui/Confirm.svelte';
	import Switch from '$lib/components/ui/Switch.svelte';
	import { currentLang } from '$lib/i18n/locale.svelte';
	import { formatRelative } from '$lib/utils/format';
	import * as m from '$lib/paraglide/messages';
	import { adminRotateCouchTvLink, adminSetCouchTvLink, type CouchTvGuestLink } from '../api';

	// The channel link (D23): one unguessable link for viewers without an account. Only its
	// hash is kept, so a new link is shown once; making one ends the old link and its guests.
	let { link: initial }: { link: CouchTvGuestLink } = $props();

	// svelte-ignore state_referenced_locally
	let link = $state(initial);
	// the switch's own state, put back to the link's when the server says otherwise
	let enabled = $state(initial.enabled);
	let made = $state('');
	let busy = $state(false);
	let confirming = $state(false);

	const exists = $derived(!!link.createdAt);
	const lang = $derived(currentLang());

	async function rotate() {
		busy = true;
		try {
			const answer = await adminRotateCouchTvLink();
			link = answer.link;
			enabled = link.enabled;
			made = new URL(answer.path, location.origin).href;
		} catch (err) {
			toast.error(problemMessage(err, m.admin_couchtv_guest_link_failed()));
		} finally {
			busy = false;
		}
	}

	async function toggle(on: boolean) {
		busy = true;
		try {
			link = await adminSetCouchTvLink({ enabled: on });
		} catch (err) {
			toast.error(problemMessage(err, m.admin_couchtv_guest_link_failed()));
		} finally {
			enabled = link.enabled;
			busy = false;
		}
	}

	async function copy() {
		await navigator.clipboard.writeText(made);
		toast.success(m.couch_link_copied());
	}
</script>

<section class="rounded-card border border-edge bg-surface/40 p-5" aria-labelledby="guest-link">
	<div class="flex flex-wrap items-start justify-between gap-4">
		<div class="max-w-xl">
			<h2 id="guest-link" class="text-sm font-semibold text-muted">
				{m.admin_couchtv_guest_link()}
			</h2>
			<p class="mt-1 text-xs text-faint">{m.admin_couchtv_guest_link_hint()}</p>
			<p class="mt-3 flex items-center gap-2 text-sm">
				<Link class="size-4 text-muted" />
				{#if !exists}
					{m.admin_couchtv_guest_link_none()}
				{:else if link.enabled && link.createdAt}
					{m.admin_couchtv_guest_link_made({ when: formatRelative(link.createdAt, lang) })}
				{:else}
					{m.admin_couchtv_guest_link_off()}
				{/if}
			</p>
		</div>
		<div class="flex flex-wrap items-center gap-4">
			{#if exists}
				<label class="flex items-center gap-2.5 text-sm">
					<Switch
						bind:checked={enabled}
						label={m.admin_couchtv_guest_link_switch()}
						onCheckedChange={toggle}
					/>
					{m.admin_couchtv_guest_link_switch()}
				</label>
			{/if}
			<Button
				variant="secondary"
				size="sm"
				loading={busy}
				onclick={() => (exists ? (confirming = true) : rotate())}
			>
				{exists ? m.admin_couchtv_guest_link_new() : m.admin_couchtv_guest_link_make()}
			</Button>
		</div>
	</div>

	{#if made}
		<div class="mt-4 space-y-2 rounded-xl border border-edge bg-surface p-3">
			<div class="flex flex-wrap items-center gap-2">
				<input
					readonly
					value={made}
					aria-label={m.admin_couchtv_guest_link_url()}
					class="h-9 min-w-0 flex-1 rounded-input border border-edge bg-bg px-3 font-mono text-xs"
					onfocus={(e) => e.currentTarget.select()}
				/>
				<Button size="sm" onclick={copy}>
					<Copy class="size-3.5" />
					{m.couch_copy_link()}
				</Button>
			</div>
			<p class="text-xs text-muted">{m.admin_couchtv_guest_link_once()}</p>
		</div>
	{/if}

	<Confirm
		bind:open={confirming}
		title={m.admin_couchtv_guest_link_new_title()}
		message={m.admin_couchtv_guest_link_new_message()}
		confirmLabel={m.admin_couchtv_guest_link_new()}
		onconfirm={rotate}
	/>
</section>
