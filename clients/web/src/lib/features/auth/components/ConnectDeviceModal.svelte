<script lang="ts">
	import { untrack } from 'svelte';
	import { Clock, Copy, LoaderCircle, RefreshCw, TriangleAlert } from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import Button from '$lib/components/ui/Button.svelte';
	import Modal from '$lib/components/ui/Modal.svelte';
	import QrCode from '$lib/components/ui/QrCode.svelte';
	import { formatClock } from '$lib/utils/format';
	import * as authApi from '../api';
	import type { Device } from '../api';
	import * as m from '$lib/paraglide/messages';

	// A one-time code as a QR for the phone app, which adds this server and signs in
	// with it. A code works once and expires, so every opening asks for a fresh one
	// and an expired one is replaced on the spot.

	let {
		open = $bindable(false),
		onconnected
	}: {
		open?: boolean;
		/** a device signed in while the code was up; the dialog closes itself */
		onconnected?: (devices: Device[], added: Device) => void;
	} = $props();

	let connect = $state<{ code: string; expiresAt: number } | null>(null);
	let loading = $state(false);
	let failed = $state(false);
	let now = $state(Date.now());
	// bumped on close, so a code that lands after the dialog closed is dropped
	let generation = 0;

	const server = location.origin;
	// a phone resolves "localhost" to itself, so such a QR can never work
	const loopback = /^(localhost|127\.\d+\.\d+\.\d+|\[::1\])$|\.localhost$/.test(location.hostname);
	const canCopy = !!navigator.clipboard;

	const link = $derived(
		connect ? `couchverse://connect?${new URLSearchParams({ server, code: connect.code })}` : ''
	);
	const remaining = $derived(
		connect ? Math.max(0, Math.ceil((connect.expiresAt - now) / 1000)) : 0
	);

	async function refresh() {
		const mine = generation;
		loading = true;
		failed = false;
		try {
			const c = await authApi.createConnectCode();
			if (mine !== generation) return;
			connect = { code: c.code, expiresAt: Date.now() + c.expiresIn * 1000 };
			now = Date.now();
		} catch {
			if (mine === generation) failed = true;
		} finally {
			if (mine === generation) loading = false;
		}
	}

	async function copy() {
		try {
			await navigator.clipboard.writeText(link);
			toast.success(m.devices_connect_copied());
		} catch {
			toast.error(m.devices_connect_copy_failed());
		}
	}

	$effect(() => {
		if (!open) return;
		untrack(refresh);
		const clock = setInterval(() => {
			now = Date.now();
			if (connect && !loading && now >= connect.expiresAt) refresh();
		}, 1000);

		// Watch for the phone that scans the code: anything not signed in when the
		// dialog opened is it.
		let known: Set<string> | null = null;
		const watch = async () => {
			const list = await authApi.listDevices().catch(() => null);
			if (!list || !open) return;
			if (!known) {
				known = new Set(list.map((d) => d.id));
				return;
			}
			const added = list.find((d) => !known?.has(d.id));
			if (!added) return;
			open = false;
			onconnected?.(list, added);
		};
		watch();
		const watcher = setInterval(watch, 3000);

		return () => {
			clearInterval(clock);
			clearInterval(watcher);
			generation++;
			connect = null;
			loading = failed = false;
		};
	});
</script>

<Modal bind:open title={m.devices_connect()} description={m.devices_connect_scan()}>
	<div class="flex flex-col items-center">
		<div
			class="relative flex size-60 items-center justify-center overflow-hidden rounded-2xl bg-white
				shadow-xl shadow-black/40"
		>
			{#if failed}
				<div class="flex flex-col items-center gap-3 px-6 text-center">
					<p class="text-sm font-medium text-neutral-700">{m.devices_connect_failed()}</p>
					<Button size="sm" onclick={refresh}>
						<RefreshCw class="size-4" />
						{m.common_retry()}
					</Button>
				</div>
			{:else if link}
				<QrCode value={link} label={m.devices_connect_qr()} size={240} />
			{/if}
			{#if loading}
				<div class="absolute inset-0 flex items-center justify-center bg-white/80">
					<LoaderCircle class="size-8 animate-spin text-neutral-400" />
				</div>
			{/if}
		</div>

		<div class="mt-4 flex items-center gap-2" class:invisible={!connect}>
			<span
				class="inline-flex items-center gap-1.5 rounded-full border border-edge px-3 py-1 text-xs
					font-semibold text-muted tnum"
			>
				<Clock class="size-3.5" />
				{m.devices_connect_new_code_in({ time: formatClock(remaining) })}
			</span>
			<Button variant="ghost" size="sm" onclick={refresh} disabled={loading}>
				<RefreshCw class="size-3.5" />
				{m.devices_connect_regenerate()}
			</Button>
		</div>

		<dl
			class="mt-5 w-full divide-y divide-edge/60 rounded-input border border-edge bg-surface text-left
				text-sm"
		>
			<div class="flex items-center gap-3 px-4 py-2.5">
				<dt class="w-14 shrink-0 text-[11px] font-semibold tracking-wider text-faint uppercase">
					{m.devices_connect_server()}
				</dt>
				<dd class="min-w-0 flex-1 truncate font-mono">{server}</dd>
			</div>
			<div class="flex items-center gap-3 px-4 py-2">
				<dt class="w-14 shrink-0 text-[11px] font-semibold tracking-wider text-faint uppercase">
					{m.devices_connect_code()}
				</dt>
				<dd class="min-w-0 flex-1 truncate font-mono text-muted">{connect?.code ?? '...'}</dd>
				{#if canCopy}
					<button
						type="button"
						onclick={copy}
						disabled={!connect}
						aria-label={m.devices_connect_copy()}
						title={m.devices_connect_copy()}
						class="-mr-2 flex size-8 shrink-0 cursor-pointer items-center justify-center rounded-full
							text-muted transition-colors hover:bg-surface-2 hover:text-text
							focus-visible:bg-surface-2 focus-visible:text-text disabled:opacity-50"
					>
						<Copy class="size-4" />
					</button>
				{/if}
			</div>
		</dl>

		{#if loopback}
			<p
				class="mt-4 flex items-start gap-2 rounded-input border border-amber-400/30 bg-amber-400/10 px-3
					py-2.5 text-xs text-amber-300"
			>
				<TriangleAlert class="mt-px size-4 shrink-0" />
				{m.devices_connect_loopback()}
			</p>
		{/if}

		<p class="mt-4 text-center text-xs text-faint">{m.devices_connect_hint()}</p>
	</div>
</Modal>
