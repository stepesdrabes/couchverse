import { core } from '$lib/core';
import type { Surface } from '$lib/generated/core';

// The account's devices come from the core, which keeps the last list it read.

export const DEVICES: Surface = { type: 'devices' };

/** Reads the list again; settles once it has landed. */
export const openDevices = () => core.send({ type: 'devicesOpened' });
