<script lang="ts">
	import { tick, untrack } from 'svelte';
	import { fade, fly, scale } from 'svelte/transition';
	import { backOut } from 'svelte/easing';
	import { replaceState } from '$app/navigation';
	import { page } from '$app/state';
	import {
		Ban,
		CircleCheck,
		Clock,
		MonitorSmartphone,
		ShieldAlert,
		TimerOff,
		type Icon
	} from 'lucide-svelte';
	import { toast } from 'svelte-sonner';
	import { ApiError } from '$lib/api/client';
	import GlowBackdrop from '$lib/components/layout/GlowBackdrop.svelte';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import UserAvatar from '$lib/components/ui/UserAvatar.svelte';
	import { isTV } from '$lib/tv/tv';
	import { formatClock } from '$lib/utils/format';
	import * as authApi from '../api';
	import type { PairingRequest } from '../api';
	import PairCodeInput from '../components/PairCodeInput.svelte';
	import { formatPairingCode, isCompletePairingCode } from '../pairing';
	import { platformIcon, platformName } from '../platforms';
	import { session } from '../session.svelte';
	import * as m from '$lib/paraglide/messages';

	// A TV or phone signing in shows a code (and a QR of /pair?code=...); a signed-in
	// member looks it up here and approves or denies it.

	type Phase = 'entry' | 'request' | 'approved' | 'denied' | 'expired';
	type Outcome = Exclude<Phase, 'entry' | 'request'>;

	const codeParam = () => formatPairingCode(page.url.searchParams.get('code') ?? '');

	let phase = $state<Phase>('entry');
	let code = $state(codeParam());
	let looking = $state(false);
	// an error belongs to the code it came from, so editing the code clears it
	let failed = $state<{ code: string; message: string } | null>(null);
	let request = $state<PairingRequest | null>(null);
	let deviceName = $state('');
	let busy = $state<'approve' | 'deny' | null>(null);
	let now = $state(Date.now());
	let clockAgrees = $state(true);
	let card = $state<HTMLElement>();
	let codeInput = $state<HTMLInputElement>();

	const error = $derived(failed?.code === code ? failed.message : '');
	const remaining = $derived(
		request ? Math.max(0, Math.ceil((Date.parse(request.expiresAt) - now) / 1000)) : 0
	);
	const PlatformIcon = $derived(platformIcon(request?.platform ?? ''));
	const user = $derived(session.user);

	const outcomes: Record<
		Outcome,
		{ icon: typeof Icon; tone: string; title: () => string; body: () => string }
	> = {
		approved: {
			icon: CircleCheck,
			tone: 'bg-success/15 text-success',
			title: () => m.pair_approved_title({ device: request?.deviceName ?? '' }),
			body: () => m.pair_approved_body()
		},
		denied: {
			icon: Ban,
			tone: 'bg-surface-2 text-muted',
			title: () => m.pair_denied_title(),
			body: () => m.pair_denied_body({ device: request?.deviceName ?? '' })
		},
		expired: {
			icon: TimerOff,
			tone: 'bg-surface-2 text-muted',
			title: () => m.pair_expired_title(),
			body: () => m.pair_expired_body()
		}
	};

	async function show(next: Phase) {
		phase = next;
		await tick();
		// Each step needs a visible starting point for the remote. A keyboard is not
		// put on Approve, where one stray Enter would sign the device in; the heading
		// takes focus instead, so a screen reader announces the request.
		const selector = isTV || next !== 'request' ? '[data-tv-autofocus]' : 'h1';
		card?.querySelector<HTMLElement>(selector)?.focus();
	}

	async function lookup() {
		if (!isCompletePairingCode(code) || looking) return;
		const asked = code;
		looking = true;
		try {
			request = await authApi.getPairingRequest(asked);
			deviceName = request.deviceName;
			now = Date.now();
			clockAgrees = true;
			await show('request');
		} catch (err) {
			const unknown = err instanceof ApiError && err.status === 404;
			failed = { code: asked, message: unknown ? m.pair_code_unknown() : m.pair_error() };
			codeInput?.focus();
		} finally {
			looking = false;
		}
	}

	function submit(e: SubmitEvent) {
		e.preventDefault();
		lookup();
	}

	// the code can run out or be answered in another tab while this page sits open
	function settle(err: unknown) {
		if (err instanceof ApiError && err.status === 404) show('expired');
		else toast.error(m.pair_error());
	}

	async function approve() {
		if (!request || busy) return;
		busy = 'approve';
		const name = deviceName.trim();
		const renamed = name !== '' && name !== request.deviceName;
		try {
			await authApi.approvePairing(request.userCode, renamed ? { deviceName: name } : undefined);
			if (renamed) request.deviceName = name;
			await show('approved');
		} catch (err) {
			settle(err);
		} finally {
			busy = null;
		}
	}

	async function deny() {
		if (!request || busy) return;
		busy = 'deny';
		try {
			await authApi.denyPairing(request.userCode);
			await show('denied');
		} catch (err) {
			settle(err);
		} finally {
			busy = null;
		}
	}

	function again() {
		request = null;
		failed = null;
		code = '';
		// a reload must not look the answered code up again
		if (page.url.searchParams.has('code')) replaceState('/pair', {});
		show('entry');
	}

	// a device's QR lands here with the code in the URL, ready to look up
	$effect(() => {
		const fromUrl = codeParam();
		if (!isCompletePairingCode(fromUrl)) return;
		untrack(() => {
			request = null;
			failed = null;
			phase = 'entry';
			code = fromUrl;
			lookup();
		});
	});

	$effect(() => {
		if (phase !== 'request' || !request) return;
		const { userCode, expiresAt } = request;
		const deadline = Date.parse(expiresAt);
		let checking = false;
		const timer = setInterval(async () => {
			now = Date.now();
			if (now < deadline || checking || !clockAgrees) return;
			// This clock can disagree with the server's, which has the last word: a
			// request still pending past the deadline only hides the countdown.
			checking = true;
			try {
				await authApi.getPairingRequest(userCode);
				clockAgrees = false;
			} catch (err) {
				if (err instanceof ApiError && err.status === 404) show('expired');
			} finally {
				checking = false;
			}
		}, 1000);
		return () => clearInterval(timer);
	});
</script>

<svelte:head>
	<title>{m.pair_page_title()}</title>
</svelte:head>

<div class="relative flex min-h-dvh items-center justify-center overflow-hidden px-4 pt-28 pb-16">
	<GlowBackdrop />

	<div bind:this={card} class="relative w-full max-w-md">
		{#key phase}
			<div
				in:fly={{ y: 12, duration: 300 }}
				class="rounded-card border border-edge bg-surface/70 p-6 shadow-2xl shadow-black/30
					backdrop-blur sm:p-8"
			>
				{#if phase === 'entry'}
					<form onsubmit={submit} class="flex flex-col items-center text-center">
						<span
							class="mb-5 flex size-14 items-center justify-center rounded-2xl bg-accent/15 text-accent"
						>
							<MonitorSmartphone class="size-7" />
						</span>
						<h1 class="text-2xl font-extrabold tracking-tight">{m.pair_heading()}</h1>
						<p class="mt-2 text-sm text-muted">{m.pair_intro()}</p>

						<PairCodeInput
							bind:value={code}
							bind:input={codeInput}
							oncomplete={lookup}
							invalid={!!error}
							readonly={looking}
							aria-label={m.pair_code_label()}
							aria-describedby={error ? 'pair-error' : undefined}
							autofocus={!code}
							data-tv-autofocus
							class="mt-7 w-full"
						/>
						{#if error}
							<p
								id="pair-error"
								transition:fade={{ duration: 150 }}
								class="mt-3 text-sm text-danger"
							>
								{error}
							</p>
						{/if}

						<Button
							type="submit"
							size="lg"
							class="mt-6 w-full"
							loading={looking}
							disabled={!isCompletePairingCode(code)}
						>
							{m.pair_continue()}
						</Button>
					</form>
				{:else if phase === 'request' && request}
					<div class="flex flex-col items-center text-center">
						<div class="mb-6 flex items-center gap-3" aria-hidden="true">
							<span
								class="flex size-16 items-center justify-center rounded-2xl border border-edge
									bg-surface-2 text-accent shadow-lg shadow-black/30"
							>
								<PlatformIcon class="size-8" />
							</span>
							<span class="link flex gap-1.5">
								<span></span>
								<span></span>
								<span></span>
							</span>
							<UserAvatar
								name={user?.displayName ?? '?'}
								avatarId={user?.avatarId}
								seed={user?.username}
								class="size-16 rounded-2xl text-xl shadow-lg shadow-black/30"
							/>
						</div>

						<p class="eyebrow mb-2 uppercase">{platformName(request.platform)}</p>
						<h1 tabindex="-1" class="text-xl font-bold tracking-tight outline-none sm:text-2xl">
							{m.pair_request_title({ device: request.deviceName, name: user?.displayName ?? '' })}
						</h1>
						{#if clockAgrees}
							<span
								class="mt-4 inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs
									font-semibold tnum transition-colors
									{remaining <= 60 ? 'border-danger/40 text-danger' : 'border-edge text-muted'}"
							>
								<Clock class="size-3.5" />
								{m.pair_expires_in({ time: formatClock(remaining) })}
							</span>
						{/if}
					</div>

					<div class="mt-7">
						<Input label={m.pair_device_name()} bind:value={deviceName} maxlength={60} />
						<p class="mt-1.5 text-xs text-faint">{m.pair_device_name_hint()}</p>
					</div>

					<p
						class="mt-5 flex items-start gap-2 rounded-input border border-edge bg-surface-2/60 px-3
							py-2.5 text-left text-xs text-muted"
					>
						<ShieldAlert class="mt-px size-4 shrink-0 text-faint" />
						{m.pair_request_warning()}
					</p>

					<div class="mt-6 grid grid-cols-2 gap-3">
						<Button
							variant="secondary"
							size="lg"
							onclick={deny}
							loading={busy === 'deny'}
							disabled={busy !== null}
						>
							{m.pair_deny()}
						</Button>
						<Button
							size="lg"
							onclick={approve}
							loading={busy === 'approve'}
							disabled={busy !== null}
							data-tv-autofocus
						>
							{m.pair_approve()}
						</Button>
					</div>
				{:else if phase !== 'request'}
					{@const outcome = outcomes[phase]}
					<div class="flex flex-col items-center text-center">
						<span
							class="mb-5 flex size-16 items-center justify-center rounded-full {outcome.tone}"
							in:scale={{ duration: 450, start: 0.6, easing: backOut }}
						>
							<outcome.icon class="size-8" />
						</span>
						<h1 tabindex="-1" class="text-2xl font-extrabold tracking-tight outline-none">
							{outcome.title()}
						</h1>
						<p class="mt-2 text-sm text-muted">{outcome.body()}</p>

						<div class="mt-7 flex w-full flex-col gap-3 sm:flex-row-reverse">
							{#if phase === 'approved'}
								<a
									href="/"
									data-tv-autofocus
									class="inline-flex h-11 flex-1 items-center justify-center rounded-full bg-accent px-6
										text-[15px] font-semibold text-[var(--color-on-accent)] transition-colors
										hover:bg-accent-strong focus-visible:bg-accent-strong"
								>
									{m.pair_done()}
								</a>
								<a
									href="/profile#devices"
									class="inline-flex h-11 flex-1 items-center justify-center rounded-full border
										border-edge bg-surface/60 px-6 text-[15px] font-semibold transition-colors
										hover:border-faint hover:bg-surface-2 focus-visible:border-faint
										focus-visible:bg-surface-2"
								>
									{m.pair_manage_devices()}
								</a>
							{:else}
								<Button size="lg" class="flex-1" onclick={again} data-tv-autofocus>
									{m.pair_another_code()}
								</Button>
							{/if}
						</div>
					</div>
				{/if}
			</div>
		{/key}
	</div>
</div>

<style lang="scss">
	// three dots travelling from the device to you
	.link span {
		width: 0.375rem;
		height: 0.375rem;
		border-radius: 999px;
		background: var(--color-accent);
		opacity: 0.25;
		animation: travel 1.4s ease-in-out infinite;

		&:nth-child(2) {
			animation-delay: 0.2s;
		}
		&:nth-child(3) {
			animation-delay: 0.4s;
		}
	}

	@keyframes travel {
		0%,
		100% {
			opacity: 0.25;
		}
		40% {
			opacity: 1;
		}
	}
</style>
