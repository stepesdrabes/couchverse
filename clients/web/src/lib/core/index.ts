import { AuthMode, Platform } from '$lib/generated/core';
import { getLocale } from '$lib/paraglide/runtime';
import { browserExecutor } from './executor';
import { CoreRuntime } from './runtime.svelte';
import { compileCore, spawner } from './wasm';

/**
 * The app's shared core, running the session over the browser's cookie. The download starts
 * as soon as this module loads, ahead of the root layout's `start()`.
 */
export const core = new CoreRuntime(
	{
		platform: Platform.Web,
		authMode: AuthMode.Cookie,
		// the web has no device sessions to name
		deviceName: '',
		// what the visitor sees now, which a first sign-in saves as the account's language
		locale: getLocale(),
		origin: location.origin
	},
	spawner(compileCore()),
	browserExecutor
);
