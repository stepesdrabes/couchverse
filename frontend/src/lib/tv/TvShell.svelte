<script lang="ts">
	import { afterNavigate, beforeNavigate, goto, preloadData } from '$app/navigation';
	import { page } from '$app/state';
	import Confirm from '$lib/components/ui/Confirm.svelte';
	import {
		activeLayer,
		defaultTarget,
		DIRECTIONS,
		findNext,
		focusable,
		focusTarget,
		leavesField,
		type Direction
	} from '$lib/tv/spatial-nav';
	import { exitApp, isBackKey } from '$lib/tv/tv';
	import * as m from '$lib/paraglide/messages';

	// Mounted by the root layout on TVs only. Owns the remote: arrows move focus, Back
	// closes the open overlay or goes back, and on the main screen asks to exit (Titan
	// OS requires the confirmation). Feature code that handles a key itself calls
	// preventDefault, and this listener (on window, so it runs last) leaves it alone.

	let exitOpen = $state(false);

	// in-app history depth, so Back never walks out of the app into the launcher
	let depth = 0;

	type Remembered = { href: string; nth: number };
	// the link focused on each page when it was left, so returning lands on the same
	// card instead of the top of the page
	const lastFocus: Record<string, Remembered> = {};

	const isMainScreen = () =>
		page.url.pathname === '/' || (page.route.id?.startsWith('/(auth)') ?? false);

	function filledField(el: EventTarget | null): boolean {
		return (
			((el instanceof HTMLInputElement && el.type !== 'range') ||
				el instanceof HTMLTextAreaElement) &&
			el.value !== ''
		);
	}

	function back() {
		const layer = activeLayer();
		if (layer) {
			// bits-ui overlays close on Escape; custom layers listen for it as well
			const at = document.activeElement;
			const target = at && layer.contains(at) ? at : layer;
			target.dispatchEvent(
				new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })
			);
			return;
		}
		if (isMainScreen()) {
			exitOpen = true;
			return;
		}
		if (depth === 0) {
			goto('/', { replaceState: true });
			return;
		}
		const from = location.href;
		history.back();
		// replaceState navigations still count towards `depth`; if there turned out
		// to be nothing to go back to, go home instead of staying stuck
		setTimeout(() => {
			if (location.href === from) goto('/', { replaceState: true });
		}, 500);
	}

	function move(dir: Direction) {
		const root = activeLayer() ?? document.body;
		const current = document.activeElement;
		if (!focusable(current) || !root.contains(current)) {
			const start = defaultTarget(root);
			if (start) focusTarget(start);
			return;
		}
		const next = findNext(current, dir, root);
		if (next) focusTarget(next);
	}

	// bits-ui opens a select or menu when its trigger gets an up/down arrow; on a TV the
	// arrows move on and OK opens it, so catch them before bits-ui sees them
	function onTriggerKeydown(e: KeyboardEvent) {
		const dir = DIRECTIONS[e.key];
		const trigger = e.target instanceof HTMLElement ? e.target : null;
		if (!dir || !trigger?.matches('[data-select-trigger],[data-dropdown-menu-trigger]')) return;
		if (trigger.getAttribute('aria-expanded') === 'true') return;
		e.stopPropagation();
		e.preventDefault();
		move(dir);
	}

	function onKeydown(e: KeyboardEvent) {
		if (e.defaultPrevented) return;
		if (isBackKey(e)) {
			// with text in a field, Back is the on-screen keyboard's delete
			if (filledField(e.target)) return;
			e.preventDefault();
			back();
			return;
		}
		const dir = DIRECTIONS[e.key];
		if (!dir || e.altKey || e.ctrlKey || e.metaKey) return;
		const current = document.activeElement;
		if (current && current !== document.body && !leavesField(current, dir)) return;
		e.preventDefault();
		move(dir);
	}

	// bits-ui focuses an opened dialog's own container, which shows no focus ring; the
	// remote needs a visible starting point, so move on to its first control
	function enterDialog(dialog: HTMLElement) {
		requestAnimationFrame(() => {
			if (document.activeElement !== dialog) return;
			const first = defaultTarget(dialog);
			if (first) focusTarget(first, 'nearest');
		});
	}

	// Focus has no hover to trigger SvelteKit's preloading, so a link the remote rests
	// on preloads instead. Same opt-in as hover: watch links say "tap" because loading
	// one starts a transcode.
	let preloadTimer: ReturnType<typeof setTimeout>;
	function onFocusIn(e: FocusEvent) {
		if (
			e.target instanceof HTMLElement &&
			e.target.matches('[role="dialog"],[role="alertdialog"]')
		) {
			enterDialog(e.target);
		}
		clearTimeout(preloadTimer);
		const link = e.target instanceof HTMLAnchorElement ? e.target : null;
		const href = link?.getAttribute('href');
		const mode = link
			?.closest('[data-sveltekit-preload-data]')
			?.getAttribute('data-sveltekit-preload-data');
		if (!href?.startsWith('/') || mode !== 'hover') return;
		preloadTimer = setTimeout(() => preloadData(href).catch(() => {}), 300);
	}

	beforeNavigate(({ from }) => {
		const el = document.activeElement;
		const href = el instanceof HTMLAnchorElement ? el.getAttribute('href') : null;
		if (!from || !el || !href) return;
		const same = [...document.querySelectorAll(`a[href="${CSS.escape(href)}"]`)];
		lastFocus[from.url.href] = { href, nth: same.indexOf(el) };
	});

	// Page content can land a moment after the navigation (cached views, skeletons), so
	// placing focus retries briefly. Focus that is already somewhere real (an autofocus
	// field, a nav link that survived the swap, the user) wins, and the nav bar is only
	// the last resort.
	let settleTimer: ReturnType<typeof setInterval>;
	function settleFocus(remembered: Remembered | undefined) {
		clearInterval(settleTimer);
		let tries = 0;
		const attempt = () => {
			const final = ++tries >= 15;
			const target = focusable(document.activeElement) ? null : arrivalTarget(remembered, final);
			// arriving never scrolls more than it has to (SvelteKit restored the scroll)
			if (target) focusTarget(target, 'nearest');
			if (final || focusable(document.activeElement)) clearInterval(settleTimer);
		};
		attempt();
		settleTimer = setInterval(attempt, 100);
	}

	function arrivalTarget(remembered: Remembered | undefined, final: boolean): HTMLElement | null {
		const layer = activeLayer();
		if (remembered && !layer) {
			const selector = `a[href="${CSS.escape(remembered.href)}"]`;
			const el = document.querySelectorAll(selector)[remembered.nth];
			if (focusable(el)) return el;
			// keep waiting for the remembered card to render
			if (!final) return null;
		}
		return defaultTarget(layer ?? document.body, final);
	}

	afterNavigate((nav) => {
		if (nav.type === 'enter') depth = 0;
		else if (nav.type === 'popstate') depth = Math.max(0, depth + (nav.delta ?? -1));
		else depth++;
		settleFocus(nav.type === 'popstate' && nav.to ? lastFocus[nav.to.url.href] : undefined);
	});
</script>

<svelte:window onkeydowncapture={onTriggerKeydown} onkeydown={onKeydown} onfocusin={onFocusIn} />

<Confirm
	bind:open={exitOpen}
	title={m.tv_exit_title()}
	message={m.tv_exit_message()}
	confirmLabel={m.tv_exit_confirm()}
	onconfirm={exitApp}
/>
