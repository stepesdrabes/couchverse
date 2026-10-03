import glueSource from './pkg/couchverse_core.js?raw';
import wasmUrl from './pkg/couchverse_core_bg.wasm?url';
import type { Bridge, Spawn } from './runtime.svelte';

interface Glue {
	default(init: { module_or_path: WebAssembly.Module }): Promise<unknown>;
	CoreBridge: new (config: string) => Bridge;
}

let instances = 0;

/**
 * Instances of the compiled core. The wasm-bindgen glue keeps its instance in module scope
 * and never instantiates twice, so each core evaluates its own copy of the glue: a trapped
 * instance is replaced, never reused.
 */
export function spawner(module: Promise<WebAssembly.Module>): Spawn {
	return async (config) => {
		const source = `${glueSource}\n// core instance ${++instances}`;
		const url = `data:text/javascript;charset=utf-8,${encodeURIComponent(source)}`;
		const glue: Glue = await import(/* @vite-ignore */ url);
		await glue.default({ module_or_path: await module });
		return new glue.CoreBridge(config);
	};
}

/** Downloads and compiles the core, compiling while the bytes stream in. */
export async function compileCore(): Promise<WebAssembly.Module> {
	try {
		return await WebAssembly.compileStreaming(fetch(wasmUrl));
	} catch {
		// a proxy that serves .wasm without its MIME type rules out streaming
		const response = await fetch(wasmUrl);
		return WebAssembly.compile(await response.arrayBuffer());
	}
}
