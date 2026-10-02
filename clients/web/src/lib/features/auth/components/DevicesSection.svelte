<script lang="ts">
	import { onMount, tick } from 'svelte';
	import { fly } from 'svelte/transition';
	import { KeyRound, LogOut, QrCode, RefreshCw } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import { ApiError } from '$lib/api/client';
	import Button from '$lib/components/ui/Button.svelte';
	import Confirm from '$lib/components/ui/Confirm.svelte';
	import Skeleton from '$lib/components/ui/Skeleton.svelte';
	import { currentLang } from '$lib/i18n/locale.svelte';
	import { formatRelative, formatYearDate } from '$lib/utils/format';
	import * as authApi from '../api';
	import type { Device } from '../api';
	import { devicesCache } from '../cache.svelte';
	import { platformIcon, platformName } from '../platforms';
	import { session } from '../session.svelte';
	import ConnectDeviceModal from './ConnectDeviceModal.svelte';
	import * as m from '$lib/paraglide/messages';

	// Every browser and app signed in to the account; on the owner's /profile only.

	// The server records activity at most every five minutes, so anything seen within
	// that window is as good as in use.
	const ACTIVE_WINDOW_MS = 5 * 60 * 1000;

	const key = $derived(session.user?.username ?? '');
	const devices = $derived(devicesCache.get(key));
	// this browser first, then the server's most recently active order
	const sorted = $derived(
		devices ? [...devices].sort((a, b) => Number(b.current) - Number(a.current)) : []
	);

	let failed = $state(false);
	let connectOpen = $state(false);
	let revoking = $state<Device | null>(null);
	let confirmOpen = $state(false);
	let busyId = $state<string | null>(null);
	let section = $state<HTMLElement>();

	function activity(device: Device): string {
		if (device.current || Date.now() - Date.parse(device.lastSeenAt) < ACTIVE_WINDOW_MS) {
			return m.devices_active_now();
		}
		return m.devices_last_seen({ time: formatRelative(device.lastSeenAt, currentLang()) });
	}

	// A browser's name already says what it is. The no-break space keeps the dot on
	// the first line when a narrow screen wraps this.
	const status = (device: Device) =>
		device.kind === 'device'
			? `${platformName(device.platform)}\u00a0· ${activity(device)}`
			: activity(device);

	async function load() {
		failed = false;
		try {
			await devicesCache.revalidate(key, () => authApi.listDevices());
		} catch {
			failed = !devicesCache.get(key);
		}
	}

	function drop(id: string) {
		devicesCache.set(
			key,
			(devicesCache.get(key) ?? []).filter((d) => d.id !== id)
		);
	}

	function askRevoke(device: Device) {
		revoking = device;
		confirmOpen = true;
	}

	async function revoke() {
		const device = revoking;
		if (!device) return;
		// signing this browser out is a logout, which also clears its cookie
		if (device.current) return session.logout();
		busyId = device.id;
		try {
			await authApi.revokeDevice(device.id);
			drop(device.id);
			toast.success(m.devices_signed_out({ device: device.name }));
		} catch (err) {
			// already signed out somewhere else
			if (err instanceof ApiError && err.status === 404) drop(device.id);
			else toast.error(m.devices_sign_out_failed());
		} finally {
			busyId = null;
		}
	}

	function connected(list: Device[], added: Device) {
		devicesCache.set(key, list);
		toast.success(m.devices_connect_connected({ device: added.name }));
	}

	onMount(async () => {
		await load();
		// /pair links here once a device is approved
		if (location.hash === '#devices') {
			await tick();
			section?.scrollIntoView({ block: 'start' });
		}
	});
</script>

<section
	id="devices"
	bind:this={section}
	class="mt-10 scroll-mt-28"
	in:fly|global={{ y: 20, duration: 400, delay: 120 }}
>
	<div class="mb-4 flex flex-wrap items-end gap-3">
		<div class="min-w-0">
			<h2 class="text-sm font-semibold text-muted">{m.devices_heading()}</h2>
			{#if devices}
				<p class="mt-1 text-xs text-faint">
					{m.devices_summary({ count: devices.length })}
					{#if devices.length > 1}{m.devices_hint()}{/if}
				</p>
			{/if}
		</div>
		<div class="ml-auto flex flex-wrap gap-2">
			<a
				href="/pair"
				class="inline-flex h-8 items-center justify-center gap-1.5 rounded-full border border-edge
					bg-surface/60 px-4 text-xs font-semibold transition-colors hover:border-faint
					hover:bg-surface-2 focus-visible:border-faint focus-visible:bg-surface-2"
			>
				<KeyRound class="size-4" />
				{m.devices_enter_code()}
			</a>
			<Button variant="secondary" size="sm" onclick={() => (connectOpen = true)}>
				<QrCode class="size-4" />
				{m.devices_connect()}
			</Button>
		</div>
	</div>

	<div class="overflow-hidden rounded-card border border-edge bg-surface/40">
		{#if failed}
			<div class="flex flex-col items-center gap-3 px-6 py-10 text-center">
				<p class="text-sm text-muted">{m.devices_load_failed()}</p>
				<Button variant="secondary" size="sm" onclick={load}>
					<RefreshCw class="size-4" />
					{m.common_retry()}
				</Button>
			</div>
		{:else if !devices}
			<ul class="divide-y divide-edge/60" aria-hidden="true">
				{#each [0, 1] as i (i)}
					<li class="flex items-center gap-4 px-4 py-4 sm:px-5">
						<Skeleton class="size-11 shrink-0 rounded-xl" />
						<div class="flex-1 space-y-2">
							<Skeleton class="h-3.5 w-40" />
							<Skeleton class="h-3 w-64 max-w-full" />
						</div>
					</li>
				{/each}
			</ul>
		{:else}
			<ul class="divide-y divide-edge/60">
				{#each sorted as device (device.id)}
					{@const Icon = platformIcon(device.platform)}
					<li
						class="flex items-center gap-4 px-4 py-3.5 sm:px-5"
						out:fly={{ x: -16, duration: 200 }}
					>
						<span
							class="flex size-11 shrink-0 items-center justify-center rounded-xl border
								{device.current
								? 'border-accent/40 bg-accent/10 text-accent'
								: 'border-edge bg-surface-2 text-muted'}"
						>
							<Icon class="size-5" />
						</span>

						<div class="min-w-0 flex-1">
							<div class="flex min-w-0 items-center gap-2">
								<p class="truncate text-sm font-semibold">{device.name}</p>
								{#if device.current}
									<span
										class="shrink-0 rounded-full bg-accent/15 px-2 py-0.5 text-[11px] font-semibold
											text-accent"
									>
										{m.devices_this_browser()}
									</span>
								{/if}
							</div>
							<p class="mt-0.5 text-xs text-faint">{status(device)}</p>
							<p class="mt-0.5 text-xs text-faint/80">
								{m.devices_signed_in_on({ date: formatYearDate(device.createdAt, currentLang()) })}
							</p>
						</div>

						<button
							type="button"
							disabled={busyId === device.id}
							onclick={() => askRevoke(device)}
							aria-label={m.devices_sign_out_device({ device: device.name })}
							class="flex h-8 shrink-0 cursor-pointer items-center gap-1.5 rounded-full px-3 text-xs
								font-semibold text-muted transition-colors hover:bg-danger/10 hover:text-danger
								focus-visible:bg-danger/10 focus-visible:text-danger disabled:opacity-50"
						>
							<LogOut class="size-4" />
							<span class="max-sm:hidden">{m.devices_sign_out()}</span>
						</button>
					</li>
				{/each}
			</ul>
		{/if}
	</div>
</section>

<ConnectDeviceModal bind:open={connectOpen} onconnected={connected} />

<Confirm
	bind:open={confirmOpen}
	title={revoking?.current
		? m.devices_sign_out_current_title()
		: m.devices_sign_out_title({ device: revoking?.name ?? '' })}
	message={revoking?.current ? m.devices_sign_out_current_message() : m.devices_sign_out_message()}
	confirmLabel={m.devices_sign_out()}
	onconfirm={revoke}
/>
